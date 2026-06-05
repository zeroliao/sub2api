package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *opsRepository) ListRequestDetails(ctx context.Context, filter *service.OpsRequestDetailFilter) ([]*service.OpsRequestDetail, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, fmt.Errorf("nil ops repository")
	}

	page, pageSize, startTime, endTime := filter.Normalize()
	offset := (page - 1) * pageSize

	conditions := make([]string, 0, 16)
	args := make([]any, 0, 24)

	// Placeholders $1/$2 reserved for time window inside the CTE.
	args = append(args, startTime.UTC(), endTime.UTC())

	addCondition := func(condition string, values ...any) {
		conditions = append(conditions, condition)
		args = append(args, values...)
	}

	if filter != nil {
		if kind := strings.TrimSpace(strings.ToLower(filter.Kind)); kind != "" && kind != "all" {
			if kind != string(service.OpsRequestKindSuccess) && kind != string(service.OpsRequestKindError) {
				return nil, 0, fmt.Errorf("invalid kind")
			}
			addCondition(fmt.Sprintf("kind = $%d", len(args)+1), kind)
		}

		if platform := strings.TrimSpace(strings.ToLower(filter.Platform)); platform != "" {
			addCondition(fmt.Sprintf("platform = $%d", len(args)+1), platform)
		}
		if filter.GroupID != nil && *filter.GroupID > 0 {
			addCondition(fmt.Sprintf("group_id = $%d", len(args)+1), *filter.GroupID)
		}

		if filter.UserID != nil && *filter.UserID > 0 {
			addCondition(fmt.Sprintf("user_id = $%d", len(args)+1), *filter.UserID)
		}
		if filter.APIKeyID != nil && *filter.APIKeyID > 0 {
			addCondition(fmt.Sprintf("api_key_id = $%d", len(args)+1), *filter.APIKeyID)
		}
		if filter.AccountID != nil && *filter.AccountID > 0 {
			addCondition(fmt.Sprintf("account_id = $%d", len(args)+1), *filter.AccountID)
		}

		if model := strings.TrimSpace(filter.Model); model != "" {
			addCondition(fmt.Sprintf("model = $%d", len(args)+1), model)
		}
		if requestID := strings.TrimSpace(filter.RequestID); requestID != "" {
			addCondition(fmt.Sprintf("request_id = $%d", len(args)+1), requestID)
		}
		if q := strings.TrimSpace(filter.Query); q != "" {
			like := "%" + strings.ToLower(q) + "%"
			startIdx := len(args) + 1
			addCondition(
				fmt.Sprintf(`(
					LOWER(COALESCE(request_id,'')) LIKE $%d OR
					LOWER(COALESCE(model,'')) LIKE $%d OR
					LOWER(COALESCE(message,'')) LIKE $%d OR
					LOWER(COALESCE(user_email,'')) LIKE $%d OR
					LOWER(COALESCE(api_key_name,'')) LIKE $%d OR
					LOWER(COALESCE(account_name,'')) LIKE $%d OR
					LOWER(COALESCE(group_name,'')) LIKE $%d OR
					LOWER(COALESCE(inbound_endpoint,'')) LIKE $%d OR
					LOWER(COALESCE(upstream_endpoint,'')) LIKE $%d OR
					LOWER(COALESCE(error_owner,'')) LIKE $%d OR
					LOWER(COALESCE(error_source,'')) LIKE $%d
				)`,
					startIdx, startIdx+1, startIdx+2, startIdx+3, startIdx+4, startIdx+5,
					startIdx+6, startIdx+7, startIdx+8, startIdx+9, startIdx+10,
				),
				like, like, like, like, like, like, like, like, like, like, like,
			)
		}

		if filter.MinDurationMs != nil {
			addCondition(fmt.Sprintf("duration_ms >= $%d", len(args)+1), *filter.MinDurationMs)
		}
		if filter.MaxDurationMs != nil {
			addCondition(fmt.Sprintf("duration_ms <= $%d", len(args)+1), *filter.MaxDurationMs)
		}
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	cte := `
WITH combined AS (
  SELECT
    'success'::TEXT AS kind,
    ul.created_at AS created_at,
    ul.request_id AS request_id,
    COALESCE(NULLIF(g.platform, ''), NULLIF(a.platform, ''), '') AS platform,
    ul.model AS model,
    ul.duration_ms AS duration_ms,
    NULL::INT AS status_code,
    ul.first_token_ms::BIGINT AS first_token_ms,
    NULL::BIGINT AS auth_latency_ms,
    NULL::BIGINT AS routing_latency_ms,
    NULL::BIGINT AS upstream_latency_ms,
    NULL::BIGINT AS response_latency_ms,
    ul.first_token_ms::BIGINT AS time_to_first_token_ms,
    NULL::INT AS upstream_status_code,
    NULL::TEXT AS error_owner,
    NULL::TEXT AS error_source,
    COALESCE(NULLIF(ul.inbound_endpoint, ''), '') AS inbound_endpoint,
    COALESCE(NULLIF(ul.upstream_endpoint, ''), '') AS upstream_endpoint,
    COALESCE(NULLIF(ul.requested_model, ''), NULLIF(ul.model, ''), '') AS requested_model,
    COALESCE(NULLIF(ul.upstream_model, ''), '') AS upstream_model,
    ul.request_type AS request_type,
    ul.channel_id AS channel_id,
    COALESCE(NULLIF(ul.model_mapping_chain, ''), '') AS model_mapping_chain,
    COALESCE(NULLIF(ul.billing_tier, ''), '') AS billing_tier,
    NULL::BIGINT AS error_id,
    NULL::TEXT AS phase,
    NULL::TEXT AS severity,
    NULL::TEXT AS message,
    ul.user_id AS user_id,
    ul.api_key_id AS api_key_id,
    ul.account_id AS account_id,
    ul.group_id AS group_id,
    COALESCE(NULLIF(u.email, ''), '') AS user_email,
    COALESCE(NULLIF(ak.name, ''), '') AS api_key_name,
    COALESCE(NULLIF(a.name, ''), '') AS account_name,
    COALESCE(NULLIF(g.name, ''), '') AS group_name,
    ul.stream AS stream
  FROM usage_logs ul
  LEFT JOIN users u ON u.id = ul.user_id
  LEFT JOIN api_keys ak ON ak.id = ul.api_key_id
  LEFT JOIN groups g ON g.id = ul.group_id
  LEFT JOIN accounts a ON a.id = ul.account_id
  WHERE ul.created_at >= $1 AND ul.created_at < $2

  UNION ALL

  SELECT
    'error'::TEXT AS kind,
    o.created_at AS created_at,
    COALESCE(NULLIF(o.request_id,''), NULLIF(o.client_request_id,''), '') AS request_id,
    COALESCE(NULLIF(o.platform, ''), NULLIF(g.platform, ''), NULLIF(a.platform, ''), '') AS platform,
    o.model AS model,
    o.duration_ms AS duration_ms,
    o.status_code AS status_code,
    o.time_to_first_token_ms AS first_token_ms,
    o.auth_latency_ms AS auth_latency_ms,
    o.routing_latency_ms AS routing_latency_ms,
    o.upstream_latency_ms AS upstream_latency_ms,
    o.response_latency_ms AS response_latency_ms,
    o.time_to_first_token_ms AS time_to_first_token_ms,
    o.upstream_status_code AS upstream_status_code,
    o.error_owner AS error_owner,
    o.error_source AS error_source,
    COALESCE(NULLIF(o.inbound_endpoint, ''), '') AS inbound_endpoint,
    COALESCE(NULLIF(o.upstream_endpoint, ''), '') AS upstream_endpoint,
    COALESCE(NULLIF(o.requested_model, ''), NULLIF(o.model, ''), '') AS requested_model,
    COALESCE(NULLIF(o.upstream_model, ''), '') AS upstream_model,
    o.request_type AS request_type,
    NULL::BIGINT AS channel_id,
    NULL::TEXT AS model_mapping_chain,
    NULL::TEXT AS billing_tier,
    o.id AS error_id,
    o.error_phase AS phase,
    o.severity AS severity,
    o.error_message AS message,
    o.user_id AS user_id,
    o.api_key_id AS api_key_id,
    o.account_id AS account_id,
    o.group_id AS group_id,
    COALESCE(NULLIF(u.email, ''), '') AS user_email,
    COALESCE(NULLIF(ak.name, ''), '') AS api_key_name,
    COALESCE(NULLIF(a.name, ''), '') AS account_name,
    COALESCE(NULLIF(g.name, ''), '') AS group_name,
    o.stream AS stream
  FROM ops_error_logs o
  LEFT JOIN users u ON u.id = o.user_id
  LEFT JOIN api_keys ak ON ak.id = o.api_key_id
  LEFT JOIN groups g ON g.id = o.group_id
  LEFT JOIN accounts a ON a.id = o.account_id
  WHERE o.created_at >= $1 AND o.created_at < $2
    AND COALESCE(o.status_code, 0) >= 400
)
`

	countQuery := fmt.Sprintf(`%s SELECT COUNT(1) FROM combined %s`, cte, where)
	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		if err == sql.ErrNoRows {
			total = 0
		} else {
			return nil, 0, err
		}
	}

	sort := "ORDER BY created_at DESC"
	if filter != nil {
		switch strings.TrimSpace(strings.ToLower(filter.Sort)) {
		case "", "created_at_desc":
			// default
		case "duration_desc":
			sort = "ORDER BY duration_ms DESC NULLS LAST, created_at DESC"
		default:
			return nil, 0, fmt.Errorf("invalid sort")
		}
	}

	listQuery := fmt.Sprintf(`
%s
SELECT
  kind,
  created_at,
  request_id,
  platform,
  model,
  duration_ms,
  status_code,
  first_token_ms,
  auth_latency_ms,
  routing_latency_ms,
  upstream_latency_ms,
  response_latency_ms,
  time_to_first_token_ms,
  upstream_status_code,
  error_owner,
  error_source,
  inbound_endpoint,
  upstream_endpoint,
  requested_model,
  upstream_model,
  request_type,
  channel_id,
  model_mapping_chain,
  billing_tier,
  error_id,
  phase,
  severity,
  message,
  user_id,
  api_key_id,
  account_id,
  group_id,
  user_email,
  api_key_name,
  account_name,
  group_name,
  stream
FROM combined
%s
%s
LIMIT $%d OFFSET $%d
`, cte, where, sort, len(args)+1, len(args)+2)

	listArgs := append(append([]any{}, args...), pageSize, offset)
	rows, err := r.db.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	toIntPtr := func(v sql.NullInt64) *int {
		if !v.Valid {
			return nil
		}
		i := int(v.Int64)
		return &i
	}
	toInt64Ptr := func(v sql.NullInt64) *int64 {
		if !v.Valid {
			return nil
		}
		i := v.Int64
		return &i
	}
	toInt16Ptr := func(v sql.NullInt16) *int16 {
		if !v.Valid {
			return nil
		}
		i := v.Int16
		return &i
	}

	out := make([]*service.OpsRequestDetail, 0, pageSize)
	for rows.Next() {
		var (
			kind      string
			createdAt time.Time
			requestID sql.NullString
			platform  sql.NullString
			model     sql.NullString

			durationMs         sql.NullInt64
			statusCode         sql.NullInt64
			firstTokenMs       sql.NullInt64
			authLatencyMs      sql.NullInt64
			routingLatencyMs   sql.NullInt64
			upstreamLatencyMs  sql.NullInt64
			responseLatencyMs  sql.NullInt64
			timeToFirstTokenMs sql.NullInt64
			upstreamStatusCode sql.NullInt64
			errorOwner         sql.NullString
			errorSource        sql.NullString
			inboundEndpoint    sql.NullString
			upstreamEndpoint   sql.NullString
			requestedModel     sql.NullString
			upstreamModel      sql.NullString
			requestType        sql.NullInt16
			channelID          sql.NullInt64
			modelMappingChain  sql.NullString
			billingTier        sql.NullString
			errorID            sql.NullInt64

			phase    sql.NullString
			severity sql.NullString
			message  sql.NullString

			userID    sql.NullInt64
			apiKeyID  sql.NullInt64
			accountID sql.NullInt64
			groupID   sql.NullInt64

			userEmail   sql.NullString
			apiKeyName  sql.NullString
			accountName sql.NullString
			groupName   sql.NullString

			stream bool
		)

		if err := rows.Scan(
			&kind,
			&createdAt,
			&requestID,
			&platform,
			&model,
			&durationMs,
			&statusCode,
			&firstTokenMs,
			&authLatencyMs,
			&routingLatencyMs,
			&upstreamLatencyMs,
			&responseLatencyMs,
			&timeToFirstTokenMs,
			&upstreamStatusCode,
			&errorOwner,
			&errorSource,
			&inboundEndpoint,
			&upstreamEndpoint,
			&requestedModel,
			&upstreamModel,
			&requestType,
			&channelID,
			&modelMappingChain,
			&billingTier,
			&errorID,
			&phase,
			&severity,
			&message,
			&userID,
			&apiKeyID,
			&accountID,
			&groupID,
			&userEmail,
			&apiKeyName,
			&accountName,
			&groupName,
			&stream,
		); err != nil {
			return nil, 0, err
		}

		item := &service.OpsRequestDetail{
			Kind:      service.OpsRequestKind(kind),
			CreatedAt: createdAt,
			RequestID: strings.TrimSpace(requestID.String),
			Platform:  strings.TrimSpace(platform.String),
			Model:     strings.TrimSpace(model.String),

			DurationMs:         toIntPtr(durationMs),
			StatusCode:         toIntPtr(statusCode),
			FirstTokenMs:       toInt64Ptr(firstTokenMs),
			AuthLatencyMs:      toInt64Ptr(authLatencyMs),
			RoutingLatencyMs:   toInt64Ptr(routingLatencyMs),
			UpstreamLatencyMs:  toInt64Ptr(upstreamLatencyMs),
			ResponseLatencyMs:  toInt64Ptr(responseLatencyMs),
			TimeToFirstTokenMs: toInt64Ptr(timeToFirstTokenMs),
			UpstreamStatusCode: toIntPtr(upstreamStatusCode),
			ErrorOwner:         strings.TrimSpace(errorOwner.String),
			ErrorSource:        strings.TrimSpace(errorSource.String),
			InboundEndpoint:    strings.TrimSpace(inboundEndpoint.String),
			UpstreamEndpoint:   strings.TrimSpace(upstreamEndpoint.String),
			RequestedModel:     strings.TrimSpace(requestedModel.String),
			UpstreamModel:      strings.TrimSpace(upstreamModel.String),
			RequestType:        toInt16Ptr(requestType),
			ChannelID:          toInt64Ptr(channelID),
			ModelMappingChain:  strings.TrimSpace(modelMappingChain.String),
			BillingTier:        strings.TrimSpace(billingTier.String),
			ErrorID:            toInt64Ptr(errorID),
			Phase:              phase.String,
			Severity:           severity.String,
			Message:            message.String,

			UserID:      toInt64Ptr(userID),
			APIKeyID:    toInt64Ptr(apiKeyID),
			AccountID:   toInt64Ptr(accountID),
			GroupID:     toInt64Ptr(groupID),
			UserEmail:   strings.TrimSpace(userEmail.String),
			APIKeyName:  strings.TrimSpace(apiKeyName.String),
			AccountName: strings.TrimSpace(accountName.String),
			GroupName:   strings.TrimSpace(groupName.String),

			Stream: stream,
		}

		if item.Platform == "" {
			item.Platform = "unknown"
		}

		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return out, total, nil
}
