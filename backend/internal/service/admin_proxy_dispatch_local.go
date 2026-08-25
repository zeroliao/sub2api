package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	openaiutil "github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"gopkg.in/yaml.v3"
)

type AccountProxyBinding struct {
	ID                int64      `json:"id"`
	IdentityKey       string     `json:"identity_key"`
	Platform          string     `json:"platform"`
	AccountID         *int64     `json:"account_id,omitempty"`
	ProxyID           int64      `json:"proxy_id"`
	Status            string     `json:"status"`
	Source            string     `json:"source"`
	FirstUsedAt       time.Time  `json:"first_used_at"`
	LastUsedAt        time.Time  `json:"last_used_at"`
	LastSuccessAt     *time.Time `json:"last_success_at,omitempty"`
	LastFailureAt     *time.Time `json:"last_failure_at,omitempty"`
	FailureCount      int        `json:"failure_count,omitempty"`
	LastFailureReason string     `json:"last_failure_reason,omitempty"`
	UseCount          int64      `json:"use_count"`
	Proxy             *Proxy     `json:"proxy,omitempty"`
}
type ProxyRelationship struct {
	AccountID          int64      `json:"account_id"`
	AccountName        string     `json:"account_name"`
	Platform           string     `json:"platform"`
	AccountType        string     `json:"account_type"`
	AccountStatus      string     `json:"account_status"`
	IdentityKey        string     `json:"identity_key"`
	CurrentProxy       *Proxy     `json:"current_proxy,omitempty"`
	ProxySource        string     `json:"proxy_source"`
	BindingStatus      string     `json:"binding_status"`
	BindingID          *int64     `json:"binding_id,omitempty"`
	LastUsedAt         *time.Time `json:"last_used_at,omitempty"`
	HistoryProxyCount  int64      `json:"history_proxy_count"`
	BoundAccountCount  int64      `json:"bound_account_count"`
	ActiveAccountCount int64      `json:"active_account_count"`
	CurrentConcurrency int64      `json:"current_concurrency"`
	LastSwitchReason   string     `json:"last_switch_reason,omitempty"`
	LastFailureReason  string     `json:"last_failure_reason,omitempty"`
	DirectFallbackMode string     `json:"direct_fallback_mode"`
	NoAvailableProxy   bool       `json:"no_available_proxy"`
}
type ProxyDispatchSettings struct {
	DirectFallbackMode string `json:"direct_fallback_mode"`
	AutoAssignEnabled  bool   `json:"auto_assign_enabled"`
}
type AbuseIPDBAPIKeySettings struct {
	Configured bool   `json:"configured"`
	Source     string `json:"source"`
}
type AbuseIPDBAPIKeySettingsInput struct {
	APIKey string `json:"api_key"`
	Clear  bool   `json:"clear"`
}
type ProxyImportPreviewInput struct {
	Content  string `json:"content"`
	URL      string `json:"url"`
	Provider string `json:"provider"`
}
type ProxyImportPreviewItem struct {
	Key             string `json:"key"`
	Name            string `json:"name"`
	Protocol        string `json:"protocol"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	Username        string `json:"username,omitempty"`
	Password        string `json:"password,omitempty"`
	Source          string `json:"source"`
	ProxyType       string `json:"proxy_type"`
	Provider        string `json:"provider,omitempty"`
	Region          string `json:"region,omitempty"`
	QualityStatus   string `json:"quality_status"`
	SidecarRequired bool   `json:"sidecar_required"`
	SidecarHint     string `json:"sidecar_hint,omitempty"`
	Duplicate       bool   `json:"duplicate"`
	Valid           bool   `json:"valid"`
	Error           string `json:"error,omitempty"`
	Selected        bool   `json:"selected"`
	Raw             string `json:"raw,omitempty"`
}
type ProxyImportPreview struct {
	Items          []ProxyImportPreviewItem `json:"items"`
	Total          int                      `json:"total"`
	Valid          int                      `json:"valid"`
	Duplicates     int                      `json:"duplicates"`
	SidecarOnly    int                      `json:"sidecar_only"`
	Recommended    int                      `json:"recommended"`
	SourceDetected string                   `json:"source_detected"`
}
type ProxyImportConfirmInput struct {
	Items []ProxyImportPreviewItem `json:"items"`
}
type ProxyImportConfirmResult struct {
	Created  int      `json:"created"`
	Skipped  int      `json:"skipped"`
	Failed   int      `json:"failed"`
	ProxyIDs []int64  `json:"proxy_ids"`
	Errors   []string `json:"errors,omitempty"`
}
type ProxySubscriptionSource struct {
	ID                         int64                     `json:"id"`
	Name                       string                    `json:"name"`
	URL                        string                    `json:"url"`
	SourceType                 string                    `json:"source_type"`
	Provider                   string                    `json:"provider,omitempty"`
	SyncEnabled                bool                      `json:"sync_enabled"`
	SyncIntervalMinutes        int                       `json:"sync_interval_minutes"`
	Strategy                   ProxySubscriptionStrategy `json:"strategy"`
	SidecarEnabled             bool                      `json:"sidecar_enabled"`
	Runtime                    string                    `json:"runtime"`
	PortStart                  int                       `json:"port_start"`
	PortEnd                    int                       `json:"port_end"`
	ScanEnabled                bool                      `json:"scan_enabled"`
	ScanIntervalMinutes        int                       `json:"scan_interval_minutes"`
	HealthCheckIntervalMinutes int                       `json:"health_check_interval_minutes"`
	ReputationProvider         string                    `json:"reputation_provider"`
	ReputationAPIKeyRef        string                    `json:"reputation_api_key_ref,omitempty"`
	LastSyncedAt               *time.Time                `json:"last_synced_at,omitempty"`
	LastScanAt                 *time.Time                `json:"last_scan_at,omitempty"`
	LastScanResult             map[string]any            `json:"last_scan_result,omitempty"`
	LastError                  string                    `json:"last_error,omitempty"`
	Status                     string                    `json:"status"`
	CreatedAt                  time.Time                 `json:"created_at"`
	UpdatedAt                  time.Time                 `json:"updated_at"`
}
type ProxySubscriptionSourceInput struct {
	Name                       string                    `json:"name"`
	URL                        string                    `json:"url"`
	SourceType                 string                    `json:"source_type"`
	Provider                   string                    `json:"provider"`
	SyncEnabled                *bool                     `json:"sync_enabled"`
	SyncIntervalMinutes        int                       `json:"sync_interval_minutes"`
	Strategy                   ProxySubscriptionStrategy `json:"strategy"`
	SidecarEnabled             *bool                     `json:"sidecar_enabled"`
	Runtime                    string                    `json:"runtime"`
	PortStart                  int                       `json:"port_start"`
	PortEnd                    int                       `json:"port_end"`
	ScanEnabled                *bool                     `json:"scan_enabled"`
	ScanIntervalMinutes        int                       `json:"scan_interval_minutes"`
	HealthCheckIntervalMinutes int                       `json:"health_check_interval_minutes"`
	ReputationProvider         string                    `json:"reputation_provider"`
	ReputationAPIKeyRef        string                    `json:"reputation_api_key_ref"`
	Status                     string                    `json:"status"`
}
type ProxySubscriptionStrategy struct {
	MaxParsedNodes          int      `json:"max_parsed_nodes"`
	MaxEnabledNodes         int      `json:"max_enabled_nodes"`
	MaxActiveSidecarNodes   int      `json:"max_active_sidecar_nodes"`
	MaxProbeConcurrency     int      `json:"max_probe_concurrency"`
	ScanBatchSize           int      `json:"scan_batch_size"`
	StandbyNodes            int      `json:"standby_nodes"`
	MinCountryCount         int      `json:"min_country_count"`
	MaxCountryCount         int      `json:"max_country_count"`
	MaxNodesPerCountry      int      `json:"max_nodes_per_country"`
	PreferredCountries      []string `json:"preferred_countries"`
	BlockedCountries        []string `json:"blocked_countries"`
	MaxLatencyMs            int      `json:"max_latency_ms"`
	MinIPCleanScore         int      `json:"min_ip_clean_score"`
	MinQualityScore         int      `json:"min_quality_score"`
	SelectionMode           string   `json:"selection_mode"`
	ReputationCacheHours    int      `json:"reputation_cache_hours"`
	ScanBudgetMinutes       int      `json:"scan_budget_minutes"`
	ScanBudgetMaxMinutes    int      `json:"scan_budget_max_minutes"`
	ResourceAdaptiveScan    bool     `json:"resource_adaptive_scan"`
	MinFreeMemoryMB         int      `json:"min_free_memory_mb"`
	PauseFreeMemoryMB       int      `json:"pause_free_memory_mb"`
	TimeoutSleepAfter       int      `json:"timeout_sleep_after"`
	SleepMinutes            int      `json:"sleep_minutes"`
	ReplaceSameCountryFirst bool     `json:"replace_same_country_first"`
}
type ProxySubscriptionNode struct {
	ID                  int64      `json:"id"`
	SourceID            int64      `json:"source_id"`
	NodeKey             string     `json:"node_key"`
	RawURI              string     `json:"raw_uri"`
	Name                string     `json:"name"`
	Protocol            string     `json:"protocol"`
	Server              string     `json:"server"`
	Port                int        `json:"port"`
	Username            string     `json:"username,omitempty"`
	CountryHint         string     `json:"country_hint,omitempty"`
	ExitIP              string     `json:"exit_ip,omitempty"`
	ExitCountry         string     `json:"exit_country,omitempty"`
	ExitCountryCode     string     `json:"exit_country_code,omitempty"`
	LatencyMs           *int       `json:"latency_ms,omitempty"`
	IPCleanScore        *int       `json:"ip_clean_score,omitempty"`
	ReputationProvider  string     `json:"reputation_provider,omitempty"`
	ReputationCheckedAt *time.Time `json:"reputation_checked_at,omitempty"`
	Score               int        `json:"score"`
	Status              string     `json:"status"`
	FailureCount        int        `json:"failure_count"`
	TimeoutCount        int        `json:"timeout_count"`
	SleepUntil          *time.Time `json:"sleep_until,omitempty"`
	LastScannedAt       *time.Time `json:"last_scanned_at,omitempty"`
	LastError           string     `json:"last_error,omitempty"`
	Selected            bool       `json:"selected"`
	SidecarRequired     bool       `json:"sidecar_required"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}
type ProxySubscriptionScanResult struct {
	SourceID         int64                     `json:"source_id"`
	Total            int                       `json:"total"`
	Parsed           int                       `json:"parsed"`
	Saved            int                       `json:"saved"`
	Selected         int                       `json:"selected"`
	SidecarRequired  int                       `json:"sidecar_required"`
	DirectImportable int                       `json:"direct_importable"`
	Skipped          int                       `json:"skipped"`
	Errors           []string                  `json:"errors,omitempty"`
	Strategy         ProxySubscriptionStrategy `json:"strategy"`
	ScannedAt        time.Time                 `json:"scanned_at"`
}
type ProxySubscriptionScanStatus struct {
	Active               bool       `json:"active"`
	SourceID             int64      `json:"source_id,omitempty"`
	SourceName           string     `json:"source_name,omitempty"`
	StartedAt            *time.Time `json:"started_at,omitempty"`
	ElapsedSeconds       int        `json:"elapsed_seconds,omitempty"`
	ScanBudgetMinutes    int        `json:"scan_budget_minutes,omitempty"`
	ScanBudgetMaxMinutes int        `json:"scan_budget_max_minutes,omitempty"`
}

const allocateProxySidecarPortSQL = `
SELECT candidate
FROM generate_series($1::int, $2::int) AS candidate
WHERE NOT EXISTS (
  SELECT 1 FROM proxy_sidecar_endpoints
  WHERE listen_port = candidate AND deleted_at IS NULL
)
ORDER BY candidate
LIMIT 1`

type proxySubscriptionNodeEvaluation struct {
	Key                 string
	Country             string
	ExitIP              string
	ExitCountry         string
	ExitCountryCode     string
	LatencyMs           *int
	IPCleanScore        *int
	ReputationProvider  string
	ReputationCheckedAt *time.Time
	ReputationRaw       map[string]any
	FailureCount        int
	TimeoutCount        int
	SleepUntil          *time.Time
	TimedOut            bool
	Score               int
	LastError           string
}
type proxyIPReputationResult struct {
	IP          string
	CleanScore  int
	Country     string
	CountryCode string
	Provider    string
	CheckedAt   time.Time
	Raw         map[string]any
}

func (s *adminServiceImpl) ListProxyRelationships(ctx context.Context, page, pageSize int, platform, status, search string) ([]ProxyRelationship, int64, error) {
	if s == nil || s.entClient == nil {
		return nil, 0, infraerrors.ServiceUnavailable("PROXY_DISPATCH_UNAVAILABLE", "proxy dispatch service unavailable")
	}
	accounts, total, err := s.ListAccounts(ctx, page, pageSize, platform, "", status, search, 0, "", "id", "desc")
	if err != nil {
		return nil, 0, err
	}
	settings, _ := s.GetProxyDispatchSettings(ctx)
	out := make([]ProxyRelationship, 0, len(accounts))
	for i := range accounts {
		rel, err := s.proxyRelationshipForAccount(ctx, &accounts[i])
		if err != nil {
			return nil, 0, err
		}
		if settings != nil {
			rel.DirectFallbackMode = settings.DirectFallbackMode
		}
		out = append(out, *rel)
	}
	return out, total, nil
}
func (s *adminServiceImpl) ReassignAccountProxy(ctx context.Context, accountID int64) (*ProxyRelationship, error) {
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if _, err := s.assignProxyForAccount(ctx, account, true); err != nil {
		return nil, err
	}
	account, _ = s.accountRepo.GetByID(ctx, accountID)
	return s.proxyRelationshipForAccount(ctx, account)
}
func (s *adminServiceImpl) RestoreAccountProxyHistory(ctx context.Context, accountID int64) (*ProxyRelationship, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.ServiceUnavailable("PROXY_DISPATCH_UNAVAILABLE", "proxy dispatch service unavailable")
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	identityKey := accountProxyIdentityKey(account)
	if identityKey == "" {
		return nil, infraerrors.BadRequest("ACCOUNT_IDENTITY_UNAVAILABLE", "account identity is unavailable")
	}
	proxyID, ok, err := s.findHistoricalProxy(ctx, identityKey)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, infraerrors.ServiceUnavailable("NO_AVAILABLE_PROXY", "no available historical proxy")
	}
	account.ProxyID = &proxyID
	account.Proxy = nil
	if err := s.accountRepo.Update(ctx, account); err != nil {
		return nil, err
	}
	if err := s.recordAccountProxyBinding(ctx, account, proxyID, ProxyBindingSourceRestored, ProxyBindingStatusActive); err != nil {
		return nil, err
	}
	account, _ = s.accountRepo.GetByID(ctx, accountID)
	return s.proxyRelationshipForAccount(ctx, account)
}
func (s *adminServiceImpl) ReportAccountProxyFailure(ctx context.Context, accountID int64, reason string) (*ProxyRelationship, error) {
	if s == nil || s.entClient == nil || s.accountRepo == nil {
		return nil, infraerrors.ServiceUnavailable("PROXY_DISPATCH_UNAVAILABLE", "proxy dispatch service unavailable")
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account.ProxyID == nil || *account.ProxyID <= 0 {
		return s.proxyRelationshipForAccount(ctx, account)
	}
	identityKey := accountProxyIdentityKey(account)
	if identityKey == "" {
		return s.proxyRelationshipForAccount(ctx, account)
	}
	currentProxyID := *account.ProxyID
	reason = truncateProxyFailureReason(reason)
	var failureCount int
	rows, err := s.entClient.QueryContext(ctx, `
INSERT INTO account_proxy_bindings (identity_key, platform, account_id, proxy_id, status, source, first_used_at, last_used_at, last_failure_at, failure_count, last_failure_reason, use_count, created_at, updated_at)
VALUES ($1, $2, $3, $4, 'active', 'auto', NOW(), NOW(), NOW(), 1, NULLIF($5, ''), 1, NOW(), NOW())
ON CONFLICT (identity_key, proxy_id)
DO UPDATE SET account_id = EXCLUDED.account_id,
              platform = EXCLUDED.platform,
              last_used_at = NOW(),
              last_failure_at = NOW(),
              failure_count = account_proxy_bindings.failure_count + 1,
              last_failure_reason = EXCLUDED.last_failure_reason,
              updated_at = NOW()
RETURNING failure_count`, identityKey, account.Platform, account.ID, currentProxyID, reason)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		if err := rows.Scan(&failureCount); err != nil {
			_ = rows.Close()
			return nil, err
		}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	_, _ = s.entClient.ExecContext(ctx, `
UPDATE proxies
SET failure_count = COALESCE(failure_count, 0) + 1,
    last_checked_at = NOW(),
    quality_status = CASE
      WHEN COALESCE(failure_count, 0) + 1 >= $2 THEN 'cooling'
      ELSE quality_status
    END
WHERE id = $1`, currentProxyID, accountProxyFailureReassignThreshold)
	if failureCount < accountProxyFailureReassignThreshold {
		return s.proxyRelationshipForAccount(ctx, account)
	}
	_, err = s.entClient.ExecContext(ctx, `
UPDATE account_proxy_bindings
SET status = 'proxy_unavailable',
    last_failure_at = NOW(),
    last_failure_reason = NULLIF($3, ''),
    updated_at = NOW()
WHERE identity_key = $1 AND proxy_id = $2`, identityKey, currentProxyID, reason)
	if err != nil {
		return nil, err
	}
	if proxy, err := s.chooseReplacementProxy(ctx, identityKey, currentProxyID); err == nil {
		account.ProxyID = &proxy.ID
		account.Proxy = proxy
		if updateErr := s.accountRepo.Update(ctx, account); updateErr != nil {
			return nil, updateErr
		}
		if updateErr := s.recordAccountProxyBinding(ctx, account, proxy.ID, ProxyBindingSourceAuto, ProxyBindingStatusActive); updateErr != nil {
			return nil, updateErr
		}
		rel, relErr := s.proxyRelationshipForAccount(ctx, account)
		if rel != nil {
			rel.LastSwitchReason = "previous proxy failed repeatedly"
			rel.LastFailureReason = reason
		}
		return rel, relErr
	}
	if runtimeDirectFallbackMode(ctx, s.settingService) == DirectFallbackGlobal {
		account.ProxyID = nil
		account.Proxy = nil
		if updateErr := s.accountRepo.Update(ctx, account); updateErr != nil {
			return nil, updateErr
		}
		rel, relErr := s.proxyRelationshipForAccount(ctx, account)
		if rel != nil {
			rel.LastSwitchReason = "all proxies unavailable; using direct fallback"
			rel.LastFailureReason = reason
			rel.DirectFallbackMode = DirectFallbackGlobal
		}
		return rel, relErr
	}
	rel, relErr := s.proxyRelationshipForAccount(ctx, account)
	if rel != nil {
		rel.LastFailureReason = reason
		rel.NoAvailableProxy = true
	}
	return rel, relErr
}
func (s *adminServiceImpl) RecordAccountProxySuccess(ctx context.Context, accountID int64) error {
	if s == nil || s.entClient == nil || s.accountRepo == nil {
		return nil
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil || account.ProxyID == nil || *account.ProxyID <= 0 {
		return nil
	}
	identityKey := accountProxyIdentityKey(account)
	if identityKey == "" {
		return nil
	}
	_, err = s.entClient.ExecContext(ctx, `
UPDATE account_proxy_bindings
SET status = 'active',
    last_success_at = NOW(),
    failure_count = 0,
    last_failure_reason = NULL,
    updated_at = NOW()
WHERE identity_key = $1 AND proxy_id = $2`, identityKey, *account.ProxyID)
	if err != nil {
		return err
	}
	_, _ = s.entClient.ExecContext(ctx, `
UPDATE proxies
SET failure_count = 0,
    quality_status = CASE WHEN quality_status = 'cooling' THEN 'healthy' ELSE quality_status END,
    last_checked_at = NOW()
WHERE id = $1`, *account.ProxyID)
	return nil
}
func (s *adminServiceImpl) GetAccountProxyHistory(ctx context.Context, accountID int64) ([]AccountProxyBinding, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.ServiceUnavailable("PROXY_DISPATCH_UNAVAILABLE", "proxy dispatch service unavailable")
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	identityKey := accountProxyIdentityKey(account)
	if identityKey == "" {
		return []AccountProxyBinding{}, nil
	}
	return s.listProxyBindingsByIdentity(ctx, identityKey)
}
func (s *adminServiceImpl) GetProxyDispatchSettings(ctx context.Context) (*ProxyDispatchSettings, error) {
	defaults := &ProxyDispatchSettings{DirectFallbackMode: DirectFallbackOff, AutoAssignEnabled: true}
	if s == nil || s.settingService == nil || s.settingService.settingRepo == nil {
		return defaults, nil
	}
	raw, err := s.settingService.settingRepo.GetValue(ctx, SettingKeyProxyDispatchSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return defaults, nil
		}
		return nil, err
	}
	if strings.TrimSpace(raw) == "" {
		return defaults, nil
	}
	if err := json.Unmarshal([]byte(raw), defaults); err != nil {
		return nil, err
	}
	defaults.DirectFallbackMode = normalizeDirectFallbackMode(defaults.DirectFallbackMode)
	return defaults, nil
}
func (s *adminServiceImpl) UpdateProxyDispatchSettings(ctx context.Context, input *ProxyDispatchSettings) (*ProxyDispatchSettings, error) {
	if input == nil {
		input = &ProxyDispatchSettings{}
	}
	settings := &ProxyDispatchSettings{DirectFallbackMode: normalizeDirectFallbackMode(input.DirectFallbackMode), AutoAssignEnabled: input.AutoAssignEnabled}
	if s == nil || s.settingService == nil || s.settingService.settingRepo == nil {
		return nil, infraerrors.ServiceUnavailable("SETTING_SERVICE_UNAVAILABLE", "setting service unavailable")
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}
	if err := s.settingService.settingRepo.Set(ctx, SettingKeyProxyDispatchSettings, string(data)); err != nil {
		return nil, err
	}
	return settings, nil
}
func (s *adminServiceImpl) GetAbuseIPDBAPIKeySettings(ctx context.Context) (*AbuseIPDBAPIKeySettings, error) {
	if s == nil || s.settingService == nil || s.settingService.settingRepo == nil {
		return &AbuseIPDBAPIKeySettings{}, nil
	}
	value, _ := s.settingService.settingRepo.GetValue(ctx, SettingKeyAbuseIPDBAPIKey)
	if strings.TrimSpace(value) != "" {
		return &AbuseIPDBAPIKeySettings{Configured: true, Source: "database"}, nil
	}
	if strings.TrimSpace(os.Getenv("ABUSEIPDB_API_KEY")) != "" {
		return &AbuseIPDBAPIKeySettings{Configured: true, Source: "environment"}, nil
	}
	return &AbuseIPDBAPIKeySettings{}, nil
}
func (s *adminServiceImpl) UpdateAbuseIPDBAPIKeySettings(ctx context.Context, input *AbuseIPDBAPIKeySettingsInput) (*AbuseIPDBAPIKeySettings, error) {
	if s == nil || s.settingService == nil || s.settingService.settingRepo == nil {
		return nil, infraerrors.ServiceUnavailable("SETTING_UNAVAILABLE", "setting service unavailable")
	}
	if input == nil {
		input = &AbuseIPDBAPIKeySettingsInput{}
	}
	key := strings.TrimSpace(input.APIKey)
	switch {
	case input.Clear:
		if err := s.settingService.settingRepo.Delete(ctx, SettingKeyAbuseIPDBAPIKey); err != nil {
			return nil, err
		}
	case key != "":
		if err := s.settingService.settingRepo.Set(ctx, SettingKeyAbuseIPDBAPIKey, key); err != nil {
			return nil, err
		}
	}
	return s.GetAbuseIPDBAPIKeySettings(ctx)
}
func (s *adminServiceImpl) PreviewProxyImport(ctx context.Context, input ProxyImportPreviewInput) (*ProxyImportPreview, error) {
	content := strings.TrimSpace(input.Content)
	sourceDetected := "text"
	if strings.TrimSpace(input.URL) != "" {
		body, err := fetchProxySubscription(ctx, input.URL)
		if err != nil {
			return nil, err
		}
		content = body
		sourceDetected = "subscription_url"
	} else if looksLikeSubscriptionURL(content) {
		body, err := fetchProxySubscription(ctx, content)
		if err != nil {
			return nil, err
		}
		content = body
		sourceDetected = "subscription_url"
	}
	if decoded := decodeMaybeBase64Subscription(content); decoded != "" {
		content = decoded
		if sourceDetected == "text" {
			sourceDetected = "base64_subscription"
		}
	}
	items := parseProxyImportItems(content, strings.TrimSpace(input.Provider))
	if strings.Contains(content, "proxies:") {
		sourceDetected = "clash_yaml"
	} else if strings.Contains(content, `"outbounds"`) {
		sourceDetected = "sing_box_json"
	}
	for i := range items {
		if items[i].Key == "" {
			items[i].Key = proxyImportItemKey(items[i])
		}
		if items[i].QualityStatus == "" {
			items[i].QualityStatus = ProxyQualityHealthy
		}
		if items[i].Source == "" {
			items[i].Source = "import"
		}
		if items[i].ProxyType == "" {
			items[i].ProxyType = "datacenter"
		}
		if items[i].Provider == "" {
			items[i].Provider = strings.TrimSpace(input.Provider)
		}
		items[i].Duplicate, _ = s.CheckProxyExists(ctx, items[i].Host, items[i].Port, items[i].Username, items[i].Password)
		items[i].Selected = items[i].Valid && !items[i].Duplicate && !items[i].SidecarRequired
	}
	preview := &ProxyImportPreview{Items: items, Total: len(items), SourceDetected: sourceDetected}
	for _, item := range items {
		if item.Valid {
			preview.Valid++
		}
		if item.Duplicate {
			preview.Duplicates++
		}
		if item.SidecarRequired {
			preview.SidecarOnly++
		}
		if item.Selected {
			preview.Recommended++
		}
	}
	return preview, nil
}
func (s *adminServiceImpl) ConfirmProxyImport(ctx context.Context, input ProxyImportConfirmInput) (*ProxyImportConfirmResult, error) {
	result := &ProxyImportConfirmResult{}
	for _, item := range input.Items {
		if !item.Valid || item.SidecarRequired || !item.Selected {
			result.Skipped++
			continue
		}
		exists, err := s.CheckProxyExists(ctx, item.Host, item.Port, item.Username, item.Password)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, err.Error())
			continue
		}
		if exists {
			result.Skipped++
			continue
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = fmt.Sprintf("%s:%d", item.Host, item.Port)
		}
		proxy, err := s.CreateProxy(ctx, &CreateProxyInput{Name: name, Protocol: item.Protocol, Host: item.Host, Port: item.Port, Username: item.Username, Password: item.Password, Source: item.Source, ProxyType: item.ProxyType, Provider: item.Provider, Region: item.Region, QualityStatus: item.QualityStatus, Weight: 100})
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, err.Error())
			continue
		}
		result.Created++
		result.ProxyIDs = append(result.ProxyIDs, proxy.ID)
	}
	return result, nil
}
func (s *adminServiceImpl) BatchHealthCheckProxies(ctx context.Context, ids []int64) ([]ProxyTestResult, error) {
	if len(ids) == 0 {
		proxies, err := s.GetAllProxies(ctx)
		if err != nil {
			return nil, err
		}
		for _, proxy := range proxies {
			ids = append(ids, proxy.ID)
		}
	}
	results := make([]ProxyTestResult, 0, len(ids))
	for _, id := range ids {
		result, err := s.TestProxy(ctx, id)
		if err != nil {
			results = append(results, ProxyTestResult{Success: false, Message: err.Error()})
			continue
		}
		results = append(results, *result)
	}
	return results, nil
}
func (s *adminServiceImpl) ListProxySubscriptionSources(ctx context.Context) ([]ProxySubscriptionSource, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.ServiceUnavailable("PROXY_SUBSCRIPTION_UNAVAILABLE", "proxy subscription service unavailable")
	}
	rows, err := s.entClient.QueryContext(ctx, `
SELECT id, name, url, source_type, COALESCE(provider, ''), sync_enabled, sync_interval_minutes,
       COALESCE(strategy_json::text, '{}'), sidecar_enabled, runtime, port_start, port_end,
       scan_enabled, scan_interval_minutes, health_check_interval_minutes, reputation_provider,
       COALESCE(reputation_api_key_ref, ''), last_synced_at, last_scan_at,
       COALESCE(last_scan_result::text, '{}'), COALESCE(last_error, ''), status, created_at, updated_at
FROM proxy_subscription_sources
WHERE deleted_at IS NULL
ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var out []ProxySubscriptionSource
	for rows.Next() {
		var item ProxySubscriptionSource
		var strategyRaw, scanResultRaw string
		if err := rows.Scan(&item.ID, &item.Name, &item.URL, &item.SourceType, &item.Provider, &item.SyncEnabled, &item.SyncIntervalMinutes, &strategyRaw, &item.SidecarEnabled, &item.Runtime, &item.PortStart, &item.PortEnd, &item.ScanEnabled, &item.ScanIntervalMinutes, &item.HealthCheckIntervalMinutes, &item.ReputationProvider, &item.ReputationAPIKeyRef, &item.LastSyncedAt, &item.LastScanAt, &scanResultRaw, &item.LastError, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Strategy = parseProxySubscriptionStrategy(strategyRaw)
		item.LastScanResult = parseJSONMap(scanResultRaw)
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *adminServiceImpl) CreateProxySubscriptionSource(ctx context.Context, input ProxySubscriptionSourceInput) (*ProxySubscriptionSource, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.ServiceUnavailable("PROXY_SUBSCRIPTION_UNAVAILABLE", "proxy subscription service unavailable")
	}
	input = normalizeProxySubscriptionInput(input)
	strategyRaw, err := json.Marshal(input.Strategy)
	if err != nil {
		return nil, err
	}
	rows, err := s.entClient.QueryContext(ctx, `
INSERT INTO proxy_subscription_sources (
  name, url, source_type, provider, sync_enabled, sync_interval_minutes,
  strategy_json, sidecar_enabled, runtime, port_start, port_end, scan_enabled,
  scan_interval_minutes, health_check_interval_minutes, reputation_provider,
  reputation_api_key_ref, status, created_at, updated_at
)
VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7::jsonb, $8, $9, $10, $11, $12, $13, $14, $15, NULLIF($16, ''), $17, NOW(), NOW())
RETURNING id, name, url, source_type, COALESCE(provider, ''), sync_enabled, sync_interval_minutes,
          COALESCE(strategy_json::text, '{}'), sidecar_enabled, runtime, port_start, port_end,
          scan_enabled, scan_interval_minutes, health_check_interval_minutes, reputation_provider,
          COALESCE(reputation_api_key_ref, ''), last_synced_at, last_scan_at,
          COALESCE(last_scan_result::text, '{}'), COALESCE(last_error, ''), status, created_at, updated_at`, input.Name, input.URL, input.SourceType, input.Provider, *input.SyncEnabled, input.SyncIntervalMinutes, string(strategyRaw), *input.SidecarEnabled, input.Runtime, input.PortStart, input.PortEnd, *input.ScanEnabled, input.ScanIntervalMinutes, input.HealthCheckIntervalMinutes, input.ReputationProvider, input.ReputationAPIKeyRef, input.Status)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	if rows.Next() {
		var item ProxySubscriptionSource
		var strategyRaw, scanResultRaw string
		if err := rows.Scan(&item.ID, &item.Name, &item.URL, &item.SourceType, &item.Provider, &item.SyncEnabled, &item.SyncIntervalMinutes, &strategyRaw, &item.SidecarEnabled, &item.Runtime, &item.PortStart, &item.PortEnd, &item.ScanEnabled, &item.ScanIntervalMinutes, &item.HealthCheckIntervalMinutes, &item.ReputationProvider, &item.ReputationAPIKeyRef, &item.LastSyncedAt, &item.LastScanAt, &scanResultRaw, &item.LastError, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Strategy = parseProxySubscriptionStrategy(strategyRaw)
		item.LastScanResult = parseJSONMap(scanResultRaw)
		return &item, nil
	}
	return nil, rows.Err()
}
func (s *adminServiceImpl) UpdateProxySubscriptionSource(ctx context.Context, id int64, input ProxySubscriptionSourceInput) (*ProxySubscriptionSource, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.ServiceUnavailable("PROXY_SUBSCRIPTION_UNAVAILABLE", "proxy subscription service unavailable")
	}
	input = normalizeProxySubscriptionInput(input)
	strategyRaw, err := json.Marshal(input.Strategy)
	if err != nil {
		return nil, err
	}
	rows, err := s.entClient.QueryContext(ctx, `
UPDATE proxy_subscription_sources
SET name = $2, url = $3, source_type = $4, provider = NULLIF($5, ''),
    sync_enabled = $6, sync_interval_minutes = $7, strategy_json = $8::jsonb,
    sidecar_enabled = $9, runtime = $10, port_start = $11, port_end = $12,
    scan_enabled = $13, scan_interval_minutes = $14, health_check_interval_minutes = $15,
    reputation_provider = $16, reputation_api_key_ref = NULLIF($17, ''), status = $18,
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, name, url, source_type, COALESCE(provider, ''), sync_enabled, sync_interval_minutes,
          COALESCE(strategy_json::text, '{}'), sidecar_enabled, runtime, port_start, port_end,
          scan_enabled, scan_interval_minutes, health_check_interval_minutes, reputation_provider,
          COALESCE(reputation_api_key_ref, ''), last_synced_at, last_scan_at,
          COALESCE(last_scan_result::text, '{}'), COALESCE(last_error, ''), status, created_at, updated_at`, id, input.Name, input.URL, input.SourceType, input.Provider, *input.SyncEnabled, input.SyncIntervalMinutes, string(strategyRaw), *input.SidecarEnabled, input.Runtime, input.PortStart, input.PortEnd, *input.ScanEnabled, input.ScanIntervalMinutes, input.HealthCheckIntervalMinutes, input.ReputationProvider, input.ReputationAPIKeyRef, input.Status)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	if rows.Next() {
		var item ProxySubscriptionSource
		var strategyRaw, scanResultRaw string
		if err := rows.Scan(&item.ID, &item.Name, &item.URL, &item.SourceType, &item.Provider, &item.SyncEnabled, &item.SyncIntervalMinutes, &strategyRaw, &item.SidecarEnabled, &item.Runtime, &item.PortStart, &item.PortEnd, &item.ScanEnabled, &item.ScanIntervalMinutes, &item.HealthCheckIntervalMinutes, &item.ReputationProvider, &item.ReputationAPIKeyRef, &item.LastSyncedAt, &item.LastScanAt, &scanResultRaw, &item.LastError, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.Strategy = parseProxySubscriptionStrategy(strategyRaw)
		item.LastScanResult = parseJSONMap(scanResultRaw)
		return &item, nil
	}
	return nil, ErrProxyNotFound
}
func (s *adminServiceImpl) DeleteProxySubscriptionSource(ctx context.Context, id int64) error {
	if s == nil || s.entClient == nil {
		return infraerrors.ServiceUnavailable("PROXY_SUBSCRIPTION_UNAVAILABLE", "proxy subscription service unavailable")
	}
	if err := s.retireProxySubscriptionSourceResources(ctx, id, "subscription source deleted"); err != nil {
		return err
	}
	_, err := s.entClient.ExecContext(ctx, `UPDATE proxy_subscription_sources SET deleted_at = NOW(), updated_at = NOW(), status = 'inactive' WHERE id = $1 AND deleted_at IS NULL`, id)
	return err
}
func (s *adminServiceImpl) SyncProxySubscriptionSource(ctx context.Context, id int64) (*ProxyImportPreview, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.ServiceUnavailable("PROXY_SUBSCRIPTION_UNAVAILABLE", "proxy subscription service unavailable")
	}
	rows, err := s.entClient.QueryContext(ctx, `SELECT url, COALESCE(provider, '') FROM proxy_subscription_sources WHERE id = $1 AND deleted_at IS NULL AND status = 'active'`, id)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var url, provider string
	if rows.Next() {
		if err := rows.Scan(&url, &provider); err != nil {
			return nil, err
		}
	} else {
		return nil, ErrProxyNotFound
	}
	preview, err := s.PreviewProxyImport(ctx, ProxyImportPreviewInput{URL: url, Provider: provider})
	lastErr := ""
	if err != nil {
		lastErr = err.Error()
	}
	_, _ = s.entClient.ExecContext(ctx, `UPDATE proxy_subscription_sources SET last_synced_at = NOW(), last_error = NULLIF($2, ''), updated_at = NOW() WHERE id = $1`, id, lastErr)
	return preview, err
}
func (s *adminServiceImpl) ScanProxySubscriptionSource(ctx context.Context, id int64) (*ProxySubscriptionScanResult, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.ServiceUnavailable("PROXY_SUBSCRIPTION_UNAVAILABLE", "proxy subscription service unavailable")
	}
	if err := s.tryStartProxySubscriptionScan(id); err != nil {
		return nil, err
	}
	return s.scanProxySubscriptionSource(ctx, id)
}

func (s *adminServiceImpl) StartProxySubscriptionScan(ctx context.Context, id int64) (*ProxySubscriptionScanStatus, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.ServiceUnavailable("PROXY_SUBSCRIPTION_UNAVAILABLE", "proxy subscription service unavailable")
	}
	source, err := s.getProxySubscriptionSourceForScan(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.tryStartProxySubscriptionScan(id); err != nil {
		return nil, err
	}
	status := s.proxySubscriptionScanStatus(source)
	go func() {
		if _, scanErr := s.scanProxySubscriptionSource(context.Background(), id); scanErr != nil {
			logger.LegacyPrintf("service.proxy_subscription_scan", "[ProxySubscriptionScan] source=%d failed: %v", id, scanErr)
		}
	}()
	return status, nil
}

func (s *adminServiceImpl) scanProxySubscriptionSource(ctx context.Context, id int64) (*ProxySubscriptionScanResult, error) {
	defer s.finishProxySubscriptionScan()
	source, err := s.getProxySubscriptionSourceForScan(ctx, id)
	if err != nil {
		return nil, err
	}
	strategy := normalizeProxySubscriptionStrategy(source.Strategy)
	scanTimeout := time.Duration(strategy.ScanBudgetMaxMinutes) * time.Minute
	if scanTimeout <= 0 {
		scanTimeout = 40 * time.Minute
	}
	scanCtx, cancel := context.WithTimeout(ctx, scanTimeout)
	defer cancel()
	preview, err := s.PreviewProxyImport(scanCtx, ProxyImportPreviewInput{URL: source.URL, Provider: source.Provider})
	if err != nil {
		_, _ = s.entClient.ExecContext(ctx, `UPDATE proxy_subscription_sources SET last_scan_at = NOW(), last_error = $2, updated_at = NOW() WHERE id = $1`, id, err.Error())
		return nil, err
	}
	items := preview.Items
	if strategy.MaxParsedNodes > 0 && len(items) > strategy.MaxParsedNodes {
		items = items[:strategy.MaxParsedNodes]
	}
	existingNodes, err := s.loadProxySubscriptionNodeState(scanCtx, id)
	if err != nil {
		return nil, err
	}
	evaluations := s.evaluateProxySubscriptionItems(scanCtx, source, items, strategy, existingNodes)
	selectedStatuses := selectProxySubscriptionItems(items, source, strategy, evaluations)
	result := &ProxySubscriptionScanResult{SourceID: id, Total: preview.Total, Parsed: len(items), Strategy: strategy, ScannedAt: time.Now()}
	sidecarCount := 0
	activeKeys := make(map[string]struct{}, len(items))
	for _, item := range items {
		if scanCtx.Err() != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("scan budget reached: %v", scanCtx.Err()))
			break
		}
		if !item.Valid {
			result.Skipped++
			if item.Error != "" {
				result.Errors = append(result.Errors, item.Error)
			}
			continue
		}
		key := item.Key
		if key == "" {
			key = proxyImportItemKey(item)
		}
		activeKeys[key] = struct{}{}
		eval, ok := evaluations[key]
		if !ok {
			eval = proxySubscriptionNodeEvaluation{Key: key, Country: inferProxySubscriptionCountry(item)}
		}
		status := defaultString(selectedStatuses[key], "candidate")
		if item.SidecarRequired && !isSupportedSubscriptionSidecarProtocol(item.Protocol) {
			status = "unsupported"
			if eval.LastError == "" {
				eval.LastError = fmt.Sprintf("sidecar protocol %s is not supported by the configured runtime", item.Protocol)
			}
		}
		if item.SidecarRequired && !source.SidecarEnabled {
			status = "sidecar_disabled"
		}
		if eval.SleepUntil != nil && eval.SleepUntil.After(result.ScannedAt) {
			status = "sleeping"
		}
		if eval.TimedOut && status == "candidate" {
			status = "timeout"
		}
		if eval.LastError != "" && status == "candidate" {
			status = "degraded"
		}
		isSelected := status == "selected"
		nodeID, err := s.upsertProxySubscriptionNode(scanCtx, id, item, key, eval, status, isSelected)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			continue
		}
		result.Saved++
		if isSelected {
			result.Selected++
		}
		if item.SidecarRequired {
			result.SidecarRequired++
			if isSelected && source.SidecarEnabled && sidecarCount < strategy.MaxActiveSidecarNodes {
				port, portErr := s.allocateProxySidecarPort(scanCtx, source, nodeID)
				if portErr != nil {
					result.Errors = append(result.Errors, portErr.Error())
				} else if err := s.reserveProxySidecarEndpoint(scanCtx, source, nodeID, port); err != nil {
					result.Errors = append(result.Errors, err.Error())
				} else if err := s.upsertSidecarProxyForSubscriptionNode(scanCtx, source, nodeID, item, eval, port); err != nil {
					result.Errors = append(result.Errors, err.Error())
				} else {
					sidecarCount++
				}
			}
		} else {
			result.DirectImportable++
			if isSelected {
				if err := s.upsertDirectProxyFromSubscriptionNode(scanCtx, source, item, eval); err != nil {
					result.Errors = append(result.Errors, err.Error())
				}
			}
		}
	}
	if scanCtx.Err() == nil {
		if err := s.markMissingProxySubscriptionNodes(scanCtx, id, activeKeys); err != nil {
			result.Errors = append(result.Errors, err.Error())
		}
	} else {
		result.Errors = append(result.Errors, "scan finished before all nodes were processed, missing-node cleanup skipped")
	}
	if err := s.saveProxySubscriptionScanResult(scanCtx, id, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *adminServiceImpl) proxySubscriptionScanStatus(source *ProxySubscriptionSource) *ProxySubscriptionScanStatus {
	status := &ProxySubscriptionScanStatus{Active: true}
	if source != nil {
		status.SourceID = source.ID
		status.SourceName = source.Name
		strategy := normalizeProxySubscriptionStrategy(source.Strategy)
		status.ScanBudgetMinutes = strategy.ScanBudgetMinutes
		status.ScanBudgetMaxMinutes = strategy.ScanBudgetMaxMinutes
	}
	s.scanStateMu.Lock()
	startedAt := s.scanStartedAt
	s.scanStateMu.Unlock()
	if !startedAt.IsZero() {
		status.StartedAt = &startedAt
		status.ElapsedSeconds = int(time.Since(startedAt).Seconds())
	}
	return status
}

func (s *adminServiceImpl) GetProxySubscriptionScanStatus(ctx context.Context) (*ProxySubscriptionScanStatus, error) {
	status := &ProxySubscriptionScanStatus{}
	if s == nil {
		return status, nil
	}
	s.scanStateMu.Lock()
	active := s.scanActive
	sourceID := s.scanActiveSourceID
	startedAt := s.scanStartedAt
	s.scanStateMu.Unlock()
	if !active || sourceID <= 0 {
		return status, nil
	}
	status.Active = true
	status.SourceID = sourceID
	if !startedAt.IsZero() {
		status.StartedAt = &startedAt
		status.ElapsedSeconds = int(time.Since(startedAt).Seconds())
	}
	if s.entClient == nil {
		return status, nil
	}
	source, err := s.getProxySubscriptionSourceForScan(ctx, sourceID)
	if err != nil {
		return status, nil
	}
	status.SourceName = source.Name
	strategy := normalizeProxySubscriptionStrategy(source.Strategy)
	status.ScanBudgetMinutes = strategy.ScanBudgetMinutes
	status.ScanBudgetMaxMinutes = strategy.ScanBudgetMaxMinutes
	return status, nil
}
func (s *adminServiceImpl) ListProxySubscriptionNodes(ctx context.Context, sourceID int64) ([]ProxySubscriptionNode, error) {
	if s == nil || s.entClient == nil {
		return nil, infraerrors.ServiceUnavailable("PROXY_SUBSCRIPTION_UNAVAILABLE", "proxy subscription service unavailable")
	}
	rows, err := s.entClient.QueryContext(ctx, `
SELECT id, source_id, node_key, raw_uri, name, protocol, server, port, COALESCE(username, ''),
       COALESCE(country_hint, ''), COALESCE(exit_ip, ''), COALESCE(exit_country, ''),
       COALESCE(exit_country_code, ''), latency_ms, ip_clean_score, COALESCE(reputation_provider, ''),
       reputation_checked_at, score, status, failure_count, timeout_count, sleep_until,
       last_scanned_at, COALESCE(last_error, ''), selected, sidecar_required, created_at, updated_at
FROM proxy_subscription_nodes
WHERE source_id = $1 AND deleted_at IS NULL
ORDER BY selected DESC, score DESC, id ASC`, sourceID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var nodes []ProxySubscriptionNode
	for rows.Next() {
		var n ProxySubscriptionNode
		if err := rows.Scan(&n.ID, &n.SourceID, &n.NodeKey, &n.RawURI, &n.Name, &n.Protocol, &n.Server, &n.Port, &n.Username, &n.CountryHint, &n.ExitIP, &n.ExitCountry, &n.ExitCountryCode, &n.LatencyMs, &n.IPCleanScore, &n.ReputationProvider, &n.ReputationCheckedAt, &n.Score, &n.Status, &n.FailureCount, &n.TimeoutCount, &n.SleepUntil, &n.LastScannedAt, &n.LastError, &n.Selected, &n.SidecarRequired, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}
func (s *adminServiceImpl) getProxySubscriptionSourceForScan(ctx context.Context, id int64) (*ProxySubscriptionSource, error) {
	rows, err := s.entClient.QueryContext(ctx, `
SELECT id, name, url, source_type, COALESCE(provider, ''), sync_enabled, sync_interval_minutes,
       COALESCE(strategy_json::text, '{}'), sidecar_enabled, runtime, port_start, port_end,
       scan_enabled, scan_interval_minutes, health_check_interval_minutes, reputation_provider,
       COALESCE(reputation_api_key_ref, ''), last_synced_at, last_scan_at,
       COALESCE(last_scan_result::text, '{}'), COALESCE(last_error, ''), status, created_at, updated_at
FROM proxy_subscription_sources
WHERE id = $1 AND deleted_at IS NULL AND status = 'active'`, id)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	if !rows.Next() {
		return nil, ErrProxyNotFound
	}
	var item ProxySubscriptionSource
	var strategyRaw, scanResultRaw string
	if err := rows.Scan(&item.ID, &item.Name, &item.URL, &item.SourceType, &item.Provider, &item.SyncEnabled, &item.SyncIntervalMinutes, &strategyRaw, &item.SidecarEnabled, &item.Runtime, &item.PortStart, &item.PortEnd, &item.ScanEnabled, &item.ScanIntervalMinutes, &item.HealthCheckIntervalMinutes, &item.ReputationProvider, &item.ReputationAPIKeyRef, &item.LastSyncedAt, &item.LastScanAt, &scanResultRaw, &item.LastError, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, err
	}
	item.Strategy = parseProxySubscriptionStrategy(strategyRaw)
	item.LastScanResult = parseJSONMap(scanResultRaw)
	return &item, rows.Err()
}
func (s *adminServiceImpl) tryStartProxySubscriptionScan(sourceID int64) error {
	s.scanStateMu.Lock()
	defer s.scanStateMu.Unlock()
	if s.scanActive {
		if s.scanActiveSourceID == sourceID {
			return infraerrors.Conflict("PROXY_SUBSCRIPTION_SCAN_BUSY", "proxy subscription scan is already running for this source")
		}
		return infraerrors.Conflict("PROXY_SUBSCRIPTION_SCAN_BUSY", "another proxy subscription scan is already running")
	}
	s.scanActive = true
	s.scanActiveSourceID = sourceID
	s.scanStartedAt = time.Now()
	return nil
}
func (s *adminServiceImpl) finishProxySubscriptionScan() {
	s.scanStateMu.Lock()
	defer s.scanStateMu.Unlock()
	s.scanActive = false
	s.scanActiveSourceID = 0
	s.scanStartedAt = time.Time{}
}
func (s *adminServiceImpl) loadProxySubscriptionNodeState(ctx context.Context, sourceID int64) (map[string]ProxySubscriptionNode, error) {
	nodes, err := s.ListProxySubscriptionNodes(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]ProxySubscriptionNode, len(nodes))
	for i := range nodes {
		out[nodes[i].NodeKey] = nodes[i]
	}
	return out, nil
}
func (s *adminServiceImpl) evaluateProxySubscriptionItems(ctx context.Context, source *ProxySubscriptionSource, items []ProxyImportPreviewItem, strategy ProxySubscriptionStrategy, existing map[string]ProxySubscriptionNode) map[string]proxySubscriptionNodeEvaluation {
	evaluations := make(map[string]proxySubscriptionNodeEvaluation, len(items))
	batchPause := proxySubscriptionBatchPause(strategy, len(items))
	for idx, item := range items {
		if idx > 0 && strategy.ScanBatchSize > 0 && idx%strategy.ScanBatchSize == 0 && batchPause > 0 {
			timer := time.NewTimer(batchPause)
			select {
			case <-ctx.Done():
				timer.Stop()
				return evaluations
			case <-timer.C:
			}
		}
		key := item.Key
		if key == "" {
			key = proxyImportItemKey(item)
		}
		eval := proxySubscriptionNodeEvaluation{Key: key, Country: inferProxySubscriptionCountry(item)}
		if previous, ok := existing[key]; ok {
			eval.FailureCount = previous.FailureCount
			eval.TimeoutCount = previous.TimeoutCount
			if previous.SleepUntil != nil && previous.SleepUntil.After(time.Now()) {
				eval.SleepUntil = previous.SleepUntil
			}
		}
		if latencyMs, timedOut, probeErr := s.measureProxySubscriptionNodeLatency(ctx, item, strategy); probeErr != nil {
			eval.TimedOut = timedOut
			eval.FailureCount++
			if timedOut {
				eval.TimeoutCount++
			}
			eval.LastError = probeErr.Error()
		} else {
			eval.LatencyMs = latencyMs
			eval.FailureCount = 0
			eval.TimeoutCount = 0
			eval.SleepUntil = nil
		}
		if strategy.TimeoutSleepAfter > 0 && eval.TimeoutCount >= strategy.TimeoutSleepAfter {
			sleepUntil := time.Now().Add(time.Duration(strategy.SleepMinutes) * time.Minute)
			eval.SleepUntil = &sleepUntil
			eval.LastError = fmt.Sprintf("node timed out %d times, sleeping until %s", eval.TimeoutCount, sleepUntil.Format(time.RFC3339))
		}
		if source != nil && source.ReputationProvider != "" && source.ReputationProvider != "none" {
			reputation, err := s.lookupProxySubscriptionNodeReputation(ctx, source, item.Host, strategy.ReputationCacheHours)
			if err != nil {
				if eval.LastError == "" {
					eval.LastError = err.Error()
				}
			} else if reputation != nil {
				eval.ExitIP = reputation.IP
				eval.ExitCountry = reputation.Country
				eval.ExitCountryCode = reputation.CountryCode
				eval.ReputationProvider = reputation.Provider
				eval.ReputationCheckedAt = &reputation.CheckedAt
				eval.ReputationRaw = reputation.Raw
				eval.IPCleanScore = &reputation.CleanScore
				if eval.Country == "" {
					eval.Country = defaultString(reputation.CountryCode, reputation.Country)
				}
			}
		}
		eval.Score = scoreProxySubscriptionItem(item, strategy, eval.IPCleanScore, eval.LatencyMs)
		evaluations[key] = eval
	}
	return evaluations
}
func selectProxySubscriptionItems(items []ProxyImportPreviewItem, source *ProxySubscriptionSource, strategy ProxySubscriptionStrategy, evaluations map[string]proxySubscriptionNodeEvaluation) map[string]string {
	strategy = normalizeProxySubscriptionStrategy(strategy)
	type candidate struct {
		key     string
		item    ProxyImportPreviewItem
		score   int
		country string
	}
	candidates := make([]candidate, 0, len(items))
	for _, item := range items {
		if !item.Valid || item.Duplicate || (item.SidecarRequired && !isSupportedSubscriptionSidecarProtocol(item.Protocol)) {
			continue
		}
		key := item.Key
		if key == "" {
			key = proxyImportItemKey(item)
		}
		eval, ok := evaluations[key]
		if !ok {
			eval = proxySubscriptionNodeEvaluation{Key: key, Country: inferProxySubscriptionCountry(item), Score: scoreProxySubscriptionItem(item, strategy, nil, nil)}
		}
		if source != nil && item.SidecarRequired && !source.SidecarEnabled {
			continue
		}
		if eval.SleepUntil != nil && eval.SleepUntil.After(time.Now()) {
			continue
		}
		if strategy.MinIPCleanScore > 0 && (eval.IPCleanScore == nil || *eval.IPCleanScore < strategy.MinIPCleanScore) {
			continue
		}
		if strategy.MinQualityScore > 0 && eval.Score < strategy.MinQualityScore {
			continue
		}
		if strategy.MaxLatencyMs > 0 && eval.LatencyMs != nil && *eval.LatencyMs > strategy.MaxLatencyMs {
			continue
		}
		candidates = append(candidates, candidate{key: key, item: item, score: eval.Score, country: defaultString(eval.Country, inferProxySubscriptionCountry(item))})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].key < candidates[j].key
		}
		return candidates[i].score > candidates[j].score
	})
	statuses := map[string]string{}
	perCountry := map[string]int{}
	countryCount := 0
	selectedCount := 0
	normalizeCountry := func(country string) string {
		if country == "" {
			country = "unknown"
		}
		return country
	}
	canSelect := func(country string) bool {
		if isCountryBlocked(country, strategy.BlockedCountries) {
			return false
		}
		if strategy.MaxNodesPerCountry > 0 && perCountry[country] >= strategy.MaxNodesPerCountry {
			return false
		}
		if perCountry[country] == 0 && strategy.MaxCountryCount > 0 && countryCount >= strategy.MaxCountryCount {
			return false
		}
		return true
	}
	selectCandidate := func(c candidate) bool {
		if selectedCount >= strategy.MaxEnabledNodes {
			return false
		}
		country := normalizeCountry(c.country)
		if !canSelect(country) {
			return false
		}
		statuses[c.key] = "selected"
		if perCountry[country] == 0 {
			countryCount++
		}
		perCountry[country]++
		selectedCount++
		return true
	}
	if strategy.MinCountryCount > 1 {
		for _, c := range candidates {
			country := normalizeCountry(c.country)
			if perCountry[country] > 0 {
				continue
			}
			if selectedCount >= strategy.MinCountryCount {
				break
			}
			selectCandidate(c)
		}
	}
	for _, c := range candidates {
		if selectedCount >= strategy.MaxEnabledNodes {
			break
		}
		if statuses[c.key] == "selected" {
			continue
		}
		selectCandidate(c)
	}
	standbyCount := 0
	appendStandby := func(c candidate) {
		if standbyCount >= strategy.StandbyNodes {
			return
		}
		if statuses[c.key] != "" {
			return
		}
		statuses[c.key] = "standby"
		standbyCount++
	}
	if strategy.ReplaceSameCountryFirst {
		for _, c := range candidates {
			country := normalizeCountry(c.country)
			if perCountry[country] == 0 {
				continue
			}
			appendStandby(c)
			if standbyCount >= strategy.StandbyNodes {
				break
			}
		}
	}
	for _, c := range candidates {
		if standbyCount >= strategy.StandbyNodes {
			break
		}
		appendStandby(c)
	}
	return statuses
}

func isSupportedSubscriptionSidecarProtocol(protocol string) bool {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "anytls", "vless", "hysteria2":
		return true
	default:
		return false
	}
}

func scoreProxySubscriptionItem(item ProxyImportPreviewItem, strategy ProxySubscriptionStrategy, cleanScore *int, latencyMs *int) int {
	score := 60
	if !item.SidecarRequired {
		score += 10
	}
	country := inferProxySubscriptionCountry(item)
	if isCountryPreferred(country, strategy.PreferredCountries) {
		score += 15
	}
	if strings.Contains(strings.ToLower(item.Name), "direct") || strings.Contains(item.Name, "直连") {
		score += 5
	}
	if item.Protocol == "anytls" {
		score += 3
	}
	if cleanScore != nil {
		score += (*cleanScore - 50) / 5
	}
	if latencyMs != nil {
		switch {
		case *latencyMs <= 300:
			score += 12
		case *latencyMs <= 800:
			score += 6
		case *latencyMs <= 1500:
			score += 1
		default:
			score -= minInt((*latencyMs-1500)/120, 25)
		}
	}
	if score > 100 {
		return 100
	}
	if score < 0 {
		return 0
	}
	return score
}
func inferProxySubscriptionCountry(item ProxyImportPreviewItem) string {
	text := " " + strings.ToLower(strings.Join([]string{item.Name, item.Region, item.Host}, " ")) + " "
	switch {
	case strings.Contains(text, "美国") || strings.Contains(text, "united states") || strings.Contains(text, " usa ") || strings.Contains(text, " us "):
		return "US"
	case strings.Contains(text, "日本") || strings.Contains(text, " japan ") || strings.Contains(text, " jp "):
		return "JP"
	case strings.Contains(text, "新加坡") || strings.Contains(text, " singapore ") || strings.Contains(text, " sg "):
		return "SG"
	case strings.Contains(text, "香港") || strings.Contains(text, " hong kong ") || strings.Contains(text, " hk "):
		return "HK"
	case strings.Contains(text, "台湾") || strings.Contains(text, " taiwan ") || strings.Contains(text, " tw "):
		return "TW"
	default:
		return ""
	}
}
func isCountryPreferred(country string, preferred []string) bool {
	country = strings.ToUpper(strings.TrimSpace(country))
	for _, item := range preferred {
		if country == strings.ToUpper(strings.TrimSpace(item)) {
			return true
		}
	}
	return false
}
func isCountryBlocked(country string, blocked []string) bool {
	country = strings.ToUpper(strings.TrimSpace(country))
	for _, item := range blocked {
		if country == strings.ToUpper(strings.TrimSpace(item)) {
			return true
		}
	}
	return false
}
func (s *adminServiceImpl) upsertProxySubscriptionNode(ctx context.Context, sourceID int64, item ProxyImportPreviewItem, key string, evaluation proxySubscriptionNodeEvaluation, status string, selected bool) (int64, error) {
	raw := strings.TrimSpace(item.Raw)
	if raw == "" {
		raw = strings.TrimSpace(item.Name)
	}
	countryHint := defaultString(evaluation.Country, inferProxySubscriptionCountry(item))
	reputationRaw, _ := json.Marshal(evaluation.ReputationRaw)
	rows, err := s.entClient.QueryContext(ctx, `
INSERT INTO proxy_subscription_nodes (
  source_id, node_key, raw_uri, name, protocol, server, port, username,
  country_hint, exit_ip, exit_country, exit_country_code, ip_clean_score, reputation_provider,
  reputation_checked_at, reputation_raw, latency_ms, score, status, failure_count, timeout_count,
  sleep_until, selected, sidecar_required, last_error, last_scanned_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''), NULLIF($11, ''), NULLIF($12, ''),
        $13, NULLIF($14, ''), $15, $16::jsonb, $17, $18, $19, $20, $21, $22, $23, $24, $25, NOW(), NOW())
ON CONFLICT (source_id, node_key) WHERE deleted_at IS NULL
DO UPDATE SET raw_uri = EXCLUDED.raw_uri, name = EXCLUDED.name, protocol = EXCLUDED.protocol,
              server = EXCLUDED.server, port = EXCLUDED.port, username = EXCLUDED.username,
              country_hint = EXCLUDED.country_hint, exit_ip = EXCLUDED.exit_ip, exit_country = EXCLUDED.exit_country,
              exit_country_code = EXCLUDED.exit_country_code, ip_clean_score = EXCLUDED.ip_clean_score,
              reputation_provider = EXCLUDED.reputation_provider, reputation_checked_at = EXCLUDED.reputation_checked_at,
              reputation_raw = EXCLUDED.reputation_raw, latency_ms = EXCLUDED.latency_ms, score = EXCLUDED.score,
              status = EXCLUDED.status, failure_count = EXCLUDED.failure_count, timeout_count = EXCLUDED.timeout_count,
              sleep_until = EXCLUDED.sleep_until, selected = EXCLUDED.selected, sidecar_required = EXCLUDED.sidecar_required,
              last_scanned_at = NOW(), last_error = EXCLUDED.last_error, updated_at = NOW()
RETURNING id`, sourceID, key, raw, item.Name, item.Protocol, item.Host, item.Port, item.Username, countryHint, evaluation.ExitIP, evaluation.ExitCountry, evaluation.ExitCountryCode, evaluation.IPCleanScore, nullIfBlank(evaluation.ReputationProvider), evaluation.ReputationCheckedAt, string(reputationRaw), evaluation.LatencyMs, evaluation.Score, status, evaluation.FailureCount, evaluation.TimeoutCount, evaluation.SleepUntil, selected, item.SidecarRequired, nullIfBlank(evaluation.LastError))
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = rows.Close()
	}()
	if rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		return id, nil
	}
	return 0, rows.Err()
}
func (s *adminServiceImpl) markMissingProxySubscriptionNodes(ctx context.Context, sourceID int64, activeKeys map[string]struct{}) error {
	nodes, err := s.ListProxySubscriptionNodes(ctx, sourceID)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if _, ok := activeKeys[node.NodeKey]; ok {
			continue
		}
		if err := s.retireProxySubscriptionNodeResources(ctx, node, "subscription node missing from latest scan"); err != nil {
			return err
		}
		if _, execErr := s.entClient.ExecContext(ctx, `
UPDATE proxy_subscription_nodes
SET status = 'missing', selected = FALSE, last_error = $2, updated_at = NOW()
WHERE id = $1`, node.ID, "subscription node missing from latest scan"); execErr != nil {
			return execErr
		}
	}
	return nil
}
func proxySubscriptionBatchPause(strategy ProxySubscriptionStrategy, totalItems int) time.Duration {
	if !strategy.ResourceAdaptiveScan {
		return 0
	}
	base := 3 * time.Second
	switch {
	case strategy.PauseFreeMemoryMB >= 768:
		base = 12 * time.Second
	case strategy.MinFreeMemoryMB >= 1024:
		base = 6 * time.Second
	}
	if totalItems <= 0 || strategy.ScanBatchSize <= 0 || strategy.ScanBudgetMinutes <= 0 {
		return base
	}
	batchCount := (totalItems + strategy.ScanBatchSize - 1) / strategy.ScanBatchSize
	if batchCount <= 1 {
		return base
	}
	targetPause := time.Duration(strategy.ScanBudgetMinutes) * time.Minute / time.Duration(batchCount) / 3
	if targetPause < base {
		return base
	}
	if targetPause > 20*time.Second {
		return 20 * time.Second
	}
	return targetPause
}
func (s *adminServiceImpl) measureProxySubscriptionNodeLatency(ctx context.Context, item ProxyImportPreviewItem, strategy ProxySubscriptionStrategy) (*int, bool, error) {
	host := strings.TrimSpace(item.Host)
	if host == "" || item.Port <= 0 {
		return nil, false, nil
	}
	switch strings.ToLower(strings.TrimSpace(item.Protocol)) {
	case "tuic", "hysteria2", "wireguard":
		return nil, false, nil
	}
	timeout := 5 * time.Second
	if strategy.MaxLatencyMs > 0 {
		timeout = time.Duration(strategy.MaxLatencyMs) * time.Millisecond
		if timeout < 800*time.Millisecond {
			timeout = 800 * time.Millisecond
		}
		if timeout > 5*time.Second {
			timeout = 5 * time.Second
		}
	}
	dialer := net.Dialer{Timeout: timeout}
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(item.Port)))
	if err != nil {
		timedOut := errors.Is(err, context.DeadlineExceeded)
		if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
			timedOut = true
		}
		return nil, timedOut, fmt.Errorf("latency probe failed for %s:%d: %w", host, item.Port, err)
	}
	latency := int(time.Since(start).Milliseconds())
	_ = conn.Close()
	return &latency, false, nil
}
func (s *adminServiceImpl) reserveProxySidecarEndpoint(ctx context.Context, source *ProxySubscriptionSource, nodeID int64, port int) error {
	if source == nil {
		return nil
	}
	if _, err := s.entClient.ExecContext(ctx, `
UPDATE proxies p
SET status = $2, quality_status = 'failed', last_checked_at = NOW(), updated_at = NOW()
WHERE p.id = (
  SELECT proxy_id FROM proxy_sidecar_endpoints
  WHERE node_id = $1 AND deleted_at IS NULL LIMIT 1
)
  AND p.port <> $3 AND p.deleted_at IS NULL`, nodeID, StatusDisabled, port); err != nil {
		return err
	}
	_, err := s.entClient.ExecContext(ctx, `
INSERT INTO proxy_sidecar_endpoints (source_id, node_id, runtime, listen_host, listen_port, protocol, status, updated_at)
VALUES ($1, $2, $3, $4, $5, 'socks5', 'pending', NOW())
ON CONFLICT (node_id) WHERE deleted_at IS NULL
DO UPDATE SET runtime = EXCLUDED.runtime, listen_host = EXCLUDED.listen_host, listen_port = EXCLUDED.listen_port,
			status = 'pending', updated_at = NOW()`, source.ID, nodeID, defaultString(source.Runtime, "sing-box"), sidecarListenHost(), port)
	return err
}

func (s *adminServiceImpl) allocateProxySidecarPort(ctx context.Context, source *ProxySubscriptionSource, nodeID int64) (int, error) {
	if source == nil || source.PortStart <= 0 || source.PortEnd < source.PortStart {
		return 0, errors.New("invalid sidecar port range")
	}
	if _, err := s.entClient.ExecContext(ctx, `SELECT pg_advisory_xact_lock(728341902)`); err != nil {
		return 0, fmt.Errorf("lock sidecar port allocator: %w", err)
	}
	var existingPort sql.NullInt64
	rows, err := s.entClient.QueryContext(ctx, `
SELECT listen_port
FROM proxy_sidecar_endpoints
WHERE node_id = $1 AND deleted_at IS NULL

LIMIT 1`, nodeID)
	if err != nil {
		return 0, fmt.Errorf("load existing sidecar port: %w", err)
	}
	if rows.Next() {
		if err := rows.Scan(&existingPort); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("scan existing sidecar port: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, fmt.Errorf("read existing sidecar port: %w", err)
	}
	_ = rows.Close()
	if existingPort.Valid && existingPort.Int64 >= int64(source.PortStart) && existingPort.Int64 <= int64(source.PortEnd) {
		var occupied bool
		rows, err := s.entClient.QueryContext(ctx, `
SELECT EXISTS (
  SELECT 1 FROM proxy_sidecar_endpoints
  WHERE listen_port = $1 AND deleted_at IS NULL AND node_id <> $2

)`, existingPort.Int64, nodeID)
		if err != nil {
			return 0, fmt.Errorf("check existing sidecar port: %w", err)
		}
		if rows.Next() {
			if err := rows.Scan(&occupied); err != nil {
				_ = rows.Close()
				return 0, fmt.Errorf("scan existing sidecar port occupancy: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("read existing sidecar port occupancy: %w", err)
		}
		_ = rows.Close()
		if !occupied {
			return int(existingPort.Int64), nil
		}
	}
	var port int
	rows, err = s.entClient.QueryContext(ctx, allocateProxySidecarPortSQL, source.PortStart, source.PortEnd)
	if err != nil {
		return 0, fmt.Errorf("allocate sidecar port: %w", err)
	}
	if !rows.Next() {
		_ = rows.Close()
		return 0, fmt.Errorf("no available sidecar port in range %d-%d", source.PortStart, source.PortEnd)
	}
	if err := rows.Scan(&port); err != nil {
		_ = rows.Close()
		return 0, fmt.Errorf("scan allocated sidecar port: %w", err)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, fmt.Errorf("read allocated sidecar port: %w", err)
	}
	_ = rows.Close()
	return port, nil
}
func (s *adminServiceImpl) refreshProxySidecarEndpointReadiness(ctx context.Context, nodeID, proxyID int64, port int) error {
	endpointStatus := "pending"
	proxyStatus := StatusDisabled
	host := sidecarProbeHost()
	lastError := fmt.Sprintf("sidecar endpoint %s:%d is not ready", host, port)
	lastStartedAt := any(nil)
	if s.proxyProber != nil {
		probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		exitInfo, _, probeErr := s.proxyProber.ProbeProxy(probeCtx, (&Proxy{Protocol: "socks5", Host: host, Port: port}).URL())
		cancel()
		if probeErr == nil && exitInfo != nil && strings.TrimSpace(exitInfo.IP) != "" {
			endpointStatus = "ready"
			proxyStatus = StatusActive
			lastError = ""
			lastStartedAt = time.Now()
		} else if probeErr != nil {
			lastError = fmt.Sprintf("sidecar proxy probe failed: %v", probeErr)
		} else {
			lastError = "sidecar proxy probe returned no exit IP"
		}
	}
	if _, err := s.entClient.ExecContext(ctx, `
UPDATE proxy_sidecar_endpoints
SET proxy_id = $2,
    status = $3,
    last_checked_at = NOW(),
    last_started_at = COALESCE($4, last_started_at),
    last_error = NULLIF($5, ''),
    updated_at = NOW()
WHERE node_id = $1 AND deleted_at IS NULL`, nodeID, proxyID, endpointStatus, lastStartedAt, lastError); err != nil {
		return err
	}
	_, err := s.entClient.ExecContext(ctx, `
UPDATE proxies
SET status = $2,
    last_checked_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL`, proxyID, proxyStatus)
	return err
}
func isLocalTCPPortReachable(ctx context.Context, host string, port int) bool {
	if strings.TrimSpace(host) == "" || port <= 0 {
		return false
	}
	timeout := 800 * time.Millisecond
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining > 0 && remaining < timeout {
			timeout = remaining
		}
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
func sidecarListenHost() string {
	if host := strings.TrimSpace(os.Getenv("SUB2API_SIDECAR_LISTEN_HOST")); host != "" {
		return host
	}
	return "0.0.0.0"
}
func sidecarProxyHost() string {
	if host := strings.TrimSpace(os.Getenv("SUB2API_SIDECAR_PROXY_HOST")); host != "" {
		return host
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("SUB2API_SIDECAR_USE_LOCALHOST")), "true") {
		return "127.0.0.1"
	}
	return "sing-box"
}
func sidecarProbeHost() string {
	if host := strings.TrimSpace(os.Getenv("SUB2API_SIDECAR_PROBE_HOST")); host != "" {
		return host
	}
	return sidecarProxyHost()
}
func (s *adminServiceImpl) retireProxySubscriptionSourceResources(ctx context.Context, sourceID int64, reason string) error {
	nodes, err := s.ListProxySubscriptionNodes(ctx, sourceID)
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if err := s.retireProxySubscriptionNodeResources(ctx, node, reason); err != nil {
			return err
		}
	}
	if _, err := s.entClient.ExecContext(ctx, `
UPDATE proxy_sidecar_endpoints
SET status = 'inactive',
    last_error = NULLIF($2, ''),
    deleted_at = NOW(),
    updated_at = NOW()
WHERE source_id = $1 AND deleted_at IS NULL`, sourceID, reason); err != nil {
		return err
	}
	_, err = s.entClient.ExecContext(ctx, `
UPDATE proxy_subscription_nodes
SET status = 'inactive',
    selected = FALSE,
    last_error = NULLIF($2, ''),
    deleted_at = NOW(),
    updated_at = NOW()
WHERE source_id = $1 AND deleted_at IS NULL`, sourceID, reason)
	return err
}
func (s *adminServiceImpl) retireProxySubscriptionNodeResources(ctx context.Context, node ProxySubscriptionNode, reason string) error {
	if node.SidecarRequired {
		return s.retireSidecarProxyForSubscriptionNode(ctx, node.ID, reason)
	}
	item := parseProxyLine(node.RawURI, "")
	if !item.Valid {
		return nil
	}
	return s.retireDirectProxyByAddress(ctx, item.Host, item.Port, item.Username, item.Password, reason)
}
func (s *adminServiceImpl) retireSidecarProxyForSubscriptionNode(ctx context.Context, nodeID int64, reason string) error {
	var proxyID sql.NullInt64
	rows, err := s.entClient.QueryContext(ctx, `
SELECT proxy_id
FROM proxy_sidecar_endpoints
WHERE node_id = $1 AND deleted_at IS NULL
LIMIT 1`, nodeID)
	if err != nil {
		return err
	}
	defer func() {
		_ = rows.Close()
	}()
	if rows.Next() {
		if err := rows.Scan(&proxyID); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if _, err := s.entClient.ExecContext(ctx, `
UPDATE proxy_sidecar_endpoints
SET status = 'inactive',
    last_checked_at = NOW(),
    last_error = NULLIF($2, ''),
    updated_at = NOW()
WHERE node_id = $1 AND deleted_at IS NULL`, nodeID, reason); err != nil {
		return err
	}
	if proxyID.Valid && proxyID.Int64 > 0 {
		return s.retireProxyByID(ctx, proxyID.Int64, reason)
	}
	return nil
}
func (s *adminServiceImpl) retireDirectProxyByAddress(ctx context.Context, host string, port int, username, password, reason string) error {
	proxy, exists, err := s.findProxyByAddress(ctx, host, port, username, password)
	if err != nil || !exists || proxy == nil {
		return err
	}
	return s.retireProxyByID(ctx, proxy.ID, reason)
}
func (s *adminServiceImpl) retireProxyByID(ctx context.Context, proxyID int64, reason string) error {
	if proxyID <= 0 {
		return nil
	}
	if _, err := s.entClient.ExecContext(ctx, `
UPDATE proxies
SET status = $2,
    quality_status = 'failed',
    last_checked_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL`, proxyID, StatusDisabled); err != nil {
		return err
	}
	_, err := s.entClient.ExecContext(ctx, `
UPDATE account_proxy_bindings
SET status = 'proxy_unavailable',
    last_failure_at = NOW(),
    last_failure_reason = NULLIF($2, ''),
    updated_at = NOW()
WHERE proxy_id = $1
  AND status = 'active'`, proxyID, reason)
	return err
}
func (s *adminServiceImpl) saveProxySubscriptionScanResult(ctx context.Context, id int64, result *ProxySubscriptionScanResult) error {
	if result == nil {
		return nil
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = s.entClient.ExecContext(ctx, `
UPDATE proxy_subscription_sources
SET last_scan_at = NOW(), last_scan_result = $2::jsonb, last_error = NULL, updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL`, id, string(raw))
	return err
}
func (s *adminServiceImpl) lookupProxySubscriptionNodeReputation(ctx context.Context, source *ProxySubscriptionSource, host string, cacheHours int) (*proxyIPReputationResult, error) {
	if s == nil || s.entClient == nil || source == nil {
		return nil, nil
	}
	provider := strings.ToLower(strings.TrimSpace(source.ReputationProvider))
	if provider == "" || provider == "none" {
		return nil, nil
	}
	ipAddress, err := resolveProxySubscriptionHostIP(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve node host %q failed: %w", host, err)
	}
	if ipAddress == "" {
		return nil, fmt.Errorf("no IP resolved for host %q", host)
	}
	cached, err := s.getCachedProxyIPReputation(ctx, ipAddress, provider)
	if err == nil && cached != nil {
		return cached, nil
	}
	apiKey, err := s.resolveProxySubscriptionAPIKey(ctx, source.ReputationAPIKeyRef)
	if err != nil {
		return nil, err
	}
	var result *proxyIPReputationResult
	switch provider {
	case "abuseipdb":
		result, err = fetchAbuseIPDBReputation(ctx, apiKey, ipAddress)
	default:
		return nil, fmt.Errorf("unsupported reputation provider: %s", provider)
	}
	if err != nil {
		return nil, err
	}
	if result != nil {
		if cacheHours <= 0 {
			cacheHours = 24
		}
		result.Provider = provider
		_ = s.saveCachedProxyIPReputation(ctx, result, cacheHours)
	}
	return result, nil
}
func resolveProxySubscriptionHostIP(ctx context.Context, host string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", errors.New("empty host")
	}
	if parsed := net.ParseIP(host); parsed != nil {
		return parsed.String(), nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ipv4 := addr.IP.To4(); ipv4 != nil {
			return ipv4.String(), nil
		}
	}
	if len(addrs) > 0 {
		return addrs[0].IP.String(), nil
	}
	return "", errors.New("host resolved to no IPs")
}
func (s *adminServiceImpl) resolveProxySubscriptionAPIKey(ctx context.Context, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if isDefaultAbuseIPDBAPIKeyRef(ref) {
		ref = ""
	}
	if ref == "" {
		if s != nil && s.settingService != nil && s.settingService.settingRepo != nil {
			if value, err := s.settingService.settingRepo.GetValue(ctx, SettingKeyAbuseIPDBAPIKey); err == nil {
				if value = strings.TrimSpace(value); value != "" {
					return value, nil
				}
			}
		}
		if value := strings.TrimSpace(os.Getenv("ABUSEIPDB_API_KEY")); value != "" {
			return value, nil
		}
		ref = "keymd:AbuseIPDB API Key"
	}
	return resolveProxySubscriptionAPIKeyRef(ref)
}
func isDefaultAbuseIPDBAPIKeyRef(ref string) bool {
	ref = strings.TrimSpace(ref)
	return strings.EqualFold(ref, "keymd:AbuseIPDB API Key")
}
func resolveProxySubscriptionAPIKeyRef(ref string) (string, error) {
	switch {
	case strings.HasPrefix(strings.ToLower(ref), "env:"):
		key := strings.TrimSpace(ref[4:])
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			return "", fmt.Errorf("environment variable %s is empty", key)
		}
		return value, nil
	case strings.HasPrefix(strings.ToLower(ref), "literal:"):
		value := strings.TrimSpace(ref[len("literal:"):])
		if value == "" {
			return "", errors.New("literal API key is empty")
		}
		return value, nil
	case strings.HasPrefix(strings.ToLower(ref), "keymd:"):
		label := strings.TrimSpace(ref[len("keymd:"):])
		return readAPIKeyFromMarkdown(label)
	default:
		return strings.TrimSpace(ref), nil
	}
}
func readAPIKeyFromMarkdown(label string) (string, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return "", errors.New("empty key label")
	}
	pattern := regexp.MustCompile(`(?im)^\s*` + regexp.QuoteMeta(label) + `\s*:\s*(.+?)\s*$`)
	for _, candidate := range findProxySubscriptionKeyMarkdownCandidates() {
		content, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		matches := pattern.FindSubmatch(content)
		if len(matches) < 2 {
			continue
		}
		value := strings.TrimSpace(string(matches[1]))
		if value != "" {
			return value, nil
		}
	}
	return "", fmt.Errorf("unable to resolve API key label %q from key.md", label)
}
func findProxySubscriptionKeyMarkdownCandidates() []string {
	wd, err := os.Getwd()
	if err != nil {
		return []string{"key.md", filepath.Join("sub2api", "key.md")}
	}
	seen := map[string]struct{}{}
	paths := make([]string, 0, 12)
	current := wd
	for i := 0; i < 5; i++ {
		for _, candidate := range []string{filepath.Join(current, "key.md"), filepath.Join(current, "sub2api", "key.md"), filepath.Join(current, "..", "sub2api", "key.md")} {
			candidate = filepath.Clean(candidate)
			if _, ok := seen[candidate]; ok {
				continue
			}
			seen[candidate] = struct{}{}
			paths = append(paths, candidate)
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return paths
}
func fetchAbuseIPDBReputation(ctx context.Context, apiKey, ipAddress string) (*proxyIPReputationResult, error) {
	client, err := httpclient.GetClient(httpclient.Options{Timeout: 15 * time.Second, ResponseHeaderTimeout: 15 * time.Second})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.abuseipdb.com/api/v2/check", nil)
	if err != nil {
		return nil, err
	}
	query := req.URL.Query()
	query.Set("ipAddress", ipAddress)
	query.Set("maxAgeInDays", "90")
	query.Set("verbose", "")
	req.URL.RawQuery = query.Encode()
	req.Header.Set("Key", apiKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", proxyQualityClientUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("AbuseIPDB returned HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Data struct {
			IPAddress            string `json:"ipAddress"`
			AbuseConfidenceScore int    `json:"abuseConfidenceScore"`
			CountryCode          string `json:"countryCode"`
			CountryName          string `json:"countryName"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	raw := map[string]any{}
	_ = json.Unmarshal(body, &raw)
	cleanScore := 100 - payload.Data.AbuseConfidenceScore
	if cleanScore < 0 {
		cleanScore = 0
	}
	if cleanScore > 100 {
		cleanScore = 100
	}
	return &proxyIPReputationResult{IP: defaultString(payload.Data.IPAddress, ipAddress), CleanScore: cleanScore, Country: payload.Data.CountryName, CountryCode: payload.Data.CountryCode, Provider: "abuseipdb", CheckedAt: time.Now(), Raw: raw}, nil
}
func (s *adminServiceImpl) getCachedProxyIPReputation(ctx context.Context, ipAddress, provider string) (*proxyIPReputationResult, error) {
	rows, err := s.entClient.QueryContext(ctx, `
SELECT clean_score, raw::text, checked_at
FROM proxy_ip_reputation_cache
WHERE ip = $1 AND provider = $2 AND expires_at > NOW()`, ipAddress, provider)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var result proxyIPReputationResult
	var rawText string
	if err := rows.Scan(&result.CleanScore, &rawText, &result.CheckedAt); err != nil {
		return nil, err
	}
	result.IP = ipAddress
	result.Provider = provider
	result.Raw = parseJSONMap(rawText)
	if data, ok := result.Raw["data"].(map[string]any); ok {
		if country, ok := data["countryName"].(string); ok {
			result.Country = country
		}
		if code, ok := data["countryCode"].(string); ok {
			result.CountryCode = code
		}
	}
	return &result, nil
}
func (s *adminServiceImpl) saveCachedProxyIPReputation(ctx context.Context, result *proxyIPReputationResult, cacheHours int) error {
	if s == nil || s.entClient == nil || result == nil {
		return nil
	}
	if cacheHours <= 0 {
		cacheHours = 24
	}
	raw, err := json.Marshal(result.Raw)
	if err != nil {
		return err
	}
	_, err = s.entClient.ExecContext(ctx, `
INSERT INTO proxy_ip_reputation_cache (ip, provider, clean_score, raw, checked_at, expires_at)
VALUES ($1, $2, $3, $4::jsonb, $5, $6)
ON CONFLICT (ip, provider)
DO UPDATE SET clean_score = EXCLUDED.clean_score, raw = EXCLUDED.raw, checked_at = EXCLUDED.checked_at, expires_at = EXCLUDED.expires_at`, result.IP, result.Provider, result.CleanScore, string(raw), result.CheckedAt, result.CheckedAt.Add(time.Duration(cacheHours)*time.Hour))
	return err
}
func (s *adminServiceImpl) upsertDirectProxyFromSubscriptionNode(ctx context.Context, source *ProxySubscriptionSource, item ProxyImportPreviewItem, evaluation proxySubscriptionNodeEvaluation) error {
	proxy, exists, err := s.findProxyByAddress(ctx, item.Host, item.Port, item.Username, item.Password)
	if err != nil {
		return err
	}
	qualityStatus := ProxyQualityHealthy
	if (evaluation.IPCleanScore != nil && *evaluation.IPCleanScore < 50) || evaluation.LastError != "" || (evaluation.LatencyMs != nil && *evaluation.LatencyMs > 1500) {
		qualityStatus = ProxyQualityDegraded
	}
	name := strings.TrimSpace(item.Name)
	if name == "" {
		name = fmt.Sprintf("%s:%d", item.Host, item.Port)
	}
	if !exists {
		_, err := s.CreateProxy(ctx, &CreateProxyInput{Name: name, Protocol: item.Protocol, Host: item.Host, Port: item.Port, Username: item.Username, Password: item.Password, Source: "subscription", ProxyType: defaultString(item.ProxyType, "datacenter"), Provider: source.Provider, Region: defaultString(evaluation.ExitCountryCode, evaluation.Country), ExitIP: evaluation.ExitIP, QualityStatus: qualityStatus, Weight: maxInt(1, evaluation.Score)})
		return err
	}
	proxy.Name = name
	proxy.Protocol = item.Protocol
	proxy.Host = item.Host
	proxy.Port = item.Port
	proxy.Username = item.Username
	proxy.Password = item.Password
	proxy.Status = StatusActive
	applyProxyUpdateMetadata(proxy, &UpdateProxyInput{Source: "subscription", ProxyType: defaultString(item.ProxyType, "datacenter"), Provider: source.Provider, Region: defaultString(evaluation.ExitCountryCode, evaluation.Country), ExitIP: evaluation.ExitIP, QualityStatus: qualityStatus, Weight: intPtr(maxInt(1, evaluation.Score))})
	_, err = s.UpdateProxy(ctx, proxy.ID, &UpdateProxyInput{Name: proxy.Name, Protocol: proxy.Protocol, Host: proxy.Host, Port: proxy.Port, Username: proxy.Username, Password: proxy.Password, Status: StatusActive, Source: proxy.Source, ProxyType: proxy.ProxyType, Provider: proxy.Provider, Region: proxy.Region, ExitIP: proxy.ExitIP, QualityStatus: proxy.QualityStatus, Weight: intPtr(proxy.Weight)})
	return err
}
func (s *adminServiceImpl) upsertSidecarProxyForSubscriptionNode(ctx context.Context, source *ProxySubscriptionSource, nodeID int64, item ProxyImportPreviewItem, evaluation proxySubscriptionNodeEvaluation, port int) error {
	proxyName := strings.TrimSpace(item.Name)
	if proxyName == "" {
		proxyName = fmt.Sprintf("%s:%d", item.Host, item.Port)
	}
	proxyName = fmt.Sprintf("%s / %s", source.Name, proxyName)
	qualityStatus := ProxyQualityDegraded
	if evaluation.LastError == "" && evaluation.LatencyMs != nil && *evaluation.LatencyMs <= 1500 {
		qualityStatus = ProxyQualityHealthy
	}
	proxyHost := sidecarProxyHost()
	proxy, exists, err := s.findProxyByAddress(ctx, proxyHost, port, "", "")
	if err != nil {
		return err
	}
	if !exists {
		proxy, err = s.CreateProxy(ctx, &CreateProxyInput{Name: proxyName, Protocol: "socks5", Host: proxyHost, Port: port, Source: "subscription", ProxyType: "sidecar", Provider: source.Provider, Region: defaultString(evaluation.ExitCountryCode, evaluation.Country), QualityStatus: qualityStatus, Weight: maxInt(1, evaluation.Score)})
		if err != nil {
			return err
		}
	}
	if proxy == nil {
		return errors.New("sidecar proxy create returned nil proxy")
	}
	if _, err := s.entClient.ExecContext(ctx, `
UPDATE proxy_sidecar_endpoints
SET proxy_id = $2, updated_at = NOW()
WHERE node_id = $1 AND deleted_at IS NULL`, nodeID, proxy.ID); err != nil {
		return err
	}
	if _, err := s.UpdateProxy(ctx, proxy.ID, &UpdateProxyInput{Name: proxyName, Protocol: "socks5", Host: proxyHost, Port: port, Status: proxy.Status, Source: "subscription", ProxyType: "sidecar", Provider: source.Provider, Region: defaultString(evaluation.ExitCountryCode, evaluation.Country), ExitIP: evaluation.ExitIP, QualityStatus: qualityStatus, Weight: intPtr(maxInt(1, evaluation.Score))}); err != nil {
		return err
	}
	return s.refreshProxySidecarEndpointReadiness(ctx, nodeID, proxy.ID, port)
}
func (s *adminServiceImpl) findProxyByAddress(ctx context.Context, host string, port int, username, password string) (*Proxy, bool, error) {
	if s == nil || s.entClient == nil {
		return nil, false, infraerrors.ServiceUnavailable("PROXY_UNAVAILABLE", "proxy service unavailable")
	}
	rows, err := s.entClient.QueryContext(ctx, `
SELECT id, name, protocol, host, port, COALESCE(username, ''), COALESCE(password, ''),
       status, created_at, updated_at,
       COALESCE(source, 'manual'), COALESCE(proxy_type, 'datacenter'), COALESCE(provider, ''),
       COALESCE(region, ''), COALESCE(exit_ip, ''), COALESCE(quality_status, 'healthy'),
       max_bound_accounts, max_active_accounts, COALESCE(weight, 100), last_checked_at,
       COALESCE(failure_count, 0)
FROM proxies
WHERE host = $1
  AND port = $2
  AND COALESCE(username, '') = $3
  AND COALESCE(password, '') = $4
  AND deleted_at IS NULL
ORDER BY id DESC
LIMIT 1`, host, port, username, password)
	if err != nil {
		return nil, false, err
	}
	defer func() {
		_ = rows.Close()
	}()
	if rows.Next() {
		var p Proxy
		if err := rows.Scan(&p.ID, &p.Name, &p.Protocol, &p.Host, &p.Port, &p.Username, &p.Password, &p.Status, &p.CreatedAt, &p.UpdatedAt, &p.Source, &p.ProxyType, &p.Provider, &p.Region, &p.ExitIP, &p.QualityStatus, &p.MaxBoundAccounts, &p.MaxActiveAccounts, &p.Weight, &p.LastCheckedAt, &p.FailureCount); err != nil {
			return nil, false, err
		}
		return &p, true, nil
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return nil, false, nil
}

// restoreDeletedProxy reactivates the newest soft-deleted proxy with the same
// endpoint and credentials. The caller has already validated the create input.
func (s *adminServiceImpl) restoreDeletedProxy(ctx context.Context, input *CreateProxyInput, fallbackMode string) (*Proxy, error) {
	if s == nil || s.entClient == nil || input == nil {
		return nil, nil
	}
	rows, err := s.entClient.QueryContext(ctx, `
UPDATE proxies
SET name = $1,
    protocol = $2,
    host = $3,
    port = $4,
    username = NULLIF($5, ''),
    password = NULLIF($6, ''),
    status = $7,
    expires_at = $8,
    fallback_mode = $9,
    backup_proxy_id = $10,
    expiry_warn_days = $11,
    last_checked_at = NULL,
    failure_count = 0,
    deleted_at = NULL,
    updated_at = NOW()
WHERE id = (
    SELECT id
    FROM proxies
    WHERE host = $3
      AND port = $4
      AND COALESCE(username, '') = $5
      AND COALESCE(password, '') = $6
      AND deleted_at IS NOT NULL
      AND NOT EXISTS (
          SELECT 1
          FROM proxies active_proxy
          WHERE active_proxy.host = $3
            AND active_proxy.port = $4
            AND COALESCE(active_proxy.username, '') = $5
            AND COALESCE(active_proxy.password, '') = $6
            AND active_proxy.deleted_at IS NULL
      )
    ORDER BY id DESC
    LIMIT 1
)
	RETURNING id`, input.Name, input.Protocol, input.Host, input.Port,
		strings.TrimSpace(input.Username), strings.TrimSpace(input.Password), StatusActive,
		input.ExpiresAt, fallbackMode, input.BackupProxyID, input.ExpiryWarnDays)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var id int64
	if err := rows.Scan(&id); err != nil {
		return nil, err
	}
	return s.proxyRepo.GetByID(ctx, id)
}

func applyProxyInputMetadata(proxy *Proxy, input *CreateProxyInput) {
	if proxy == nil || input == nil {
		return
	}
	proxy.Source = defaultString(input.Source, "manual")
	proxy.ProxyType = defaultString(input.ProxyType, "datacenter")
	proxy.Provider = strings.TrimSpace(input.Provider)
	proxy.Region = strings.TrimSpace(input.Region)
	proxy.ExitIP = strings.TrimSpace(input.ExitIP)
	proxy.QualityStatus = normalizeProxyQualityStatus(input.QualityStatus)
	proxy.MaxBoundAccounts = input.MaxBoundAccounts
	proxy.MaxActiveAccounts = input.MaxActiveAccounts
	if input.Weight > 0 {
		proxy.Weight = input.Weight
	} else {
		proxy.Weight = 100
	}
}
func applyProxyUpdateMetadata(proxy *Proxy, input *UpdateProxyInput) {
	if proxy == nil || input == nil {
		return
	}
	if strings.TrimSpace(input.Source) != "" {
		proxy.Source = strings.TrimSpace(input.Source)
	}
	if strings.TrimSpace(input.ProxyType) != "" {
		proxy.ProxyType = strings.TrimSpace(input.ProxyType)
	}
	if input.Provider != "" {
		proxy.Provider = strings.TrimSpace(input.Provider)
	}
	if input.Region != "" {
		proxy.Region = strings.TrimSpace(input.Region)
	}
	if input.ExitIP != "" {
		proxy.ExitIP = strings.TrimSpace(input.ExitIP)
	}
	if input.QualityStatus != "" {
		proxy.QualityStatus = normalizeProxyQualityStatus(input.QualityStatus)
	}
	if input.MaxBoundAccounts != nil {
		proxy.MaxBoundAccounts = input.MaxBoundAccounts
	}
	if input.MaxActiveAccounts != nil {
		proxy.MaxActiveAccounts = input.MaxActiveAccounts
	}
	if input.Weight != nil && *input.Weight > 0 {
		proxy.Weight = *input.Weight
	}
}
func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
func nullIfBlank(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func normalizeProxyQualityStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case ProxyQualityDegraded, "warn", "warning", "challenge":
		return ProxyQualityDegraded
	case ProxyQualityFailed, "fail":
		return ProxyQualityFailed
	case ProxyQualityCooling:
		return ProxyQualityCooling
	default:
		return ProxyQualityHealthy
	}
}
func normalizeDirectFallbackMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case DirectFallbackManualOnly:
		return DirectFallbackManualOnly
	case DirectFallbackGlobal:
		return DirectFallbackGlobal
	default:
		return DirectFallbackOff
	}
}
func (s *adminServiceImpl) attachProxyMetadata(ctx context.Context, proxies []Proxy) {
	if len(proxies) == 0 || s == nil || s.entClient == nil {
		return
	}
	ids := make([]string, 0, len(proxies))
	for i := range proxies {
		ids = append(ids, strconv.FormatInt(proxies[i].ID, 10))
	}
	rows, err := s.entClient.QueryContext(ctx, fmt.Sprintf(`
SELECT id, source, proxy_type, COALESCE(provider, ''), COALESCE(region, ''), COALESCE(exit_ip, ''),
       quality_status, max_bound_accounts, max_active_accounts, weight, last_checked_at, failure_count
FROM proxies
WHERE id IN (%s)`, strings.Join(ids, ",")))
	if err != nil {
		return
	}
	defer func() {
		_ = rows.Close()
	}()
	type meta struct {
		Source            string
		ProxyType         string
		Provider          string
		Region            string
		ExitIP            string
		QualityStatus     string
		MaxBoundAccounts  *int
		MaxActiveAccounts *int
		Weight            int
		LastCheckedAt     *time.Time
		FailureCount      int
	}
	byID := map[int64]meta{}
	for rows.Next() {
		var id int64
		var m meta
		if err := rows.Scan(&id, &m.Source, &m.ProxyType, &m.Provider, &m.Region, &m.ExitIP, &m.QualityStatus, &m.MaxBoundAccounts, &m.MaxActiveAccounts, &m.Weight, &m.LastCheckedAt, &m.FailureCount); err != nil {
			continue
		}
		byID[id] = m
	}
	for i := range proxies {
		m, ok := byID[proxies[i].ID]
		if !ok {
			proxies[i].Source = "manual"
			proxies[i].ProxyType = "datacenter"
			proxies[i].QualityStatus = ProxyQualityHealthy
			proxies[i].Weight = 100
			continue
		}
		proxies[i].Source = m.Source
		proxies[i].ProxyType = m.ProxyType
		proxies[i].Provider = m.Provider
		proxies[i].Region = m.Region
		proxies[i].ExitIP = m.ExitIP
		proxies[i].QualityStatus = m.QualityStatus
		proxies[i].MaxBoundAccounts = m.MaxBoundAccounts
		proxies[i].MaxActiveAccounts = m.MaxActiveAccounts
		proxies[i].Weight = m.Weight
		proxies[i].LastCheckedAt = m.LastCheckedAt
		proxies[i].FailureCount = m.FailureCount
	}
}
func (s *adminServiceImpl) saveProxyMetadata(ctx context.Context, id int64, proxy *Proxy) error {
	if proxy == nil || s == nil || s.entClient == nil {
		return nil
	}
	_, err := s.entClient.ExecContext(ctx, `
UPDATE proxies
SET source = $2, proxy_type = $3, provider = NULLIF($4, ''), region = NULLIF($5, ''),
    exit_ip = NULLIF($6, ''), quality_status = $7, max_bound_accounts = $8,
    max_active_accounts = $9, weight = $10, failure_count = $11
WHERE id = $1`, id, defaultString(proxy.Source, "manual"), defaultString(proxy.ProxyType, "datacenter"), proxy.Provider, proxy.Region, proxy.ExitIP, normalizeProxyQualityStatus(proxy.QualityStatus), proxy.MaxBoundAccounts, proxy.MaxActiveAccounts, positiveOrDefaultInt(proxy.Weight, 100), proxy.FailureCount)
	return err
}
func positiveOrDefaultInt(v, fallback int) int {
	if v <= 0 {
		return fallback
	}
	return v
}
func accountProxyIdentityKey(account *Account) string {
	if account == nil {
		return ""
	}
	platform := strings.ToLower(strings.TrimSpace(account.Platform))
	kind, raw := accountProxyIdentityRaw(account)
	if raw == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(platform + ":" + kind + ":" + raw))
	return platform + ":" + kind + ":" + hex.EncodeToString(sum[:])
}
func accountProxyIdentityRaw(account *Account) (string, string) {
	if account == nil {
		return "", ""
	}
	cred := func(key string) string {
		return strings.TrimSpace(account.GetCredential(key))
	}
	lowerCred := func(key string) string {
		return strings.ToLower(cred(key))
	}
	switch account.Platform {
	case PlatformOpenAI:
		if account.Type == AccountTypeOAuth {
			if v := cred("chatgpt_account_id"); v != "" {
				return "chatgpt_account_id", v
			}
			if userID := cred("chatgpt_user_id"); userID != "" {
				if orgID := cred("organization_id"); orgID != "" {
					return "chatgpt_user_org", userID + "|" + orgID
				}
			}
			if idToken := cred("id_token"); idToken != "" {
				if claims, err := openaiutil.DecodeIDToken(idToken); err == nil && claims.Sub != "" {
					return "id_token_sub", claims.Sub
				}
				if claims, err := openaiutil.ParseIDToken(idToken); err == nil && claims.Sub != "" {
					return "id_token_sub", claims.Sub
				}
			}
			if v := lowerCred("email"); v != "" {
				return "email", v
			}
		}
		if v := cred("api_key"); v != "" {
			return "api_key", v
		}
	case PlatformGemini, PlatformAnthropic, PlatformAntigravity:
		for _, key := range []string{"account_id", "user_id", "subject", "sub"} {
			if v := cred(key); v != "" {
				return key, v
			}
		}
		if idToken := cred("id_token"); idToken != "" {
			if sub := jwtSubWithoutValidation(idToken); sub != "" {
				return "id_token_sub", sub
			}
		}
		if v := lowerCred("email"); v != "" {
			return "email", v
		}
		if v := cred("api_key"); v != "" {
			return "api_key", v
		}
	}
	if account.Type == AccountTypeServiceAccount {
		if v := cred("client_email"); v != "" {
			return "client_email", strings.ToLower(v)
		}
		privateKeyID := cred("private_key_id")
		projectID := cred("project_id")
		if privateKeyID != "" && projectID != "" {
			return "private_key_project", privateKeyID + "|" + projectID
		}
	}
	if account.ID > 0 {
		return "account_id", strconv.FormatInt(account.ID, 10)
	}
	return "", ""
}
func jwtSubWithoutValidation(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		Sub string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	return strings.TrimSpace(claims.Sub)
}
func (s *adminServiceImpl) assignProxyForAccount(ctx context.Context, account *Account, force bool) (*Proxy, error) {
	if account == nil {
		return nil, ErrAccountNotFound
	}
	if s == nil || s.entClient == nil || s.accountRepo == nil {
		return nil, infraerrors.ServiceUnavailable("PROXY_DISPATCH_UNAVAILABLE", "proxy dispatch service unavailable")
	}
	if account.ProxyID != nil && *account.ProxyID > 0 && !force {
		proxy, err := s.GetProxy(ctx, *account.ProxyID)
		if err != nil {
			return nil, err
		}
		if err := s.recordAccountProxyBinding(ctx, account, proxy.ID, ProxyBindingSourceManual, ProxyBindingStatusActive); err != nil {
			return nil, err
		}
		return proxy, nil
	}
	identityKey := accountProxyIdentityKey(account)
	if identityKey == "" {
		return nil, infraerrors.BadRequest("ACCOUNT_IDENTITY_UNAVAILABLE", "account identity is unavailable")
	}
	if proxyID, ok, err := s.findHistoricalProxy(ctx, identityKey); err != nil {
		return nil, err
	} else if ok {
		account.ProxyID = &proxyID
		account.Proxy = nil
		if err := s.accountRepo.Update(ctx, account); err != nil {
			return nil, err
		}
		if err := s.recordAccountProxyBinding(ctx, account, proxyID, ProxyBindingSourceRestored, ProxyBindingStatusActive); err != nil {
			return nil, err
		}
		return s.GetProxy(ctx, proxyID)
	}
	proxy, err := s.chooseNewProxy(ctx)
	if err != nil {
		return nil, err
	}
	account.ProxyID = &proxy.ID
	account.Proxy = nil
	if err := s.accountRepo.Update(ctx, account); err != nil {
		return nil, err
	}
	if err := s.recordAccountProxyBinding(ctx, account, proxy.ID, ProxyBindingSourceAuto, ProxyBindingStatusActive); err != nil {
		return nil, err
	}
	return proxy, nil
}
func (s *adminServiceImpl) findHistoricalProxy(ctx context.Context, identityKey string) (int64, bool, error) {
	rows, err := s.entClient.QueryContext(ctx, `
SELECT b.proxy_id
FROM account_proxy_bindings b
JOIN proxies p ON p.id = b.proxy_id AND p.deleted_at IS NULL
WHERE b.identity_key = $1
  AND b.status IN ('active', 'account_deleted', 'inactive')
  AND p.status = 'active'
  AND COALESCE(p.quality_status, 'healthy') NOT IN ('failed', 'cooling')
ORDER BY b.last_used_at DESC, b.id DESC
LIMIT 1`, identityKey)
	if err != nil {
		return 0, false, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var proxyID int64
	if rows.Next() {
		if err := rows.Scan(&proxyID); err != nil {
			return 0, false, err
		}
		return proxyID, true, nil
	}
	return 0, false, rows.Err()
}
func (s *adminServiceImpl) chooseNewProxy(ctx context.Context) (*Proxy, error) {
	rows, err := s.entClient.QueryContext(ctx, `
SELECT p.id, p.name, p.protocol, p.host, p.port, COALESCE(p.username, ''), COALESCE(p.password, ''),
       p.status, p.created_at, p.updated_at,
       COALESCE(p.source, 'manual'), COALESCE(p.proxy_type, 'datacenter'), COALESCE(p.provider, ''),
       COALESCE(p.region, ''), COALESCE(p.exit_ip, ''), COALESCE(p.quality_status, 'healthy'),
       p.max_bound_accounts, p.max_active_accounts, COALESCE(p.weight, 100), p.last_checked_at,
       COALESCE(p.failure_count, 0),
       COALESCE(bound.bound_count, 0), COALESCE(active.active_count, 0), COALESCE(active.current_concurrency, 0)
FROM proxies p
LEFT JOIN (
  SELECT proxy_id, COUNT(DISTINCT identity_key) AS bound_count
  FROM account_proxy_bindings
  WHERE status = 'active'
  GROUP BY proxy_id
) bound ON bound.proxy_id = p.id
LEFT JOIN (
  SELECT proxy_id, COUNT(*) AS active_count, COALESCE(SUM(concurrency), 0) AS current_concurrency
  FROM accounts
  WHERE deleted_at IS NULL AND status = 'active' AND proxy_id IS NOT NULL
  GROUP BY proxy_id
) active ON active.proxy_id = p.id
WHERE p.deleted_at IS NULL
  AND p.status = 'active'
  AND COALESCE(p.quality_status, 'healthy') NOT IN ('failed', 'cooling')
  AND (p.max_bound_accounts IS NULL OR COALESCE(bound.bound_count, 0) < p.max_bound_accounts)
  AND (p.max_active_accounts IS NULL OR COALESCE(active.active_count, 0) < p.max_active_accounts)
ORDER BY COALESCE(active.active_count, 0) ASC,
         COALESCE(bound.bound_count, 0) ASC,
         COALESCE(active.current_concurrency, 0) ASC,
         COALESCE(p.failure_count, 0) ASC,
         COALESCE(p.weight, 100) DESC,
         p.id ASC
LIMIT 1`)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	if rows.Next() {
		var p Proxy
		var bound, active, concurrency int64
		if err := rows.Scan(&p.ID, &p.Name, &p.Protocol, &p.Host, &p.Port, &p.Username, &p.Password, &p.Status, &p.CreatedAt, &p.UpdatedAt, &p.Source, &p.ProxyType, &p.Provider, &p.Region, &p.ExitIP, &p.QualityStatus, &p.MaxBoundAccounts, &p.MaxActiveAccounts, &p.Weight, &p.LastCheckedAt, &p.FailureCount, &bound, &active, &concurrency); err != nil {
			return nil, err
		}
		return &p, nil
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nil, infraerrors.ServiceUnavailable("NO_AVAILABLE_PROXY", "no available proxy")
}
func (s *adminServiceImpl) chooseReplacementProxy(ctx context.Context, identityKey string, currentProxyID int64) (*Proxy, error) {
	rows, err := s.entClient.QueryContext(ctx, `
SELECT p.id, p.name, p.protocol, p.host, p.port, COALESCE(p.username, ''), COALESCE(p.password, ''),
       p.status, p.created_at, p.updated_at,
       COALESCE(p.source, 'manual'), COALESCE(p.proxy_type, 'datacenter'), COALESCE(p.provider, ''),
       COALESCE(p.region, ''), COALESCE(p.exit_ip, ''), COALESCE(p.quality_status, 'healthy'),
       p.max_bound_accounts, p.max_active_accounts, COALESCE(p.weight, 100), p.last_checked_at,
       COALESCE(p.failure_count, 0),
       COALESCE(bound.bound_count, 0), COALESCE(active.active_count, 0), COALESCE(active.current_concurrency, 0)
FROM proxies p
LEFT JOIN (
  SELECT proxy_id, COUNT(DISTINCT identity_key) AS bound_count
  FROM account_proxy_bindings
  WHERE status = 'active'
  GROUP BY proxy_id
) bound ON bound.proxy_id = p.id
LEFT JOIN (
  SELECT proxy_id, COUNT(*) AS active_count, COALESCE(SUM(concurrency), 0) AS current_concurrency
  FROM accounts
  WHERE deleted_at IS NULL AND status = 'active' AND proxy_id IS NOT NULL
  GROUP BY proxy_id
) active ON active.proxy_id = p.id
WHERE p.deleted_at IS NULL
  AND p.id <> $2
  AND p.status = 'active'
  AND COALESCE(p.quality_status, 'healthy') NOT IN ('failed', 'cooling')
  AND NOT EXISTS (
    SELECT 1 FROM account_proxy_bindings b
    WHERE b.identity_key = $1
      AND b.proxy_id = p.id
      AND b.status = 'proxy_unavailable'
  )
  AND (p.max_bound_accounts IS NULL OR COALESCE(bound.bound_count, 0) < p.max_bound_accounts)
  AND (p.max_active_accounts IS NULL OR COALESCE(active.active_count, 0) < p.max_active_accounts)
ORDER BY COALESCE(active.active_count, 0) ASC,
         COALESCE(bound.bound_count, 0) ASC,
         COALESCE(active.current_concurrency, 0) ASC,
         COALESCE(p.failure_count, 0) ASC,
         COALESCE(p.weight, 100) DESC,
         p.id ASC
LIMIT 1`, identityKey, currentProxyID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	if rows.Next() {
		var p Proxy
		var bound, active, concurrency int64
		if err := rows.Scan(&p.ID, &p.Name, &p.Protocol, &p.Host, &p.Port, &p.Username, &p.Password, &p.Status, &p.CreatedAt, &p.UpdatedAt, &p.Source, &p.ProxyType, &p.Provider, &p.Region, &p.ExitIP, &p.QualityStatus, &p.MaxBoundAccounts, &p.MaxActiveAccounts, &p.Weight, &p.LastCheckedAt, &p.FailureCount, &bound, &active, &concurrency); err != nil {
			return nil, err
		}
		return &p, nil
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nil, infraerrors.ServiceUnavailable("NO_AVAILABLE_PROXY", "no available proxy")
}
func truncateProxyFailureReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if len(reason) <= 500 {
		return reason
	}
	return reason[:500]
}
func (s *adminServiceImpl) recordAccountProxyBinding(ctx context.Context, account *Account, proxyID int64, source, status string) error {
	if s == nil || s.entClient == nil {
		return nil
	}
	identityKey := accountProxyIdentityKey(account)
	if identityKey == "" || proxyID <= 0 {
		return nil
	}
	source = defaultString(source, ProxyBindingSourceAuto)
	status = defaultString(status, ProxyBindingStatusActive)
	_, err := s.entClient.ExecContext(ctx, `
INSERT INTO account_proxy_bindings (identity_key, platform, account_id, proxy_id, status, source, first_used_at, last_used_at, use_count, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW(), 1, NOW(), NOW())
ON CONFLICT (identity_key, proxy_id)
DO UPDATE SET account_id = EXCLUDED.account_id,
              platform = EXCLUDED.platform,
              status = EXCLUDED.status,
              source = EXCLUDED.source,
              last_used_at = NOW(),
              use_count = account_proxy_bindings.use_count + 1,
              updated_at = NOW()`, identityKey, account.Platform, account.ID, proxyID, status, source)
	return err
}
func (s *adminServiceImpl) deactivateAccountProxyBindings(ctx context.Context, account *Account) error {
	if s == nil || s.entClient == nil {
		return nil
	}
	identityKey := accountProxyIdentityKey(account)
	if identityKey == "" {
		return nil
	}
	_, err := s.entClient.ExecContext(ctx, `
UPDATE account_proxy_bindings
SET status = 'inactive', updated_at = NOW()
WHERE identity_key = $1 AND account_id = $2 AND status = 'active'`, identityKey, account.ID)
	return err
}
func (s *adminServiceImpl) markAccountProxyBindingsDeleted(ctx context.Context, account *Account) error {
	if s == nil || s.entClient == nil {
		return nil
	}
	identityKey := accountProxyIdentityKey(account)
	if identityKey == "" {
		return nil
	}
	_, err := s.entClient.ExecContext(ctx, `
UPDATE account_proxy_bindings
SET account_id = NULL, status = 'account_deleted', updated_at = NOW()
WHERE identity_key = $1 OR account_id = $2`, identityKey, account.ID)
	return err
}
func (s *adminServiceImpl) listProxyBindingsByIdentity(ctx context.Context, identityKey string) ([]AccountProxyBinding, error) {
	rows, err := s.entClient.QueryContext(ctx, `
SELECT b.id, b.identity_key, b.platform, b.account_id, b.proxy_id, b.status, b.source,
       b.first_used_at, b.last_used_at, b.last_success_at, b.last_failure_at, b.use_count,
       COALESCE(b.failure_count, 0), COALESCE(b.last_failure_reason, ''),
       p.name, p.protocol, p.host, p.port, COALESCE(p.username, ''), COALESCE(p.password, ''), p.status, p.created_at, p.updated_at
FROM account_proxy_bindings b
JOIN proxies p ON p.id = b.proxy_id
WHERE b.identity_key = $1
ORDER BY b.last_used_at DESC, b.id DESC`, identityKey)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	var out []AccountProxyBinding
	for rows.Next() {
		var b AccountProxyBinding
		var p Proxy
		if err := rows.Scan(&b.ID, &b.IdentityKey, &b.Platform, &b.AccountID, &b.ProxyID, &b.Status, &b.Source, &b.FirstUsedAt, &b.LastUsedAt, &b.LastSuccessAt, &b.LastFailureAt, &b.UseCount, &b.FailureCount, &b.LastFailureReason, &p.Name, &p.Protocol, &p.Host, &p.Port, &p.Username, &p.Password, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.ID = b.ProxyID
		b.Proxy = &p
		out = append(out, b)
	}
	return out, rows.Err()
}
func (s *adminServiceImpl) proxyRelationshipForAccount(ctx context.Context, account *Account) (*ProxyRelationship, error) {
	if account == nil {
		return nil, ErrAccountNotFound
	}
	identityKey := accountProxyIdentityKey(account)
	rel := &ProxyRelationship{AccountID: account.ID, AccountName: account.Name, Platform: account.Platform, AccountType: account.Type, AccountStatus: account.Status, IdentityKey: identityKey, ProxySource: ProxyBindingSourceAuto}
	if account.ProxyID == nil {
		rel.NoAvailableProxy = true
		return rel, nil
	}
	if proxy, err := s.GetProxy(ctx, *account.ProxyID); err == nil {
		rel.CurrentProxy = proxy
	}
	rows, err := s.entClient.QueryContext(ctx, `
SELECT b.id, b.status, b.source, b.last_used_at, COALESCE(b.last_failure_reason, ''),
       (SELECT COUNT(DISTINCT proxy_id) FROM account_proxy_bindings WHERE identity_key = $1) AS history_count,
       (SELECT COUNT(DISTINCT identity_key) FROM account_proxy_bindings WHERE proxy_id = $2 AND status = 'active') AS bound_count,
       (SELECT COUNT(*) FROM accounts WHERE proxy_id = $2 AND deleted_at IS NULL AND status = 'active') AS active_count,
       (SELECT COALESCE(SUM(concurrency), 0) FROM accounts WHERE proxy_id = $2 AND deleted_at IS NULL AND status = 'active') AS current_concurrency
FROM account_proxy_bindings b
WHERE b.identity_key = $1 AND b.proxy_id = $2
ORDER BY b.last_used_at DESC, b.id DESC
LIMIT 1`, identityKey, *account.ProxyID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	if rows.Next() {
		var bindingID int64
		var lastUsed time.Time
		if err := rows.Scan(&bindingID, &rel.BindingStatus, &rel.ProxySource, &lastUsed, &rel.LastFailureReason, &rel.HistoryProxyCount, &rel.BoundAccountCount, &rel.ActiveAccountCount, &rel.CurrentConcurrency); err != nil {
			return nil, err
		}
		rel.BindingID = &bindingID
		rel.LastUsedAt = &lastUsed
	}
	if rel.BindingStatus == "" && account.ProxyID != nil {
		rel.BindingStatus = ProxyBindingStatusActive
		rel.ProxySource = ProxyBindingSourceManual
	}
	return rel, nil
}
func parseProxyImportItems(content, provider string) []ProxyImportPreviewItem {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	if strings.HasPrefix(content, "{") {
		if items := parseSingBoxJSON(content, provider); len(items) > 0 {
			return items
		}
	}
	if strings.Contains(content, "proxies:") {
		if items := parseClashYAML(content, provider); len(items) > 0 {
			return items
		}
	}
	lines := strings.Split(content, "\n")
	items := make([]ProxyImportPreviewItem, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		item := parseProxyLine(line, provider)
		items = append(items, item)
	}
	return dedupeImportItems(items)
}
func parseProxyLine(line, provider string) ProxyImportPreviewItem {
	item := ProxyImportPreviewItem{Raw: line, Provider: provider, Source: "import", QualityStatus: ProxyQualityHealthy}
	if u, err := url.Parse(line); err == nil && u.Scheme != "" {
		scheme := strings.ToLower(u.Scheme)
		switch scheme {
		case "http", "https", "socks5", "socks5h":
			port, _ := strconv.Atoi(u.Port())
			item.Name = strings.TrimPrefix(line, scheme+"://")
			item.Protocol = scheme
			item.Host = u.Hostname()
			item.Port = port
			item.ProxyType = "direct"
			if u.User != nil {
				item.Username = u.User.Username()
				item.Password, _ = u.User.Password()
			}
			item.Valid = item.Host != "" && item.Port > 0
			if !item.Valid {
				item.Error = "invalid proxy url"
			}
			return item
		case "ss", "vmess", "vless", "trojan", "hysteria2", "tuic", "wireguard", "anytls":
			item.Protocol = scheme
			item.Host = u.Hostname()
			item.Port, _ = strconv.Atoi(u.Port())
			if u.User != nil {
				item.Username = u.User.Username()
			}
			item.ProxyType = "sidecar"
			item.SidecarRequired = true
			item.SidecarHint = "需要通过 mihomo / sing-box / xray sidecar 转成本地 http/socks5 出口"
			item.Valid = item.Host != "" && item.Port > 0
			if isSupportedSubscriptionSidecarProtocol(scheme) && item.Username == "" {
				item.Valid = false
				item.Error = "sidecar proxy URL is missing credentials"
			}
			if !item.Valid && item.Error == "" {
				item.Error = "invalid sidecar proxy URL"
			}
			item.Name = strings.TrimSpace(u.Fragment)
			if item.Name == "" {
				item.Name = scheme + " node"
			}
			return item
		}
	}
	parts := strings.Split(line, ":")
	if len(parts) >= 2 {
		port, err := strconv.Atoi(parts[1])
		if err == nil {
			item.Protocol = "http"
			item.Host = parts[0]
			item.Port = port
			if len(parts) >= 4 {
				item.Username = parts[2]
				item.Password = strings.Join(parts[3:], ":")
			}
			item.Name = item.Host + ":" + strconv.Itoa(item.Port)
			item.ProxyType = "direct"
			item.Valid = strings.TrimSpace(item.Host) != ""
			return item
		}
	}
	item.Error = "unsupported proxy format"
	return item
}
func parseClashYAML(content, provider string) []ProxyImportPreviewItem {
	var root struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal([]byte(content), &root); err != nil {
		return nil
	}
	items := make([]ProxyImportPreviewItem, 0, len(root.Proxies))
	for _, p := range root.Proxies {
		typ := strings.ToLower(fmt.Sprint(p["type"]))
		item := ProxyImportPreviewItem{Name: strings.TrimSpace(fmt.Sprint(p["name"])), Protocol: typ, Host: strings.TrimSpace(fmt.Sprint(p["server"])), Username: strings.TrimSpace(fmt.Sprint(p["username"])), Password: strings.TrimSpace(fmt.Sprint(p["password"])), Provider: provider, Source: "clash", QualityStatus: ProxyQualityHealthy}
		item.Port, _ = strconv.Atoi(fmt.Sprint(p["port"]))
		switch typ {
		case "http", "https", "socks5", "socks5h":
			item.ProxyType = "direct"
			item.Valid = item.Host != "" && item.Port > 0
		default:
			item.ProxyType = "sidecar"
			item.SidecarRequired = true
			item.SidecarHint = "Clash/Mihomo 节点需要通过 sidecar 暴露本地 http/socks5 出口"
			item.Valid = item.Host != "" && item.Port > 0
			if isSupportedSubscriptionSidecarProtocol(typ) && item.Username == "" && item.Password == "" {
				item.Valid = false
				item.Error = "sidecar proxy is missing credentials"
			}
		}
		items = append(items, item)
	}
	return dedupeImportItems(items)
}
func parseSingBoxJSON(content, provider string) []ProxyImportPreviewItem {
	var root struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(content), &root); err != nil {
		return nil
	}
	items := make([]ProxyImportPreviewItem, 0, len(root.Outbounds))
	for _, o := range root.Outbounds {
		typ := strings.ToLower(fmt.Sprint(o["type"]))
		if typ == "selector" || typ == "urltest" || typ == "direct" || typ == "block" {
			continue
		}
		item := ProxyImportPreviewItem{Name: strings.TrimSpace(fmt.Sprint(o["tag"])), Protocol: typ, Host: strings.TrimSpace(fmt.Sprint(o["server"])), Provider: provider, Source: "sing-box", QualityStatus: ProxyQualityHealthy}
		item.Port, _ = strconv.Atoi(fmt.Sprint(o["server_port"]))
		switch typ {
		case "http", "socks", "socks5":
			item.Protocol = "socks5"
			if typ == "http" {
				item.Protocol = "http"
			}
			item.ProxyType = "direct"
			item.Valid = item.Host != "" && item.Port > 0
		default:
			item.ProxyType = "sidecar"
			item.SidecarRequired = true
			item.SidecarHint = "sing-box 非 HTTP 原生节点需要通过 sidecar 暴露本地出口"
			item.Valid = false
			item.Error = "sing-box JSON sidecar import requires a supported raw URI"
		}
		items = append(items, item)
	}
	return dedupeImportItems(items)
}
func dedupeImportItems(items []ProxyImportPreviewItem) []ProxyImportPreviewItem {
	seen := map[string]bool{}
	out := make([]ProxyImportPreviewItem, 0, len(items))
	for _, item := range items {
		key := proxyImportItemKey(item)
		item.Key = key
		if key != "" && seen[key] {
			item.Duplicate = true
			item.Selected = false
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}
func proxyImportItemKey(item ProxyImportPreviewItem) string {
	if item.SidecarRequired {
		sum := sha256.Sum256([]byte(item.Raw + item.Name + item.Protocol + item.Host))
		return "sidecar:" + hex.EncodeToString(sum[:8])
	}
	return strings.ToLower(fmt.Sprintf("%s://%s:%d:%s", item.Protocol, item.Host, item.Port, item.Username))
}
func looksLikeSubscriptionURL(content string) bool {
	content = strings.TrimSpace(content)
	return strings.HasPrefix(content, "http://") || strings.HasPrefix(content, "https://")
}
func fetchProxySubscription(ctx context.Context, rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", infraerrors.BadRequest("PROXY_SUBSCRIPTION_URL_REQUIRED", "subscription URL is required")
	}
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return "", infraerrors.BadRequest("PROXY_SUBSCRIPTION_URL_INVALID", "invalid subscription URL").WithCause(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return "", infraerrors.BadRequest("PROXY_SUBSCRIPTION_URL_INVALID", "invalid subscription URL").WithCause(err)
	}
	req.Header.Set("User-Agent", proxyQualityClientUserAgent)
	req.Header.Set("Accept", "*/*")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || subscriptionFetchTimedOut(err) {
			return "", infraerrors.GatewayTimeout("PROXY_SUBSCRIPTION_FETCH_TIMEOUT", "subscription request timed out").WithCause(err)
		}
		return "", infraerrors.BadRequest("PROXY_SUBSCRIPTION_FETCH_FAILED", subscriptionFetchErrorMessage(parsedURL.Host, err)).WithCause(err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", infraerrors.BadRequest("PROXY_SUBSCRIPTION_FETCH_FAILED", fmt.Sprintf("subscription URL returned HTTP %d", resp.StatusCode))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return "", infraerrors.BadRequest("PROXY_SUBSCRIPTION_FETCH_FAILED", "failed to read subscription response").WithCause(err)
	}
	body := string(data)
	if strings.TrimSpace(body) == "" {
		return "", infraerrors.BadRequest("PROXY_SUBSCRIPTION_FETCH_FAILED", "subscription response is empty")
	}
	return body, nil
}
func decodeMaybeBase64Subscription(content string) string {
	compact := strings.TrimSpace(content)
	if compact == "" || strings.Contains(compact, "\n") || strings.Contains(compact, "://") || strings.Contains(compact, "proxies:") {
		return ""
	}
	data, err := base64.StdEncoding.DecodeString(compact)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(compact)
	}
	if err != nil {
		return ""
	}
	decoded := strings.TrimSpace(string(data))
	if decoded == "" || (!strings.Contains(decoded, "://") && !strings.Contains(decoded, "proxies:")) {
		return ""
	}
	return decoded
}
func normalizeProxySubscriptionInput(input ProxySubscriptionSourceInput) ProxySubscriptionSourceInput {
	input.Name = strings.TrimSpace(input.Name)
	input.URL = strings.TrimSpace(input.URL)
	input.SourceType = defaultString(input.SourceType, "clash")
	input.Provider = strings.TrimSpace(input.Provider)
	input.Status = defaultString(input.Status, StatusActive)
	input.Runtime = defaultString(input.Runtime, "sing-box")
	input.ReputationProvider = defaultString(input.ReputationProvider, "none")
	if input.SyncEnabled == nil {
		v := true
		input.SyncEnabled = &v
	}
	if input.SidecarEnabled == nil {
		v := false
		input.SidecarEnabled = &v
	}
	if input.ScanEnabled == nil {
		v := true
		input.ScanEnabled = &v
	}
	if input.SyncIntervalMinutes <= 0 {
		input.SyncIntervalMinutes = 1440
	}
	if input.PortStart <= 0 {
		input.PortStart = 31000
	}
	if input.PortEnd < input.PortStart {
		input.PortEnd = input.PortStart + 999
	}
	if input.ScanIntervalMinutes <= 0 {
		input.ScanIntervalMinutes = 360
	}
	if input.HealthCheckIntervalMinutes <= 0 {
		input.HealthCheckIntervalMinutes = 20
	}
	input.Strategy = normalizeProxySubscriptionStrategy(input.Strategy)
	return input
}
func defaultProxySubscriptionStrategy() ProxySubscriptionStrategy {
	return ProxySubscriptionStrategy{MaxParsedNodes: 300, MaxEnabledNodes: 30, MaxActiveSidecarNodes: 3, MaxProbeConcurrency: 1, ScanBatchSize: 5, StandbyNodes: 10, MinCountryCount: 3, MaxCountryCount: 8, MaxNodesPerCountry: 5, MaxLatencyMs: 1200, MinIPCleanScore: 70, MinQualityScore: 65, SelectionMode: "balanced", ReputationCacheHours: 24, ScanBudgetMinutes: 30, ScanBudgetMaxMinutes: 40, ResourceAdaptiveScan: true, MinFreeMemoryMB: 800, PauseFreeMemoryMB: 500, TimeoutSleepAfter: 3, SleepMinutes: 60, ReplaceSameCountryFirst: true}
}
func normalizeProxySubscriptionStrategy(strategy ProxySubscriptionStrategy) ProxySubscriptionStrategy {
	defaults := defaultProxySubscriptionStrategy()
	if strategy.MaxParsedNodes <= 0 {
		strategy.MaxParsedNodes = defaults.MaxParsedNodes
	}
	if strategy.MaxEnabledNodes <= 0 {
		strategy.MaxEnabledNodes = defaults.MaxEnabledNodes
	}
	if strategy.MaxActiveSidecarNodes <= 0 {
		strategy.MaxActiveSidecarNodes = defaults.MaxActiveSidecarNodes
	}
	if strategy.MaxProbeConcurrency <= 0 {
		strategy.MaxProbeConcurrency = defaults.MaxProbeConcurrency
	}
	if strategy.ScanBatchSize <= 0 {
		strategy.ScanBatchSize = defaults.ScanBatchSize
	}
	if strategy.StandbyNodes < 0 {
		strategy.StandbyNodes = defaults.StandbyNodes
	}
	if strategy.MinCountryCount <= 0 {
		strategy.MinCountryCount = defaults.MinCountryCount
	}
	if strategy.MaxCountryCount <= 0 {
		strategy.MaxCountryCount = defaults.MaxCountryCount
	}
	if strategy.MaxNodesPerCountry <= 0 {
		strategy.MaxNodesPerCountry = defaults.MaxNodesPerCountry
	}
	if strategy.MaxLatencyMs <= 0 {
		strategy.MaxLatencyMs = defaults.MaxLatencyMs
	}
	if strategy.MinIPCleanScore <= 0 {
		strategy.MinIPCleanScore = defaults.MinIPCleanScore
	}
	if strategy.MinQualityScore <= 0 {
		strategy.MinQualityScore = defaults.MinQualityScore
	}
	if strings.TrimSpace(strategy.SelectionMode) == "" {
		strategy.SelectionMode = defaults.SelectionMode
	}
	if strategy.ReputationCacheHours <= 0 {
		strategy.ReputationCacheHours = defaults.ReputationCacheHours
	}
	if strategy.ScanBudgetMinutes <= 0 {
		strategy.ScanBudgetMinutes = defaults.ScanBudgetMinutes
	}
	if strategy.ScanBudgetMaxMinutes < strategy.ScanBudgetMinutes {
		strategy.ScanBudgetMaxMinutes = maxInt(defaults.ScanBudgetMaxMinutes, strategy.ScanBudgetMinutes)
	}
	if strategy.MinFreeMemoryMB <= 0 {
		strategy.MinFreeMemoryMB = defaults.MinFreeMemoryMB
	}
	if strategy.PauseFreeMemoryMB <= 0 {
		strategy.PauseFreeMemoryMB = defaults.PauseFreeMemoryMB
	}
	if strategy.PauseFreeMemoryMB >= strategy.MinFreeMemoryMB {
		strategy.MinFreeMemoryMB = maxInt(strategy.PauseFreeMemoryMB+128, defaults.MinFreeMemoryMB)
	}
	if strategy.TimeoutSleepAfter <= 0 {
		strategy.TimeoutSleepAfter = defaults.TimeoutSleepAfter
	}
	if strategy.SleepMinutes <= 0 {
		strategy.SleepMinutes = defaults.SleepMinutes
	}
	return strategy
}
func parseProxySubscriptionStrategy(raw string) ProxySubscriptionStrategy {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return defaultProxySubscriptionStrategy()
	}
	if raw == "{}" {
		return defaultProxySubscriptionStrategy()
	}
	var strategy ProxySubscriptionStrategy
	if err := json.Unmarshal([]byte(raw), &strategy); err != nil {
		return defaultProxySubscriptionStrategy()
	}
	normalized := normalizeProxySubscriptionStrategy(strategy)
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &rawMap); err == nil {
		if _, ok := rawMap["resource_adaptive_scan"]; !ok {
			normalized.ResourceAdaptiveScan = defaultProxySubscriptionStrategy().ResourceAdaptiveScan
		}
		if _, ok := rawMap["replace_same_country_first"]; !ok {
			normalized.ReplaceSameCountryFirst = defaultProxySubscriptionStrategy().ReplaceSameCountryFirst
		}
	}
	return normalized
}
func parseJSONMap(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return map[string]any{}
	}
	return out
}
func subscriptionFetchTimedOut(err error) bool {
	type timeoutError interface{ Timeout() bool }
	var timeoutErr timeoutError
	return errors.As(err, &timeoutErr) && timeoutErr.Timeout()
}
func subscriptionFetchErrorMessage(host string, err error) string {
	message := "failed to fetch subscription URL"
	if host != "" {
		message = fmt.Sprintf("failed to fetch subscription URL from %s", host)
	}
	if err == nil {
		return message
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		err = urlErr.Err
	}
	detail := strings.TrimSpace(err.Error())
	if detail == "" {
		return message
	}
	return fmt.Sprintf("%s: %s", message, detail)
}
