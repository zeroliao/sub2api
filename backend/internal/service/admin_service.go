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
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/authidentity"
	"github.com/Wei-Shaw/sub2api/ent/authidentitychannel"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	openaiutil "github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/util/httputil"
	"gopkg.in/yaml.v3"
)

// AdminService interface defines admin management operations
type AdminService interface {
	// User management
	ListUsers(ctx context.Context, page, pageSize int, filters UserListFilters, sortBy, sortOrder string) ([]User, int64, error)
	GetUser(ctx context.Context, id int64) (*User, error)
	CreateUser(ctx context.Context, input *CreateUserInput) (*User, error)
	UpdateUser(ctx context.Context, id int64, input *UpdateUserInput) (*User, error)
	DeleteUser(ctx context.Context, id int64) error
	UpdateUserBalance(ctx context.Context, userID int64, balance float64, operation string, notes string) (*User, error)
	BatchUpdateConcurrency(ctx context.Context, userIDs []int64, value int, mode string) (int, error)
	GetUserAPIKeys(ctx context.Context, userID int64, page, pageSize int, sortBy, sortOrder string) ([]APIKey, int64, error)
	GetUserUsageStats(ctx context.Context, userID int64, period string) (any, error)
	GetUserRPMStatus(ctx context.Context, userID int64) (*UserRPMStatus, error)
	// GetUserBalanceHistory returns paginated balance/concurrency change records for a user.
	// codeType is optional - pass empty string to return all types.
	// Also returns totalRecharged (sum of all positive balance top-ups).
	GetUserBalanceHistory(ctx context.Context, userID int64, page, pageSize int, codeType string) ([]RedeemCode, int64, float64, error)
	BindUserAuthIdentity(ctx context.Context, userID int64, input AdminBindAuthIdentityInput) (*AdminBoundAuthIdentity, error)

	// Group management
	ListGroups(ctx context.Context, page, pageSize int, platform, status, search string, isExclusive *bool, sortBy, sortOrder string) ([]Group, int64, error)
	GetAllGroups(ctx context.Context) ([]Group, error)
	GetAllGroupsByPlatform(ctx context.Context, platform string) ([]Group, error)
	GetGroup(ctx context.Context, id int64) (*Group, error)
	CreateGroup(ctx context.Context, input *CreateGroupInput) (*Group, error)
	UpdateGroup(ctx context.Context, id int64, input *UpdateGroupInput) (*Group, error)
	DeleteGroup(ctx context.Context, id int64) error
	GetGroupAPIKeys(ctx context.Context, groupID int64, page, pageSize int) ([]APIKey, int64, error)
	GetGroupRateMultipliers(ctx context.Context, groupID int64) ([]UserGroupRateEntry, error)
	ClearGroupRateMultipliers(ctx context.Context, groupID int64) error
	BatchSetGroupRateMultipliers(ctx context.Context, groupID int64, entries []GroupRateMultiplierInput) error
	ClearGroupRPMOverrides(ctx context.Context, groupID int64) error
	BatchSetGroupRPMOverrides(ctx context.Context, groupID int64, entries []GroupRPMOverrideInput) error
	UpdateGroupSortOrders(ctx context.Context, updates []GroupSortOrderUpdate) error

	// API Key management (admin)
	AdminUpdateAPIKeyGroupID(ctx context.Context, keyID int64, groupID *int64) (*AdminUpdateAPIKeyGroupIDResult, error)
	AdminResetAPIKeyRateLimitUsage(ctx context.Context, keyID int64) (*APIKey, error)

	// ReplaceUserGroup 替换用户的专属分组：授予新分组权限、迁移 Key、移除旧分组权限
	ReplaceUserGroup(ctx context.Context, userID, oldGroupID, newGroupID int64) (*ReplaceUserGroupResult, error)

	// Account management
	ListAccounts(ctx context.Context, page, pageSize int, platform, accountType, status, search string, groupID int64, privacyMode string, sortBy, sortOrder string) ([]Account, int64, error)
	GetAccount(ctx context.Context, id int64) (*Account, error)
	GetAccountsByIDs(ctx context.Context, ids []int64) ([]*Account, error)
	CreateAccount(ctx context.Context, input *CreateAccountInput) (*Account, error)
	UpdateAccount(ctx context.Context, id int64, input *UpdateAccountInput) (*Account, error)
	DeleteAccount(ctx context.Context, id int64) error
	RefreshAccountCredentials(ctx context.Context, id int64) (*Account, error)
	ClearAccountError(ctx context.Context, id int64) (*Account, error)
	SetAccountError(ctx context.Context, id int64, errorMsg string) error
	// EnsureOpenAIPrivacy 检查 OpenAI OAuth 账号 privacy_mode，未设置则尝试关闭训练数据共享并持久化。
	EnsureOpenAIPrivacy(ctx context.Context, account *Account) string
	// EnsureAntigravityPrivacy 检查 Antigravity OAuth 账号 privacy_mode，未设置则调用 setUserSettings 并持久化。
	EnsureAntigravityPrivacy(ctx context.Context, account *Account) string
	// ForceOpenAIPrivacy 强制重新设置 OpenAI OAuth 账号隐私，无论当前状态。
	ForceOpenAIPrivacy(ctx context.Context, account *Account) string
	// ForceAntigravityPrivacy 强制重新设置 Antigravity OAuth 账号隐私，无论当前状态。
	ForceAntigravityPrivacy(ctx context.Context, account *Account) string
	SetAccountSchedulable(ctx context.Context, id int64, schedulable bool) (*Account, error)
	BulkUpdateAccounts(ctx context.Context, input *BulkUpdateAccountsInput) (*BulkUpdateAccountsResult, error)
	CheckMixedChannelRisk(ctx context.Context, currentAccountID int64, currentAccountPlatform string, groupIDs []int64) error

	// Proxy management
	ListProxies(ctx context.Context, page, pageSize int, protocol, status, search string, sortBy, sortOrder string) ([]Proxy, int64, error)
	ListProxiesWithAccountCount(ctx context.Context, page, pageSize int, protocol, status, search string, sortBy, sortOrder string) ([]ProxyWithAccountCount, int64, error)
	GetAllProxies(ctx context.Context) ([]Proxy, error)
	GetAllProxiesWithAccountCount(ctx context.Context) ([]ProxyWithAccountCount, error)
	GetProxy(ctx context.Context, id int64) (*Proxy, error)
	GetProxiesByIDs(ctx context.Context, ids []int64) ([]Proxy, error)
	CreateProxy(ctx context.Context, input *CreateProxyInput) (*Proxy, error)
	UpdateProxy(ctx context.Context, id int64, input *UpdateProxyInput) (*Proxy, error)
	DeleteProxy(ctx context.Context, id int64) error
	BatchDeleteProxies(ctx context.Context, ids []int64) (*ProxyBatchDeleteResult, error)
	GetProxyAccounts(ctx context.Context, proxyID int64) ([]ProxyAccountSummary, error)
	CheckProxyExists(ctx context.Context, host string, port int, username, password string) (bool, error)
	TestProxy(ctx context.Context, id int64) (*ProxyTestResult, error)
	CheckProxyQuality(ctx context.Context, id int64) (*ProxyQualityCheckResult, error)
	ListProxyRelationships(ctx context.Context, page, pageSize int, platform, status, search string) ([]ProxyRelationship, int64, error)
	ReassignAccountProxy(ctx context.Context, accountID int64) (*ProxyRelationship, error)
	RestoreAccountProxyHistory(ctx context.Context, accountID int64) (*ProxyRelationship, error)
	ReportAccountProxyFailure(ctx context.Context, accountID int64, reason string) (*ProxyRelationship, error)
	RecordAccountProxySuccess(ctx context.Context, accountID int64) error
	GetAccountProxyHistory(ctx context.Context, accountID int64) ([]AccountProxyBinding, error)
	GetProxyDispatchSettings(ctx context.Context) (*ProxyDispatchSettings, error)
	UpdateProxyDispatchSettings(ctx context.Context, input *ProxyDispatchSettings) (*ProxyDispatchSettings, error)
	GetAbuseIPDBAPIKeySettings(ctx context.Context) (*AbuseIPDBAPIKeySettings, error)
	UpdateAbuseIPDBAPIKeySettings(ctx context.Context, input *AbuseIPDBAPIKeySettingsInput) (*AbuseIPDBAPIKeySettings, error)
	PreviewProxyImport(ctx context.Context, input ProxyImportPreviewInput) (*ProxyImportPreview, error)
	ConfirmProxyImport(ctx context.Context, input ProxyImportConfirmInput) (*ProxyImportConfirmResult, error)
	BatchHealthCheckProxies(ctx context.Context, ids []int64) ([]ProxyTestResult, error)
	ListProxySubscriptionSources(ctx context.Context) ([]ProxySubscriptionSource, error)
	CreateProxySubscriptionSource(ctx context.Context, input ProxySubscriptionSourceInput) (*ProxySubscriptionSource, error)
	UpdateProxySubscriptionSource(ctx context.Context, id int64, input ProxySubscriptionSourceInput) (*ProxySubscriptionSource, error)
	DeleteProxySubscriptionSource(ctx context.Context, id int64) error
	SyncProxySubscriptionSource(ctx context.Context, id int64) (*ProxyImportPreview, error)
	ScanProxySubscriptionSource(ctx context.Context, id int64) (*ProxySubscriptionScanResult, error)
	GetProxySubscriptionScanStatus(ctx context.Context) (*ProxySubscriptionScanStatus, error)
	ListProxySubscriptionNodes(ctx context.Context, sourceID int64) ([]ProxySubscriptionNode, error)

	// Redeem code management
	ListRedeemCodes(ctx context.Context, page, pageSize int, codeType, status, search string, sortBy, sortOrder string) ([]RedeemCode, int64, error)
	GetRedeemCode(ctx context.Context, id int64) (*RedeemCode, error)
	GenerateRedeemCodes(ctx context.Context, input *GenerateRedeemCodesInput) ([]RedeemCode, error)
	DeleteRedeemCode(ctx context.Context, id int64) error
	BatchDeleteRedeemCodes(ctx context.Context, ids []int64) (int64, error)
	ExpireRedeemCode(ctx context.Context, id int64) (*RedeemCode, error)
	ResetAccountQuota(ctx context.Context, id int64) error
}

// CreateUserInput represents input for creating a new user via admin operations.
type CreateUserInput struct {
	Email         string
	Password      string
	Username      string
	Notes         string
	Balance       float64
	Concurrency   int
	RPMLimit      int
	AllowedGroups []int64
}

type UpdateUserInput struct {
	Email         string
	Password      string
	Username      *string
	Notes         *string
	Balance       *float64 // 使用指针区分"未提供"和"设置为0"
	Concurrency   *int     // 使用指针区分"未提供"和"设置为0"
	RPMLimit      *int     // 使用指针区分"未提供"和"设置为0"
	Status        string
	AllowedGroups *[]int64 // 使用指针区分"未提供"和"设置为空数组"
	// GroupRates 用户专属分组倍率配置
	// map[groupID]*rate，nil 表示删除该分组的专属倍率
	GroupRates map[int64]*float64
}

type AdminBindAuthIdentityInput struct {
	ProviderType    string
	ProviderKey     string
	ProviderSubject string
	Issuer          *string
	Metadata        map[string]any
	Channel         *AdminBindAuthIdentityChannelInput
}

type AdminBindAuthIdentityChannelInput struct {
	Channel        string
	ChannelAppID   string
	ChannelSubject string
	Metadata       map[string]any
}

type AdminBoundAuthIdentity struct {
	UserID          int64                          `json:"user_id"`
	ProviderType    string                         `json:"provider_type"`
	ProviderKey     string                         `json:"provider_key"`
	ProviderSubject string                         `json:"provider_subject"`
	VerifiedAt      *time.Time                     `json:"verified_at,omitempty"`
	Issuer          *string                        `json:"issuer,omitempty"`
	Metadata        map[string]any                 `json:"metadata"`
	CreatedAt       time.Time                      `json:"created_at"`
	UpdatedAt       time.Time                      `json:"updated_at"`
	Channel         *AdminBoundAuthIdentityChannel `json:"channel,omitempty"`
}

type AdminBoundAuthIdentityChannel struct {
	Channel        string         `json:"channel"`
	ChannelAppID   string         `json:"channel_app_id"`
	ChannelSubject string         `json:"channel_subject"`
	Metadata       map[string]any `json:"metadata"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type CreateGroupInput struct {
	Name             string
	Description      string
	Platform         string
	RateMultiplier   float64
	IsExclusive      bool
	SubscriptionType string   // standard/subscription
	DailyLimitUSD    *float64 // 日限额 (USD)
	WeeklyLimitUSD   *float64 // 周限额 (USD)
	MonthlyLimitUSD  *float64 // 月限额 (USD)
	// 图片生成计费配置（仅 antigravity 平台使用）
	AllowImageGeneration bool
	ImageRateIndependent bool
	ImageRateMultiplier  *float64
	ImagePrice1K         *float64
	ImagePrice2K         *float64
	ImagePrice4K         *float64
	ClaudeCodeOnly       bool   // 仅允许 Claude Code 客户端
	FallbackGroupID      *int64 // 降级分组 ID
	// 无效请求兜底分组 ID（仅 anthropic 平台使用）
	FallbackGroupIDOnInvalidRequest *int64
	// 模型路由配置（仅 anthropic 平台使用）
	ModelRouting        map[string][]int64
	ModelRoutingEnabled bool // 是否启用模型路由
	MCPXMLInject        *bool
	// 支持的模型系列（仅 antigravity 平台使用）
	SupportedModelScopes []string
	// OpenAI Messages 调度配置（仅 openai 平台使用）
	AllowMessagesDispatch       bool
	DefaultMappedModel          string
	RequireOAuthOnly            bool
	RequirePrivacySet           bool
	MessagesDispatchModelConfig OpenAIMessagesDispatchModelConfig
	// RPMLimit 分组 RPM 上限（0 = 不限制）
	RPMLimit int
	// 从指定分组复制账号（创建分组后在同一事务内绑定）
	CopyAccountsFromGroupIDs []int64
}

type UpdateGroupInput struct {
	Name             string
	Description      string
	Platform         string
	RateMultiplier   *float64 // 使用指针以支持设置为0
	IsExclusive      *bool
	Status           string
	SubscriptionType string   // standard/subscription
	DailyLimitUSD    *float64 // 日限额 (USD)
	WeeklyLimitUSD   *float64 // 周限额 (USD)
	MonthlyLimitUSD  *float64 // 月限额 (USD)
	// 图片生成计费配置（仅 antigravity 平台使用）
	AllowImageGeneration *bool
	ImageRateIndependent *bool
	ImageRateMultiplier  *float64
	ImagePrice1K         *float64
	ImagePrice2K         *float64
	ImagePrice4K         *float64
	ClaudeCodeOnly       *bool  // 仅允许 Claude Code 客户端
	FallbackGroupID      *int64 // 降级分组 ID
	// 无效请求兜底分组 ID（仅 anthropic 平台使用）
	FallbackGroupIDOnInvalidRequest *int64
	// 模型路由配置（仅 anthropic 平台使用）
	ModelRouting        map[string][]int64
	ModelRoutingEnabled *bool // 是否启用模型路由
	MCPXMLInject        *bool
	// 支持的模型系列（仅 antigravity 平台使用）
	SupportedModelScopes *[]string
	// OpenAI Messages 调度配置（仅 openai 平台使用）
	AllowMessagesDispatch       *bool
	DefaultMappedModel          *string
	RequireOAuthOnly            *bool
	RequirePrivacySet           *bool
	MessagesDispatchModelConfig *OpenAIMessagesDispatchModelConfig
	// RPMLimit 分组 RPM 上限（0 = 不限制），nil 表示未提供不改动。
	RPMLimit *int
	// 从指定分组复制账号（同步操作：先清空当前分组的账号绑定，再绑定源分组的账号）
	CopyAccountsFromGroupIDs []int64
}

type CreateAccountInput struct {
	Name               string
	Notes              *string
	Platform           string
	Type               string
	Credentials        map[string]any
	Extra              map[string]any
	ProxyID            *int64
	Concurrency        int
	Priority           int
	RateMultiplier     *float64 // 账号计费倍率（>=0，允许 0）
	LoadFactor         *int
	GroupIDs           []int64
	ExpiresAt          *int64
	AutoPauseOnExpired *bool
	// SkipDefaultGroupBind prevents auto-binding to platform default group when GroupIDs is empty.
	SkipDefaultGroupBind bool
	// SkipMixedChannelCheck skips the mixed channel risk check when binding groups.
	// This should only be set when the caller has explicitly confirmed the risk.
	SkipMixedChannelCheck bool
}

type UpdateAccountInput struct {
	Name                  string
	Notes                 *string
	Type                  string // Account type: oauth, setup-token, apikey
	Credentials           map[string]any
	Extra                 map[string]any
	ProxyID               *int64
	Concurrency           *int     // 使用指针区分"未提供"和"设置为0"
	Priority              *int     // 使用指针区分"未提供"和"设置为0"
	RateMultiplier        *float64 // 账号计费倍率（>=0，允许 0）
	LoadFactor            *int
	Status                string
	GroupIDs              *[]int64
	ExpiresAt             *int64
	AutoPauseOnExpired    *bool
	SkipMixedChannelCheck bool // 跳过混合渠道检查（用户已确认风险）
}

// BulkUpdateAccountsInput describes the payload for bulk updating accounts.
type BulkUpdateAccountsInput struct {
	AccountIDs     []int64
	Filters        *BulkUpdateAccountFilters
	Name           string
	ProxyID        *int64
	Concurrency    *int
	Priority       *int
	RateMultiplier *float64 // 账号计费倍率（>=0，允许 0）
	LoadFactor     *int
	Status         string
	Schedulable    *bool
	GroupIDs       *[]int64
	Credentials    map[string]any
	Extra          map[string]any
	// SkipMixedChannelCheck skips the mixed channel risk check when binding groups.
	// This should only be set when the caller has explicitly confirmed the risk.
	SkipMixedChannelCheck bool
}

type BulkUpdateAccountFilters struct {
	Platform    string
	Type        string
	Status      string
	Group       string
	Search      string
	PrivacyMode string
}

// BulkUpdateAccountResult captures the result for a single account update.
type BulkUpdateAccountResult struct {
	AccountID int64  `json:"account_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// AdminUpdateAPIKeyGroupIDResult is the result of AdminUpdateAPIKeyGroupID.
type AdminUpdateAPIKeyGroupIDResult struct {
	APIKey                 *APIKey
	AutoGrantedGroupAccess bool   // true if a new exclusive group permission was auto-added
	GrantedGroupID         *int64 // the group ID that was auto-granted
	GrantedGroupName       string // the group name that was auto-granted
}

// ReplaceUserGroupResult 分组替换操作的结果
type ReplaceUserGroupResult struct {
	MigratedKeys int64 // 迁移的 Key 数量
}

// UserRPMStatus describes a user's current per-minute RPM usage.
type UserRPMStatus struct {
	UserRPMUsed  int                  `json:"user_rpm_used"`
	UserRPMLimit int                  `json:"user_rpm_limit"`
	PerGroup     []UserGroupRPMStatus `json:"per_group"`
}

// UserGroupRPMStatus describes current per-minute RPM usage for one user/group pair.
type UserGroupRPMStatus struct {
	GroupID   int64  `json:"group_id"`
	GroupName string `json:"group_name"`
	Used      int    `json:"used"`
	Limit     int    `json:"limit"`
	Source    string `json:"source"` // "group" | "override"
}

// BulkUpdateAccountsResult is the aggregated response for bulk updates.
type BulkUpdateAccountsResult struct {
	Success    int                       `json:"success"`
	Failed     int                       `json:"failed"`
	SuccessIDs []int64                   `json:"success_ids"`
	FailedIDs  []int64                   `json:"failed_ids"`
	Results    []BulkUpdateAccountResult `json:"results"`
}

type CreateProxyInput struct {
	Name              string
	Protocol          string
	Host              string
	Port              int
	Username          string
	Password          string
	Source            string
	ProxyType         string
	Provider          string
	Region            string
	ExitIP            string
	QualityStatus     string
	MaxBoundAccounts  *int
	MaxActiveAccounts *int
	Weight            int
}

type UpdateProxyInput struct {
	Name              string
	Protocol          string
	Host              string
	Port              int
	Username          string
	Password          string
	Status            string
	Source            string
	ProxyType         string
	Provider          string
	Region            string
	ExitIP            string
	QualityStatus     string
	MaxBoundAccounts  *int
	MaxActiveAccounts *int
	Weight            *int
}

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

type GenerateRedeemCodesInput struct {
	Count        int
	Type         string
	Value        float64
	GroupID      *int64 // 订阅类型专用：关联的分组ID
	ValidityDays int    // 订阅类型专用：有效天数
}

type ProxyBatchDeleteResult struct {
	DeletedIDs []int64                   `json:"deleted_ids"`
	Skipped    []ProxyBatchDeleteSkipped `json:"skipped"`
}

type ProxyBatchDeleteSkipped struct {
	ID     int64  `json:"id"`
	Reason string `json:"reason"`
}

// ProxyTestResult represents the result of testing a proxy
type ProxyTestResult struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	LatencyMs   int64  `json:"latency_ms,omitempty"`
	IPAddress   string `json:"ip_address,omitempty"`
	City        string `json:"city,omitempty"`
	Region      string `json:"region,omitempty"`
	Country     string `json:"country,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
}

type ProxyQualityCheckResult struct {
	ProxyID        int64                   `json:"proxy_id"`
	Score          int                     `json:"score"`
	Grade          string                  `json:"grade"`
	Summary        string                  `json:"summary"`
	ExitIP         string                  `json:"exit_ip,omitempty"`
	Country        string                  `json:"country,omitempty"`
	CountryCode    string                  `json:"country_code,omitempty"`
	BaseLatencyMs  int64                   `json:"base_latency_ms,omitempty"`
	PassedCount    int                     `json:"passed_count"`
	WarnCount      int                     `json:"warn_count"`
	FailedCount    int                     `json:"failed_count"`
	ChallengeCount int                     `json:"challenge_count"`
	CheckedAt      int64                   `json:"checked_at"`
	Items          []ProxyQualityCheckItem `json:"items"`
}

type ProxyQualityCheckItem struct {
	Target     string `json:"target"`
	Status     string `json:"status"` // pass/warn/fail/challenge
	HTTPStatus int    `json:"http_status,omitempty"`
	LatencyMs  int64  `json:"latency_ms,omitempty"`
	Message    string `json:"message,omitempty"`
	CFRay      string `json:"cf_ray,omitempty"`
}

// ProxyExitInfo represents proxy exit information from ip-api.com
type ProxyExitInfo struct {
	IP          string
	City        string
	Region      string
	Country     string
	CountryCode string
}

// ProxyExitInfoProber tests proxy connectivity and retrieves exit information
type ProxyExitInfoProber interface {
	ProbeProxy(ctx context.Context, proxyURL string) (*ProxyExitInfo, int64, error)
}

type groupExistenceBatchReader interface {
	ExistsByIDs(ctx context.Context, ids []int64) (map[int64]bool, error)
}

type proxyQualityTarget struct {
	Target          string
	URL             string
	Method          string
	AllowedStatuses map[int]struct{}
}

var proxyQualityTargets = []proxyQualityTarget{
	{
		Target: "openai",
		URL:    "https://api.openai.com/v1/models",
		Method: http.MethodGet,
		AllowedStatuses: map[int]struct{}{
			http.StatusUnauthorized: {},
		},
	},
	{
		Target: "anthropic",
		URL:    "https://api.anthropic.com/v1/messages",
		Method: http.MethodGet,
		AllowedStatuses: map[int]struct{}{
			http.StatusUnauthorized:     {},
			http.StatusMethodNotAllowed: {},
			http.StatusNotFound:         {},
			http.StatusBadRequest:       {},
		},
	},
	{
		Target: "gemini",
		URL:    "https://generativelanguage.googleapis.com/$discovery/rest?version=v1beta",
		Method: http.MethodGet,
		AllowedStatuses: map[int]struct{}{
			http.StatusOK: {},
		},
	},
}

const (
	accountProxyFailureReassignThreshold = 2
	proxyQualityRequestTimeout           = 15 * time.Second
	proxyQualityResponseHeaderTimeout    = 10 * time.Second
	proxyQualityMaxBodyBytes             = int64(8 * 1024)
	proxyQualityClientUserAgent          = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"
)

var ErrRPMStatusUnavailable = infraerrors.New(http.StatusNotImplemented, "RPM_STATUS_UNAVAILABLE", "RPM cache not available")

// adminServiceImpl implements AdminService
type adminServiceImpl struct {
	userRepo             UserRepository
	groupRepo            GroupRepository
	accountRepo          AccountRepository
	proxyRepo            ProxyRepository
	apiKeyRepo           APIKeyRepository
	redeemCodeRepo       RedeemCodeRepository
	userGroupRateRepo    UserGroupRateRepository
	userRPMCache         UserRPMCache
	billingCacheService  *BillingCacheService
	proxyProber          ProxyExitInfoProber
	proxyLatencyCache    ProxyLatencyCache
	authCacheInvalidator APIKeyAuthCacheInvalidator
	entClient            *dbent.Client // 用于开启数据库事务
	settingService       *SettingService
	defaultSubAssigner   DefaultSubscriptionAssigner
	userSubRepo          UserSubscriptionRepository
	privacyClientFactory PrivacyClientFactory
	scanStateMu          sync.Mutex
	scanActive           bool
	scanActiveSourceID   int64
	scanStartedAt        time.Time
}

type userGroupRateBatchReader interface {
	GetByUserIDs(ctx context.Context, userIDs []int64) (map[int64]map[int64]float64, error)
}

// NewAdminService creates a new AdminService
func NewAdminService(
	userRepo UserRepository,
	groupRepo GroupRepository,
	accountRepo AccountRepository,
	proxyRepo ProxyRepository,
	apiKeyRepo APIKeyRepository,
	redeemCodeRepo RedeemCodeRepository,
	userGroupRateRepo UserGroupRateRepository,
	userRPMCache UserRPMCache,
	billingCacheService *BillingCacheService,
	proxyProber ProxyExitInfoProber,
	proxyLatencyCache ProxyLatencyCache,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	entClient *dbent.Client,
	settingService *SettingService,
	defaultSubAssigner DefaultSubscriptionAssigner,
	userSubRepo UserSubscriptionRepository,
	privacyClientFactory PrivacyClientFactory,
) AdminService {
	return &adminServiceImpl{
		userRepo:             userRepo,
		groupRepo:            groupRepo,
		accountRepo:          accountRepo,
		proxyRepo:            proxyRepo,
		apiKeyRepo:           apiKeyRepo,
		redeemCodeRepo:       redeemCodeRepo,
		userGroupRateRepo:    userGroupRateRepo,
		userRPMCache:         userRPMCache,
		billingCacheService:  billingCacheService,
		proxyProber:          proxyProber,
		proxyLatencyCache:    proxyLatencyCache,
		authCacheInvalidator: authCacheInvalidator,
		entClient:            entClient,
		settingService:       settingService,
		defaultSubAssigner:   defaultSubAssigner,
		userSubRepo:          userSubRepo,
		privacyClientFactory: privacyClientFactory,
	}
}

// User management implementations
func (s *adminServiceImpl) ListUsers(ctx context.Context, page, pageSize int, filters UserListFilters, sortBy, sortOrder string) ([]User, int64, error) {
	params := pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: sortBy, SortOrder: sortOrder}
	users, result, err := s.userRepo.ListWithFilters(ctx, params, filters)
	if err != nil {
		return nil, 0, err
	}
	if len(users) > 0 {
		userIDs := make([]int64, 0, len(users))
		for i := range users {
			userIDs = append(userIDs, users[i].ID)
		}
		lastUsedByUserID, latestErr := s.userRepo.GetLatestUsedAtByUserIDs(ctx, userIDs)
		if latestErr != nil {
			logger.LegacyPrintf("service.admin", "failed to load user last_used_at in batch: err=%v", latestErr)
		} else {
			for i := range users {
				users[i].LastUsedAt = lastUsedByUserID[users[i].ID]
			}
		}
	}
	// 批量加载用户专属分组倍率
	if s.userGroupRateRepo != nil && len(users) > 0 {
		if batchRepo, ok := s.userGroupRateRepo.(userGroupRateBatchReader); ok {
			userIDs := make([]int64, 0, len(users))
			for i := range users {
				userIDs = append(userIDs, users[i].ID)
			}
			ratesByUser, err := batchRepo.GetByUserIDs(ctx, userIDs)
			if err != nil {
				logger.LegacyPrintf("service.admin", "failed to load user group rates in batch: err=%v", err)
				s.loadUserGroupRatesOneByOne(ctx, users)
			} else {
				for i := range users {
					if rates, ok := ratesByUser[users[i].ID]; ok {
						users[i].GroupRates = rates
					}
				}
			}
		} else {
			s.loadUserGroupRatesOneByOne(ctx, users)
		}
	}
	return users, result.Total, nil
}

func (s *adminServiceImpl) loadUserGroupRatesOneByOne(ctx context.Context, users []User) {
	if s.userGroupRateRepo == nil {
		return
	}
	for i := range users {
		rates, err := s.userGroupRateRepo.GetByUserID(ctx, users[i].ID)
		if err != nil {
			logger.LegacyPrintf("service.admin", "failed to load user group rates: user_id=%d err=%v", users[i].ID, err)
			continue
		}
		users[i].GroupRates = rates
	}
}

func (s *adminServiceImpl) GetUser(ctx context.Context, id int64) (*User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	lastUsedAt, latestErr := s.userRepo.GetLatestUsedAtByUserID(ctx, id)
	if latestErr != nil {
		logger.LegacyPrintf("service.admin", "failed to load user last_used_at: user_id=%d err=%v", id, latestErr)
	} else {
		user.LastUsedAt = lastUsedAt
	}
	// 加载用户专属分组倍率
	if s.userGroupRateRepo != nil {
		rates, err := s.userGroupRateRepo.GetByUserID(ctx, id)
		if err != nil {
			logger.LegacyPrintf("service.admin", "failed to load user group rates: user_id=%d err=%v", id, err)
		} else {
			user.GroupRates = rates
		}
	}
	return user, nil
}

func (s *adminServiceImpl) CreateUser(ctx context.Context, input *CreateUserInput) (*User, error) {
	user := &User{
		Email:         input.Email,
		Username:      input.Username,
		Notes:         input.Notes,
		Role:          RoleUser, // Always create as regular user, never admin
		Balance:       input.Balance,
		Concurrency:   input.Concurrency,
		RPMLimit:      input.RPMLimit,
		Status:        StatusActive,
		AllowedGroups: input.AllowedGroups,
	}
	if err := user.SetPassword(input.Password); err != nil {
		return nil, err
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	s.assignDefaultSubscriptions(ctx, user.ID)
	return user, nil
}

func (s *adminServiceImpl) assignDefaultSubscriptions(ctx context.Context, userID int64) {
	if s.settingService == nil || s.defaultSubAssigner == nil || userID <= 0 {
		return
	}
	items := s.settingService.GetDefaultSubscriptions(ctx)
	for _, item := range items {
		if _, _, err := s.defaultSubAssigner.AssignOrExtendSubscription(ctx, &AssignSubscriptionInput{
			UserID:       userID,
			GroupID:      item.GroupID,
			ValidityDays: item.ValidityDays,
			Notes:        "auto assigned by default user subscriptions setting",
		}); err != nil {
			logger.LegacyPrintf("service.admin", "failed to assign default subscription: user_id=%d group_id=%d err=%v", userID, item.GroupID, err)
		}
	}
}

func (s *adminServiceImpl) UpdateUser(ctx context.Context, id int64, input *UpdateUserInput) (*User, error) {
	// 校验用户专属分组倍率：必须 > 0（nil 合法，表示清除专属倍率）
	if input.GroupRates != nil {
		for groupID, rate := range input.GroupRates {
			if rate != nil && *rate <= 0 {
				return nil, fmt.Errorf("rate_multiplier must be > 0 (group_id=%d)", groupID)
			}
		}
	}

	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Protect admin users: cannot disable admin accounts
	if user.Role == "admin" && input.Status == "disabled" {
		return nil, errors.New("cannot disable admin user")
	}

	oldConcurrency := user.Concurrency
	oldStatus := user.Status
	oldRole := user.Role
	oldRPMLimit := user.RPMLimit

	if input.Email != "" {
		user.Email = input.Email
	}
	if input.Password != "" {
		if err := user.SetPassword(input.Password); err != nil {
			return nil, err
		}
	}

	if input.Username != nil {
		user.Username = *input.Username
	}
	if input.Notes != nil {
		user.Notes = *input.Notes
	}

	if input.Status != "" {
		user.Status = input.Status
	}

	if input.Concurrency != nil {
		user.Concurrency = *input.Concurrency
	}

	if input.RPMLimit != nil {
		user.RPMLimit = *input.RPMLimit
	}

	if input.AllowedGroups != nil {
		user.AllowedGroups = *input.AllowedGroups
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	// 同步用户专属分组倍率
	if input.GroupRates != nil && s.userGroupRateRepo != nil {
		if err := s.userGroupRateRepo.SyncUserGroupRates(ctx, user.ID, input.GroupRates); err != nil {
			logger.LegacyPrintf("service.admin", "failed to sync user group rates: user_id=%d err=%v", user.ID, err)
		}
	}

	if s.authCacheInvalidator != nil {
		// RPMLimit 直接参与 billing_cache_service.checkRPM 的三级级联，
		// 不失效缓存会让修改在一个 L2 TTL 内失去效果。
		if user.Concurrency != oldConcurrency || user.Status != oldStatus || user.Role != oldRole || user.RPMLimit != oldRPMLimit {
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, user.ID)
		}
	}

	concurrencyDiff := user.Concurrency - oldConcurrency
	if concurrencyDiff != 0 {
		code, err := GenerateRedeemCode()
		if err != nil {
			logger.LegacyPrintf("service.admin", "failed to generate adjustment redeem code: %v", err)
			return user, nil
		}
		adjustmentRecord := &RedeemCode{
			Code:   code,
			Type:   AdjustmentTypeAdminConcurrency,
			Value:  float64(concurrencyDiff),
			Status: StatusUsed,
			UsedBy: &user.ID,
		}
		now := time.Now()
		adjustmentRecord.UsedAt = &now
		if err := s.redeemCodeRepo.Create(ctx, adjustmentRecord); err != nil {
			logger.LegacyPrintf("service.admin", "failed to create concurrency adjustment redeem code: %v", err)
		}
	}

	return user, nil
}

func (s *adminServiceImpl) DeleteUser(ctx context.Context, id int64) error {
	// Protect admin users: cannot delete admin accounts
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if user.Role == "admin" {
		return errors.New("cannot delete admin user")
	}
	if err := s.userRepo.Delete(ctx, id); err != nil {
		logger.LegacyPrintf("service.admin", "delete user failed: user_id=%d err=%v", id, err)
		return err
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, id)
	}
	return nil
}

func (s *adminServiceImpl) BatchUpdateConcurrency(ctx context.Context, userIDs []int64, value int, mode string) (int, error) {
	cleaned := make([]int64, 0, len(userIDs))
	for _, uid := range userIDs {
		if uid > 0 {
			cleaned = append(cleaned, uid)
		}
	}
	if len(cleaned) == 0 {
		return 0, nil
	}

	var affected int
	var err error
	switch mode {
	case "set":
		affected, err = s.userRepo.BatchSetConcurrency(ctx, cleaned, value)
	case "add":
		affected, err = s.userRepo.BatchAddConcurrency(ctx, cleaned, value)
	default:
		return 0, errors.New("invalid mode: must be 'set' or 'add'")
	}
	if err != nil {
		return 0, err
	}

	if s.authCacheInvalidator != nil {
		for _, uid := range cleaned {
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, uid)
		}
	}
	return affected, nil
}

func (s *adminServiceImpl) UpdateUserBalance(ctx context.Context, userID int64, balance float64, operation string, notes string) (*User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	oldBalance := user.Balance

	switch operation {
	case "set":
		user.Balance = balance
	case "add":
		user.Balance += balance
	case "subtract":
		user.Balance -= balance
	}

	if user.Balance < 0 {
		return nil, fmt.Errorf("balance cannot be negative, current balance: %.2f, requested operation would result in: %.2f", oldBalance, user.Balance)
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}
	balanceDiff := user.Balance - oldBalance
	if s.authCacheInvalidator != nil && balanceDiff != 0 {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}

	if s.billingCacheService != nil {
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.billingCacheService.InvalidateUserBalance(cacheCtx, userID); err != nil {
				logger.LegacyPrintf("service.admin", "invalidate user balance cache failed: user_id=%d err=%v", userID, err)
			}
		}()
	}

	if balanceDiff != 0 {
		code, err := GenerateRedeemCode()
		if err != nil {
			logger.LegacyPrintf("service.admin", "failed to generate adjustment redeem code: %v", err)
			return user, nil
		}

		adjustmentRecord := &RedeemCode{
			Code:   code,
			Type:   AdjustmentTypeAdminBalance,
			Value:  balanceDiff,
			Status: StatusUsed,
			UsedBy: &user.ID,
			Notes:  notes,
		}
		now := time.Now()
		adjustmentRecord.UsedAt = &now

		if err := s.redeemCodeRepo.Create(ctx, adjustmentRecord); err != nil {
			logger.LegacyPrintf("service.admin", "failed to create balance adjustment redeem code: %v", err)
		}
	}

	return user, nil
}

func (s *adminServiceImpl) GetUserAPIKeys(ctx context.Context, userID int64, page, pageSize int, sortBy, sortOrder string) ([]APIKey, int64, error) {
	params := pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: sortBy, SortOrder: sortOrder}
	keys, result, err := s.apiKeyRepo.ListByUserID(ctx, userID, params, APIKeyListFilters{})
	if err != nil {
		return nil, 0, err
	}
	return keys, result.Total, nil
}

func (s *adminServiceImpl) GetUserRPMStatus(ctx context.Context, userID int64) (*UserRPMStatus, error) {
	if s.userRPMCache == nil {
		return nil, ErrRPMStatusUnavailable
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	userRPMUsed, err := s.userRPMCache.GetUserRPM(ctx, userID)
	if err != nil {
		logger.LegacyPrintf("service.admin", "failed to get user rpm: user_id=%d err=%v", userID, err)
	}

	keys, _, err := s.GetUserAPIKeys(ctx, userID, 1, 1000, "", "")
	if err != nil {
		return nil, err
	}

	groupIDSet := make(map[int64]struct{})
	for _, key := range keys {
		if key.GroupID != nil && *key.GroupID > 0 {
			groupIDSet[*key.GroupID] = struct{}{}
		}
	}

	groupIDs := make([]int64, 0, len(groupIDSet))
	for groupID := range groupIDSet {
		groupIDs = append(groupIDs, groupID)
	}
	sort.Slice(groupIDs, func(i, j int) bool { return groupIDs[i] < groupIDs[j] })

	var perGroup []UserGroupRPMStatus
	for _, groupID := range groupIDs {
		used, getErr := s.userRPMCache.GetUserGroupRPM(ctx, userID, groupID)
		if getErr != nil {
			logger.LegacyPrintf("service.admin", "failed to get user group rpm: user_id=%d group_id=%d err=%v", userID, groupID, getErr)
		}

		entry := UserGroupRPMStatus{
			GroupID: groupID,
			Used:    used,
		}

		if s.groupRepo != nil {
			if group, groupErr := s.groupRepo.GetByIDLite(ctx, groupID); groupErr == nil && group != nil {
				entry.GroupName = group.Name
				entry.Limit = group.RPMLimit
				entry.Source = "group"
			} else if groupErr != nil {
				logger.LegacyPrintf("service.admin", "failed to get group rpm status metadata: group_id=%d err=%v", groupID, groupErr)
			}
		}

		if s.userGroupRateRepo != nil {
			override, overrideErr := s.userGroupRateRepo.GetRPMOverrideByUserAndGroup(ctx, userID, groupID)
			if overrideErr != nil {
				logger.LegacyPrintf("service.admin", "failed to get rpm override: user_id=%d group_id=%d err=%v", userID, groupID, overrideErr)
			} else if override != nil {
				entry.Limit = *override
				entry.Source = "override"
			}
		}

		perGroup = append(perGroup, entry)
	}

	return &UserRPMStatus{
		UserRPMUsed:  userRPMUsed,
		UserRPMLimit: user.RPMLimit,
		PerGroup:     perGroup,
	}, nil
}

func (s *adminServiceImpl) GetUserUsageStats(ctx context.Context, userID int64, period string) (any, error) {
	// Return mock data for now
	return map[string]any{
		"period":          period,
		"total_requests":  0,
		"total_cost":      0.0,
		"total_tokens":    0,
		"avg_duration_ms": 0,
	}, nil
}

// GetUserBalanceHistory returns paginated balance/concurrency change records for a user.
func (s *adminServiceImpl) GetUserBalanceHistory(ctx context.Context, userID int64, page, pageSize int, codeType string) ([]RedeemCode, int64, float64, error) {
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}
	if codeType == RedeemTypeAffiliateBalance {
		codes, total, err := s.listAffiliateBalanceHistory(ctx, userID, params)
		if err != nil {
			return nil, 0, 0, err
		}
		totalRecharged, err := s.redeemCodeRepo.SumPositiveBalanceByUser(ctx, userID)
		if err != nil {
			return nil, 0, 0, err
		}
		return codes, total, totalRecharged, nil
	}

	if codeType == "" {
		return s.getAllUserBalanceHistory(ctx, userID, params)
	}

	codes, result, err := s.redeemCodeRepo.ListByUserPaginated(ctx, userID, params, codeType)
	if err != nil {
		return nil, 0, 0, err
	}
	total := result.Total
	// Aggregate total recharged amount (only once, regardless of type filter)
	totalRecharged, err := s.redeemCodeRepo.SumPositiveBalanceByUser(ctx, userID)
	if err != nil {
		return nil, 0, 0, err
	}
	return codes, total, totalRecharged, nil
}

func (s *adminServiceImpl) getAllUserBalanceHistory(ctx context.Context, userID int64, params pagination.PaginationParams) ([]RedeemCode, int64, float64, error) {
	needed := params.Offset() + params.Limit()
	if needed < params.Limit() {
		needed = params.Limit()
	}

	redeemCodes, redeemTotal, err := s.listRedeemBalanceHistoryForMerge(ctx, userID, needed)
	if err != nil {
		return nil, 0, 0, err
	}
	affiliateCodes, affiliateTotal, err := s.listAffiliateBalanceHistoryForMerge(ctx, userID, needed)
	if err != nil {
		return nil, 0, 0, err
	}
	codes := mergeBalanceHistoryCodes(redeemCodes, affiliateCodes, params)

	totalRecharged, err := s.redeemCodeRepo.SumPositiveBalanceByUser(ctx, userID)
	if err != nil {
		return nil, 0, 0, err
	}
	return codes, redeemTotal + affiliateTotal, totalRecharged, nil
}

func (s *adminServiceImpl) listRedeemBalanceHistoryForMerge(ctx context.Context, userID int64, needed int) ([]RedeemCode, int64, error) {
	if needed <= 0 {
		return nil, 0, nil
	}

	var (
		out   []RedeemCode
		total int64
	)
	for page := 1; len(out) < needed; page++ {
		params := pagination.PaginationParams{Page: page, PageSize: 1000}
		codes, result, err := s.redeemCodeRepo.ListByUserPaginated(ctx, userID, params, "")
		if err != nil {
			return nil, 0, err
		}
		if result != nil {
			total = result.Total
		}
		out = append(out, codes...)
		if len(codes) < params.Limit() || int64(len(out)) >= total {
			break
		}
	}
	if len(out) > needed {
		out = out[:needed]
	}
	return out, total, nil
}

func (s *adminServiceImpl) listAffiliateBalanceHistoryForMerge(ctx context.Context, userID int64, needed int) ([]RedeemCode, int64, error) {
	if needed <= 0 {
		return nil, 0, nil
	}

	var (
		out   []RedeemCode
		total int64
	)
	for page := 1; len(out) < needed; page++ {
		params := pagination.PaginationParams{Page: page, PageSize: 1000}
		codes, currentTotal, err := s.listAffiliateBalanceHistory(ctx, userID, params)
		if err != nil {
			return nil, 0, err
		}
		total = currentTotal
		out = append(out, codes...)
		if len(codes) < params.Limit() || int64(len(out)) >= total {
			break
		}
	}
	if len(out) > needed {
		out = out[:needed]
	}
	return out, total, nil
}

func (s *adminServiceImpl) listAffiliateBalanceHistory(ctx context.Context, userID int64, params pagination.PaginationParams) ([]RedeemCode, int64, error) {
	if s == nil || s.entClient == nil || userID <= 0 {
		return nil, 0, nil
	}

	rows, err := s.entClient.QueryContext(ctx, `
SELECT id,
       amount::double precision,
       created_at
FROM user_affiliate_ledger
WHERE user_id = $1
  AND action = 'transfer'
ORDER BY created_at DESC, id DESC
OFFSET $2
LIMIT $3`, userID, params.Offset(), params.Limit())
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	codes := make([]RedeemCode, 0, params.Limit())
	for rows.Next() {
		var id int64
		var amount float64
		var createdAt time.Time
		if err := rows.Scan(&id, &amount, &createdAt); err != nil {
			return nil, 0, err
		}
		usedBy := userID
		usedAt := createdAt
		codes = append(codes, RedeemCode{
			ID:        -id,
			Code:      fmt.Sprintf("AFF-%d", id),
			Type:      RedeemTypeAffiliateBalance,
			Value:     amount,
			Status:    StatusUsed,
			UsedBy:    &usedBy,
			UsedAt:    &usedAt,
			CreatedAt: createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	total, err := countAffiliateBalanceHistory(ctx, s.entClient, userID)
	if err != nil {
		return nil, 0, err
	}
	return codes, total, nil
}

func countAffiliateBalanceHistory(ctx context.Context, client *dbent.Client, userID int64) (int64, error) {
	rows, err := client.QueryContext(ctx, `
SELECT COUNT(*)
FROM user_affiliate_ledger
WHERE user_id = $1
  AND action = 'transfer'`, userID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()

	var total sql.NullInt64
	if rows.Next() {
		if err := rows.Scan(&total); err != nil {
			return 0, err
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if !total.Valid {
		return 0, nil
	}
	return total.Int64, nil
}

func mergeBalanceHistoryCodes(redeemCodes, affiliateCodes []RedeemCode, params pagination.PaginationParams) []RedeemCode {
	combined := append(append([]RedeemCode{}, redeemCodes...), affiliateCodes...)
	sort.SliceStable(combined, func(i, j int) bool {
		return redeemCodeHistoryTime(combined[i]).After(redeemCodeHistoryTime(combined[j]))
	})
	offset := params.Offset()
	if offset >= len(combined) {
		return []RedeemCode{}
	}
	end := offset + params.Limit()
	if end > len(combined) {
		end = len(combined)
	}
	return combined[offset:end]
}

func redeemCodeHistoryTime(code RedeemCode) time.Time {
	if code.UsedAt != nil {
		return *code.UsedAt
	}
	return code.CreatedAt
}

func (s *adminServiceImpl) BindUserAuthIdentity(ctx context.Context, userID int64, input AdminBindAuthIdentityInput) (*AdminBoundAuthIdentity, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "user_id must be greater than 0")
	}
	if s == nil || s.entClient == nil || s.userRepo == nil {
		return nil, infraerrors.InternalServer("ADMIN_AUTH_IDENTITY_BIND_UNAVAILABLE", "auth identity binding service is unavailable")
	}
	if _, err := s.userRepo.GetByID(ctx, userID); err != nil {
		return nil, err
	}

	providerType := normalizeAdminAuthIdentityProviderType(input.ProviderType)
	providerKey := strings.TrimSpace(input.ProviderKey)
	providerSubject := strings.TrimSpace(input.ProviderSubject)
	if providerType == "" {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "provider_type must be one of email, linuxdo, oidc, or wechat")
	}
	if providerKey == "" || providerSubject == "" {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "provider_type, provider_key, and provider_subject are required")
	}
	canonicalProviderKey := canonicalAdminAuthIdentityProviderKey(providerType, "", providerKey)
	compatibleProviderKeys := compatibleAdminAuthIdentityProviderKeys(providerType, providerKey)

	var issuer *string
	if input.Issuer != nil {
		trimmed := strings.TrimSpace(*input.Issuer)
		if trimmed != "" {
			issuer = &trimmed
		}
	}

	channelInput := normalizeAdminBindChannelInput(input.Channel)
	if input.Channel != nil && channelInput == nil {
		return nil, infraerrors.BadRequest("INVALID_INPUT", "channel, channel_app_id, and channel_subject are required when channel binding is provided")
	}

	verifiedAt := time.Now().UTC()
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, infraerrors.InternalServer("ADMIN_AUTH_IDENTITY_BIND_TX_FAILED", "failed to start auth identity bind transaction").WithCause(err)
	}
	defer func() { _ = tx.Rollback() }()

	identityRecords, err := tx.AuthIdentity.Query().
		Where(
			authidentity.ProviderTypeEQ(providerType),
			authidentity.ProviderKeyIn(compatibleProviderKeys...),
			authidentity.ProviderSubjectEQ(providerSubject),
		).
		All(ctx)
	if err != nil {
		return nil, infraerrors.InternalServer("ADMIN_AUTH_IDENTITY_BIND_LOOKUP_FAILED", "failed to inspect auth identity ownership").WithCause(err)
	}
	if hasAdminAuthIdentityOwnershipConflict(identityRecords, userID) {
		return nil, infraerrors.Conflict("AUTH_IDENTITY_OWNERSHIP_CONFLICT", "auth identity already belongs to another user")
	}
	identity := selectOwnedAdminAuthIdentity(identityRecords, userID)

	if identity == nil {
		create := tx.AuthIdentity.Create().
			SetUserID(userID).
			SetProviderType(providerType).
			SetProviderKey(canonicalProviderKey).
			SetProviderSubject(providerSubject).
			SetVerifiedAt(verifiedAt)
		if issuer != nil {
			create = create.SetIssuer(*issuer)
		}
		if input.Metadata != nil {
			create = create.SetMetadata(cloneAdminAuthIdentityMetadata(input.Metadata))
		}
		identity, err = create.Save(ctx)
		if err != nil {
			return nil, infraerrors.InternalServer("ADMIN_AUTH_IDENTITY_BIND_SAVE_FAILED", "failed to save auth identity").WithCause(err)
		}
	} else {
		update := tx.AuthIdentity.UpdateOneID(identity.ID).
			SetVerifiedAt(verifiedAt).
			SetProviderKey(canonicalProviderKey)
		if issuer != nil {
			update = update.SetIssuer(*issuer)
		}
		if input.Metadata != nil {
			update = update.SetMetadata(cloneAdminAuthIdentityMetadata(input.Metadata))
		}
		identity, err = update.Save(ctx)
		if err != nil {
			return nil, infraerrors.InternalServer("ADMIN_AUTH_IDENTITY_BIND_SAVE_FAILED", "failed to save auth identity").WithCause(err)
		}
	}

	var channel *dbent.AuthIdentityChannel
	if channelInput != nil {
		channelRecords, err := tx.AuthIdentityChannel.Query().
			Where(
				authidentitychannel.ProviderTypeEQ(providerType),
				authidentitychannel.ProviderKeyIn(compatibleProviderKeys...),
				authidentitychannel.ChannelEQ(channelInput.Channel),
				authidentitychannel.ChannelAppIDEQ(channelInput.ChannelAppID),
				authidentitychannel.ChannelSubjectEQ(channelInput.ChannelSubject),
			).
			WithIdentity().
			All(ctx)
		if err != nil {
			return nil, infraerrors.InternalServer("ADMIN_AUTH_IDENTITY_CHANNEL_LOOKUP_FAILED", "failed to inspect auth identity channel ownership").WithCause(err)
		}
		if hasAdminAuthIdentityChannelOwnershipConflict(channelRecords, userID) {
			return nil, infraerrors.Conflict("AUTH_IDENTITY_CHANNEL_OWNERSHIP_CONFLICT", "auth identity channel already belongs to another user")
		}
		channel = selectOwnedAdminAuthIdentityChannel(channelRecords, userID)
		if channel == nil {
			create := tx.AuthIdentityChannel.Create().
				SetIdentityID(identity.ID).
				SetProviderType(providerType).
				SetProviderKey(canonicalProviderKey).
				SetChannel(channelInput.Channel).
				SetChannelAppID(channelInput.ChannelAppID).
				SetChannelSubject(channelInput.ChannelSubject)
			if channelInput.Metadata != nil {
				create = create.SetMetadata(cloneAdminAuthIdentityMetadata(channelInput.Metadata))
			}
			channel, err = create.Save(ctx)
			if err != nil {
				return nil, infraerrors.InternalServer("ADMIN_AUTH_IDENTITY_CHANNEL_SAVE_FAILED", "failed to save auth identity channel").WithCause(err)
			}
		} else {
			update := tx.AuthIdentityChannel.UpdateOneID(channel.ID).
				SetIdentityID(identity.ID).
				SetProviderKey(canonicalProviderKey)
			if channelInput.Metadata != nil {
				update = update.SetMetadata(cloneAdminAuthIdentityMetadata(channelInput.Metadata))
			}
			channel, err = update.Save(ctx)
			if err != nil {
				return nil, infraerrors.InternalServer("ADMIN_AUTH_IDENTITY_CHANNEL_SAVE_FAILED", "failed to save auth identity channel").WithCause(err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, infraerrors.InternalServer("ADMIN_AUTH_IDENTITY_BIND_COMMIT_FAILED", "failed to commit auth identity bind").WithCause(err)
	}
	return buildAdminBoundAuthIdentity(identity, channel), nil
}

func compatibleAdminAuthIdentityProviderKeys(providerType, providerKey string) []string {
	providerType = strings.TrimSpace(strings.ToLower(providerType))
	providerKey = strings.TrimSpace(providerKey)
	if providerKey == "" {
		return []string{providerKey}
	}
	if providerType != "wechat" {
		return []string{providerKey}
	}

	keys := []string{providerKey}
	if !strings.EqualFold(providerKey, "wechat-main") {
		keys = append(keys, "wechat-main")
	}
	if !strings.EqualFold(providerKey, "wechat") {
		keys = append(keys, "wechat")
	}
	return keys
}

func canonicalAdminAuthIdentityProviderKey(providerType, existingKey, requestedKey string) string {
	providerType = strings.TrimSpace(strings.ToLower(providerType))
	existingKey = strings.TrimSpace(existingKey)
	requestedKey = strings.TrimSpace(requestedKey)
	if providerType != "wechat" {
		if requestedKey != "" {
			return requestedKey
		}
		return existingKey
	}
	if strings.EqualFold(existingKey, "wechat") || strings.EqualFold(existingKey, "wechat-main") || strings.EqualFold(requestedKey, "wechat-main") {
		return "wechat-main"
	}
	if requestedKey != "" {
		return requestedKey
	}
	return existingKey
}

func adminAuthIdentityProviderKeyRank(providerType, providerKey string) int {
	providerType = strings.TrimSpace(strings.ToLower(providerType))
	providerKey = strings.TrimSpace(providerKey)
	if providerType != "wechat" {
		return 0
	}
	switch {
	case strings.EqualFold(providerKey, "wechat-main"):
		return 0
	case strings.EqualFold(providerKey, "wechat"):
		return 2
	default:
		return 1
	}
}

func selectOwnedAdminAuthIdentity(records []*dbent.AuthIdentity, userID int64) *dbent.AuthIdentity {
	var selected *dbent.AuthIdentity
	for _, record := range records {
		if record.UserID != userID {
			continue
		}
		if selected == nil || adminAuthIdentityProviderKeyRank(record.ProviderType, record.ProviderKey) < adminAuthIdentityProviderKeyRank(selected.ProviderType, selected.ProviderKey) {
			selected = record
		}
	}
	return selected
}

func hasAdminAuthIdentityOwnershipConflict(records []*dbent.AuthIdentity, userID int64) bool {
	for _, record := range records {
		if record.UserID != userID {
			return true
		}
	}
	return false
}

func selectOwnedAdminAuthIdentityChannel(records []*dbent.AuthIdentityChannel, userID int64) *dbent.AuthIdentityChannel {
	var selected *dbent.AuthIdentityChannel
	for _, record := range records {
		if record.Edges.Identity == nil || record.Edges.Identity.UserID != userID {
			continue
		}
		if selected == nil || adminAuthIdentityProviderKeyRank(record.ProviderType, record.ProviderKey) < adminAuthIdentityProviderKeyRank(selected.ProviderType, selected.ProviderKey) {
			selected = record
		}
	}
	return selected
}

func hasAdminAuthIdentityChannelOwnershipConflict(records []*dbent.AuthIdentityChannel, userID int64) bool {
	for _, record := range records {
		if record.Edges.Identity != nil && record.Edges.Identity.UserID != userID {
			return true
		}
	}
	return false
}

func normalizeAdminBindChannelInput(input *AdminBindAuthIdentityChannelInput) *AdminBindAuthIdentityChannelInput {
	if input == nil {
		return nil
	}
	channel := &AdminBindAuthIdentityChannelInput{
		Channel:        strings.TrimSpace(input.Channel),
		ChannelAppID:   strings.TrimSpace(input.ChannelAppID),
		ChannelSubject: strings.TrimSpace(input.ChannelSubject),
		Metadata:       cloneAdminAuthIdentityMetadata(input.Metadata),
	}
	if channel.Channel == "" || channel.ChannelAppID == "" || channel.ChannelSubject == "" {
		return nil
	}
	return channel
}

func normalizeAdminAuthIdentityProviderType(input string) string {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "email":
		return "email"
	case "linuxdo":
		return "linuxdo"
	case "oidc":
		return "oidc"
	case "wechat":
		return "wechat"
	default:
		return ""
	}
}

func buildAdminBoundAuthIdentity(identity *dbent.AuthIdentity, channel *dbent.AuthIdentityChannel) *AdminBoundAuthIdentity {
	if identity == nil {
		return nil
	}
	result := &AdminBoundAuthIdentity{
		UserID:          identity.UserID,
		ProviderType:    strings.TrimSpace(identity.ProviderType),
		ProviderKey:     strings.TrimSpace(identity.ProviderKey),
		ProviderSubject: strings.TrimSpace(identity.ProviderSubject),
		VerifiedAt:      identity.VerifiedAt,
		Issuer:          identity.Issuer,
		Metadata:        cloneAdminAuthIdentityMetadata(identity.Metadata),
		CreatedAt:       identity.CreatedAt,
		UpdatedAt:       identity.UpdatedAt,
	}
	if channel != nil {
		result.Channel = &AdminBoundAuthIdentityChannel{
			Channel:        strings.TrimSpace(channel.Channel),
			ChannelAppID:   strings.TrimSpace(channel.ChannelAppID),
			ChannelSubject: strings.TrimSpace(channel.ChannelSubject),
			Metadata:       cloneAdminAuthIdentityMetadata(channel.Metadata),
			CreatedAt:      channel.CreatedAt,
			UpdatedAt:      channel.UpdatedAt,
		}
	}
	return result
}

func cloneAdminAuthIdentityMetadata(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	if len(input) == 0 {
		return map[string]any{}
	}
	data, err := json.Marshal(input)
	if err != nil {
		out := make(map[string]any, len(input))
		for key, value := range input {
			out[key] = value
		}
		return out
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		out = make(map[string]any, len(input))
		for key, value := range input {
			out[key] = value
		}
	}
	return out
}

// Group management implementations
func (s *adminServiceImpl) ListGroups(ctx context.Context, page, pageSize int, platform, status, search string, isExclusive *bool, sortBy, sortOrder string) ([]Group, int64, error) {
	params := pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: sortBy, SortOrder: sortOrder}
	groups, result, err := s.groupRepo.ListWithFilters(ctx, params, platform, status, search, isExclusive)
	if err != nil {
		return nil, 0, err
	}
	return groups, result.Total, nil
}

func (s *adminServiceImpl) GetAllGroups(ctx context.Context) ([]Group, error) {
	return s.groupRepo.ListActive(ctx)
}

func (s *adminServiceImpl) GetAllGroupsByPlatform(ctx context.Context, platform string) ([]Group, error) {
	return s.groupRepo.ListActiveByPlatform(ctx, platform)
}

func (s *adminServiceImpl) GetGroup(ctx context.Context, id int64) (*Group, error) {
	return s.groupRepo.GetByID(ctx, id)
}

func (s *adminServiceImpl) CreateGroup(ctx context.Context, input *CreateGroupInput) (*Group, error) {
	if input.RateMultiplier <= 0 {
		return nil, errors.New("rate_multiplier must be > 0")
	}

	platform := input.Platform
	if platform == "" {
		platform = PlatformAnthropic
	}

	subscriptionType := input.SubscriptionType
	if subscriptionType == "" {
		subscriptionType = SubscriptionTypeStandard
	}

	// 限额字段：nil/负数 表示"无限制"，0 表示"不允许用量"，正数表示具体限额
	dailyLimit := normalizeLimit(input.DailyLimitUSD)
	weeklyLimit := normalizeLimit(input.WeeklyLimitUSD)
	monthlyLimit := normalizeLimit(input.MonthlyLimitUSD)

	// 图片价格：负数表示清除（使用默认价格），0 保留（表示免费）
	imagePrice1K := normalizePrice(input.ImagePrice1K)
	imagePrice2K := normalizePrice(input.ImagePrice2K)
	imagePrice4K := normalizePrice(input.ImagePrice4K)
	imageRateMultiplier := 1.0
	if input.ImageRateMultiplier != nil {
		if *input.ImageRateMultiplier < 0 {
			return nil, errors.New("image_rate_multiplier must be >= 0")
		}
		imageRateMultiplier = *input.ImageRateMultiplier
	}

	// 校验降级分组
	if input.FallbackGroupID != nil {
		if err := s.validateFallbackGroup(ctx, 0, *input.FallbackGroupID); err != nil {
			return nil, err
		}
	}
	fallbackOnInvalidRequest := input.FallbackGroupIDOnInvalidRequest
	if fallbackOnInvalidRequest != nil && *fallbackOnInvalidRequest <= 0 {
		fallbackOnInvalidRequest = nil
	}
	// 校验无效请求兜底分组
	if fallbackOnInvalidRequest != nil {
		if err := s.validateFallbackGroupOnInvalidRequest(ctx, 0, platform, subscriptionType, *fallbackOnInvalidRequest); err != nil {
			return nil, err
		}
	}

	// MCPXMLInject：默认为 true，仅当显式传入 false 时关闭
	mcpXMLInject := true
	if input.MCPXMLInject != nil {
		mcpXMLInject = *input.MCPXMLInject
	}

	// 如果指定了复制账号的源分组，先获取账号 ID 列表
	var accountIDsToCopy []int64
	if len(input.CopyAccountsFromGroupIDs) > 0 {
		// 去重源分组 IDs
		seen := make(map[int64]struct{})
		uniqueSourceGroupIDs := make([]int64, 0, len(input.CopyAccountsFromGroupIDs))
		for _, srcGroupID := range input.CopyAccountsFromGroupIDs {
			if _, exists := seen[srcGroupID]; !exists {
				seen[srcGroupID] = struct{}{}
				uniqueSourceGroupIDs = append(uniqueSourceGroupIDs, srcGroupID)
			}
		}

		// 校验源分组的平台是否与新分组一致
		for _, srcGroupID := range uniqueSourceGroupIDs {
			srcGroup, err := s.groupRepo.GetByIDLite(ctx, srcGroupID)
			if err != nil {
				return nil, fmt.Errorf("source group %d not found: %w", srcGroupID, err)
			}
			if srcGroup.Platform != platform {
				return nil, fmt.Errorf("source group %d platform mismatch: expected %s, got %s", srcGroupID, platform, srcGroup.Platform)
			}
		}

		// 获取所有源分组的账号（去重）
		var err error
		accountIDsToCopy, err = s.groupRepo.GetAccountIDsByGroupIDs(ctx, uniqueSourceGroupIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to get accounts from source groups: %w", err)
		}
	}

	group := &Group{
		Name:                            input.Name,
		Description:                     input.Description,
		Platform:                        platform,
		RateMultiplier:                  input.RateMultiplier,
		IsExclusive:                     input.IsExclusive,
		Status:                          StatusActive,
		SubscriptionType:                subscriptionType,
		DailyLimitUSD:                   dailyLimit,
		WeeklyLimitUSD:                  weeklyLimit,
		MonthlyLimitUSD:                 monthlyLimit,
		AllowImageGeneration:            input.AllowImageGeneration,
		ImageRateIndependent:            input.ImageRateIndependent,
		ImageRateMultiplier:             imageRateMultiplier,
		ImagePrice1K:                    imagePrice1K,
		ImagePrice2K:                    imagePrice2K,
		ImagePrice4K:                    imagePrice4K,
		ClaudeCodeOnly:                  input.ClaudeCodeOnly,
		FallbackGroupID:                 input.FallbackGroupID,
		FallbackGroupIDOnInvalidRequest: fallbackOnInvalidRequest,
		ModelRouting:                    input.ModelRouting,
		MCPXMLInject:                    mcpXMLInject,
		SupportedModelScopes:            input.SupportedModelScopes,
		AllowMessagesDispatch:           input.AllowMessagesDispatch,
		RequireOAuthOnly:                input.RequireOAuthOnly,
		RequirePrivacySet:               input.RequirePrivacySet,
		DefaultMappedModel:              input.DefaultMappedModel,
		MessagesDispatchModelConfig:     normalizeOpenAIMessagesDispatchModelConfig(input.MessagesDispatchModelConfig),
		RPMLimit:                        input.RPMLimit,
	}
	sanitizeGroupMessagesDispatchFields(group)
	if err := s.groupRepo.Create(ctx, group); err != nil {
		return nil, err
	}

	// require_oauth_only: 过滤掉 apikey 类型账号
	if group.RequireOAuthOnly && (group.Platform == PlatformOpenAI || group.Platform == PlatformAntigravity || group.Platform == PlatformAnthropic || group.Platform == PlatformGemini) && len(accountIDsToCopy) > 0 {
		accounts, err := s.accountRepo.GetByIDs(ctx, accountIDsToCopy)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch accounts for oauth filter: %w", err)
		}
		oauthIDs := make(map[int64]struct{}, len(accounts))
		for _, acc := range accounts {
			if acc.Type != AccountTypeAPIKey {
				oauthIDs[acc.ID] = struct{}{}
			}
		}
		var filtered []int64
		for _, aid := range accountIDsToCopy {
			if _, ok := oauthIDs[aid]; ok {
				filtered = append(filtered, aid)
			}
		}
		accountIDsToCopy = filtered
	}

	// 如果有需要复制的账号，绑定到新分组
	if len(accountIDsToCopy) > 0 {
		if err := s.groupRepo.BindAccountsToGroup(ctx, group.ID, accountIDsToCopy); err != nil {
			return nil, fmt.Errorf("failed to bind accounts to new group: %w", err)
		}
		group.AccountCount = int64(len(accountIDsToCopy))
	}

	return group, nil
}

// normalizeLimit 将负数转换为 nil（表示无限制），0 保留（表示限额为零）
func normalizeLimit(limit *float64) *float64 {
	if limit == nil || *limit < 0 {
		return nil
	}
	return limit
}

// normalizePrice 将负数转换为 nil（表示使用默认价格），0 保留（表示免费）
func normalizePrice(price *float64) *float64 {
	if price == nil || *price < 0 {
		return nil
	}
	return price
}

// validateFallbackGroup 校验降级分组的有效性
// currentGroupID: 当前分组 ID（新建时为 0）
// fallbackGroupID: 降级分组 ID
func (s *adminServiceImpl) validateFallbackGroup(ctx context.Context, currentGroupID, fallbackGroupID int64) error {
	// 不能将自己设置为降级分组
	if currentGroupID > 0 && currentGroupID == fallbackGroupID {
		return fmt.Errorf("cannot set self as fallback group")
	}

	visited := map[int64]struct{}{}
	nextID := fallbackGroupID
	for {
		if _, seen := visited[nextID]; seen {
			return fmt.Errorf("fallback group cycle detected")
		}
		visited[nextID] = struct{}{}
		if currentGroupID > 0 && nextID == currentGroupID {
			return fmt.Errorf("fallback group cycle detected")
		}

		// 检查降级分组是否存在
		fallbackGroup, err := s.groupRepo.GetByIDLite(ctx, nextID)
		if err != nil {
			return fmt.Errorf("fallback group not found: %w", err)
		}

		// 降级分组不能启用 claude_code_only，否则会造成死循环
		if nextID == fallbackGroupID && fallbackGroup.ClaudeCodeOnly {
			return fmt.Errorf("fallback group cannot have claude_code_only enabled")
		}

		if fallbackGroup.FallbackGroupID == nil {
			return nil
		}
		nextID = *fallbackGroup.FallbackGroupID
	}
}

// validateFallbackGroupOnInvalidRequest 校验无效请求兜底分组的有效性
// currentGroupID: 当前分组 ID（新建时为 0）
// platform/subscriptionType: 当前分组的有效平台/订阅类型
// fallbackGroupID: 兜底分组 ID
func (s *adminServiceImpl) validateFallbackGroupOnInvalidRequest(ctx context.Context, currentGroupID int64, platform, subscriptionType string, fallbackGroupID int64) error {
	if platform != PlatformAnthropic && platform != PlatformAntigravity {
		return fmt.Errorf("invalid request fallback only supported for anthropic or antigravity groups")
	}
	if subscriptionType == SubscriptionTypeSubscription {
		return fmt.Errorf("subscription groups cannot set invalid request fallback")
	}
	if currentGroupID > 0 && currentGroupID == fallbackGroupID {
		return fmt.Errorf("cannot set self as invalid request fallback group")
	}

	fallbackGroup, err := s.groupRepo.GetByIDLite(ctx, fallbackGroupID)
	if err != nil {
		return fmt.Errorf("fallback group not found: %w", err)
	}
	if fallbackGroup.Platform != PlatformAnthropic {
		return fmt.Errorf("fallback group must be anthropic platform")
	}
	if fallbackGroup.SubscriptionType == SubscriptionTypeSubscription {
		return fmt.Errorf("fallback group cannot be subscription type")
	}
	if fallbackGroup.FallbackGroupIDOnInvalidRequest != nil {
		return fmt.Errorf("fallback group cannot have invalid request fallback configured")
	}
	return nil
}

func (s *adminServiceImpl) UpdateGroup(ctx context.Context, id int64, input *UpdateGroupInput) (*Group, error) {
	group, err := s.groupRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Name != "" {
		group.Name = input.Name
	}
	if input.Description != "" {
		group.Description = input.Description
	}
	if input.Platform != "" {
		group.Platform = input.Platform
	}
	if input.RateMultiplier != nil {
		if *input.RateMultiplier <= 0 {
			return nil, errors.New("rate_multiplier must be > 0")
		}
		group.RateMultiplier = *input.RateMultiplier
	}
	if input.IsExclusive != nil {
		group.IsExclusive = *input.IsExclusive
	}
	if input.Status != "" {
		group.Status = input.Status
	}

	// 订阅相关字段
	if input.SubscriptionType != "" {
		group.SubscriptionType = input.SubscriptionType
	}
	// 限额字段：nil/负数 表示"无限制"，0 表示"不允许用量"，正数表示具体限额
	// 前端始终发送这三个字段，无需 nil 守卫
	group.DailyLimitUSD = normalizeLimit(input.DailyLimitUSD)
	group.WeeklyLimitUSD = normalizeLimit(input.WeeklyLimitUSD)
	group.MonthlyLimitUSD = normalizeLimit(input.MonthlyLimitUSD)
	// 图片生成计费配置：负数表示清除（使用默认价格）
	if input.AllowImageGeneration != nil {
		group.AllowImageGeneration = *input.AllowImageGeneration
	}
	if input.ImageRateIndependent != nil {
		group.ImageRateIndependent = *input.ImageRateIndependent
	}
	if input.ImageRateMultiplier != nil {
		if *input.ImageRateMultiplier < 0 {
			return nil, errors.New("image_rate_multiplier must be >= 0")
		}
		group.ImageRateMultiplier = *input.ImageRateMultiplier
	}
	if input.ImagePrice1K != nil {
		group.ImagePrice1K = normalizePrice(input.ImagePrice1K)
	}
	if input.ImagePrice2K != nil {
		group.ImagePrice2K = normalizePrice(input.ImagePrice2K)
	}
	if input.ImagePrice4K != nil {
		group.ImagePrice4K = normalizePrice(input.ImagePrice4K)
	}

	// Claude Code 客户端限制
	if input.ClaudeCodeOnly != nil {
		group.ClaudeCodeOnly = *input.ClaudeCodeOnly
	}
	if input.FallbackGroupID != nil {
		// 校验降级分组
		if *input.FallbackGroupID > 0 {
			if err := s.validateFallbackGroup(ctx, id, *input.FallbackGroupID); err != nil {
				return nil, err
			}
			group.FallbackGroupID = input.FallbackGroupID
		} else {
			// 传入 0 或负数表示清除降级分组
			group.FallbackGroupID = nil
		}
	}
	fallbackOnInvalidRequest := group.FallbackGroupIDOnInvalidRequest
	if input.FallbackGroupIDOnInvalidRequest != nil {
		if *input.FallbackGroupIDOnInvalidRequest > 0 {
			fallbackOnInvalidRequest = input.FallbackGroupIDOnInvalidRequest
		} else {
			fallbackOnInvalidRequest = nil
		}
	}
	if fallbackOnInvalidRequest != nil {
		if err := s.validateFallbackGroupOnInvalidRequest(ctx, id, group.Platform, group.SubscriptionType, *fallbackOnInvalidRequest); err != nil {
			return nil, err
		}
	}
	group.FallbackGroupIDOnInvalidRequest = fallbackOnInvalidRequest

	// 模型路由配置
	if input.ModelRouting != nil {
		group.ModelRouting = input.ModelRouting
	}
	if input.ModelRoutingEnabled != nil {
		group.ModelRoutingEnabled = *input.ModelRoutingEnabled
	}
	if input.MCPXMLInject != nil {
		group.MCPXMLInject = *input.MCPXMLInject
	}

	// 支持的模型系列（仅 antigravity 平台使用）
	if input.SupportedModelScopes != nil {
		group.SupportedModelScopes = *input.SupportedModelScopes
	}

	// OpenAI Messages 调度配置
	if input.AllowMessagesDispatch != nil {
		group.AllowMessagesDispatch = *input.AllowMessagesDispatch
	}
	if input.RequireOAuthOnly != nil {
		group.RequireOAuthOnly = *input.RequireOAuthOnly
	}
	if input.RequirePrivacySet != nil {
		group.RequirePrivacySet = *input.RequirePrivacySet
	}
	if input.DefaultMappedModel != nil {
		group.DefaultMappedModel = *input.DefaultMappedModel
	}
	if input.MessagesDispatchModelConfig != nil {
		group.MessagesDispatchModelConfig = normalizeOpenAIMessagesDispatchModelConfig(*input.MessagesDispatchModelConfig)
	}
	if input.RPMLimit != nil {
		group.RPMLimit = *input.RPMLimit
	}
	sanitizeGroupMessagesDispatchFields(group)

	if err := s.groupRepo.Update(ctx, group); err != nil {
		return nil, err
	}

	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByGroupID(ctx, id)
	}

	// 如果指定了复制账号的源分组，同步绑定（替换当前分组的账号）
	if len(input.CopyAccountsFromGroupIDs) > 0 {
		// 去重源分组 IDs
		seen := make(map[int64]struct{})
		uniqueSourceGroupIDs := make([]int64, 0, len(input.CopyAccountsFromGroupIDs))
		for _, srcGroupID := range input.CopyAccountsFromGroupIDs {
			// 校验：源分组不能是自身
			if srcGroupID == id {
				return nil, fmt.Errorf("cannot copy accounts from self")
			}
			// 去重
			if _, exists := seen[srcGroupID]; !exists {
				seen[srcGroupID] = struct{}{}
				uniqueSourceGroupIDs = append(uniqueSourceGroupIDs, srcGroupID)
			}
		}

		// 校验源分组的平台是否与当前分组一致
		for _, srcGroupID := range uniqueSourceGroupIDs {
			srcGroup, err := s.groupRepo.GetByIDLite(ctx, srcGroupID)
			if err != nil {
				return nil, fmt.Errorf("source group %d not found: %w", srcGroupID, err)
			}
			if srcGroup.Platform != group.Platform {
				return nil, fmt.Errorf("source group %d platform mismatch: expected %s, got %s", srcGroupID, group.Platform, srcGroup.Platform)
			}
		}

		// 获取所有源分组的账号（去重）
		accountIDsToCopy, err := s.groupRepo.GetAccountIDsByGroupIDs(ctx, uniqueSourceGroupIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to get accounts from source groups: %w", err)
		}

		// 先清空当前分组的所有账号绑定
		if _, err := s.groupRepo.DeleteAccountGroupsByGroupID(ctx, id); err != nil {
			return nil, fmt.Errorf("failed to clear existing account bindings: %w", err)
		}

		// require_oauth_only: 过滤掉 apikey 类型账号
		if group.RequireOAuthOnly && (group.Platform == PlatformOpenAI || group.Platform == PlatformAntigravity || group.Platform == PlatformAnthropic || group.Platform == PlatformGemini) && len(accountIDsToCopy) > 0 {
			accounts, err := s.accountRepo.GetByIDs(ctx, accountIDsToCopy)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch accounts for oauth filter: %w", err)
			}
			oauthIDs := make(map[int64]struct{}, len(accounts))
			for _, acc := range accounts {
				if acc.Type != AccountTypeAPIKey {
					oauthIDs[acc.ID] = struct{}{}
				}
			}
			var filtered []int64
			for _, aid := range accountIDsToCopy {
				if _, ok := oauthIDs[aid]; ok {
					filtered = append(filtered, aid)
				}
			}
			accountIDsToCopy = filtered
		}

		// 再绑定源分组的账号
		if len(accountIDsToCopy) > 0 {
			if err := s.groupRepo.BindAccountsToGroup(ctx, id, accountIDsToCopy); err != nil {
				return nil, fmt.Errorf("failed to bind accounts to group: %w", err)
			}
		}
	}

	return group, nil
}

func (s *adminServiceImpl) DeleteGroup(ctx context.Context, id int64) error {
	var groupKeys []string
	if s.authCacheInvalidator != nil {
		keys, err := s.apiKeyRepo.ListKeysByGroupID(ctx, id)
		if err == nil {
			groupKeys = keys
		}
	}

	affectedUserIDs, err := s.groupRepo.DeleteCascade(ctx, id)
	if err != nil {
		return err
	}
	// 注意：user_group_rate_multipliers 表通过外键 ON DELETE CASCADE 自动清理

	// 事务成功后，异步失效受影响用户的订阅缓存
	if len(affectedUserIDs) > 0 && s.billingCacheService != nil {
		groupID := id
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			for _, userID := range affectedUserIDs {
				if err := s.billingCacheService.InvalidateSubscription(cacheCtx, userID, groupID); err != nil {
					logger.LegacyPrintf("service.admin", "invalidate subscription cache failed: user_id=%d group_id=%d err=%v", userID, groupID, err)
				}
			}
		}()
	}
	if s.authCacheInvalidator != nil {
		for _, key := range groupKeys {
			s.authCacheInvalidator.InvalidateAuthCacheByKey(ctx, key)
		}
	}

	return nil
}

func (s *adminServiceImpl) GetGroupAPIKeys(ctx context.Context, groupID int64, page, pageSize int) ([]APIKey, int64, error) {
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}
	keys, result, err := s.apiKeyRepo.ListByGroupID(ctx, groupID, params)
	if err != nil {
		return nil, 0, err
	}
	return keys, result.Total, nil
}

func (s *adminServiceImpl) GetGroupRateMultipliers(ctx context.Context, groupID int64) ([]UserGroupRateEntry, error) {
	if s.userGroupRateRepo == nil {
		return nil, nil
	}
	return s.userGroupRateRepo.GetByGroupID(ctx, groupID)
}

func (s *adminServiceImpl) ClearGroupRateMultipliers(ctx context.Context, groupID int64) error {
	if s.userGroupRateRepo == nil {
		return nil
	}
	return s.userGroupRateRepo.DeleteByGroupID(ctx, groupID)
}

func (s *adminServiceImpl) BatchSetGroupRateMultipliers(ctx context.Context, groupID int64, entries []GroupRateMultiplierInput) error {
	if s.userGroupRateRepo == nil {
		return nil
	}
	for _, e := range entries {
		if e.RateMultiplier <= 0 {
			return fmt.Errorf("rate_multiplier must be > 0 (user_id=%d)", e.UserID)
		}
	}
	return s.userGroupRateRepo.SyncGroupRateMultipliers(ctx, groupID, entries)
}

func (s *adminServiceImpl) ClearGroupRPMOverrides(ctx context.Context, groupID int64) error {
	if s.userGroupRateRepo == nil {
		return nil
	}
	if err := s.userGroupRateRepo.ClearGroupRPMOverrides(ctx, groupID); err != nil {
		return err
	}
	// RPM override 已嵌入 auth cache snapshot (v7)，变更后必须失效相关缓存。
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByGroupID(ctx, groupID)
	}
	return nil
}

func (s *adminServiceImpl) BatchSetGroupRPMOverrides(ctx context.Context, groupID int64, entries []GroupRPMOverrideInput) error {
	if s.userGroupRateRepo == nil {
		return nil
	}
	for _, e := range entries {
		if e.RPMOverride != nil && *e.RPMOverride < 0 {
			return infraerrors.BadRequest("INVALID_RPM_OVERRIDE", fmt.Sprintf("rpm_override must be >= 0 (user_id=%d)", e.UserID))
		}
	}
	if err := s.userGroupRateRepo.SyncGroupRPMOverrides(ctx, groupID, entries); err != nil {
		return err
	}
	// RPM override 已嵌入 auth cache snapshot (v7)，变更后必须失效相关缓存。
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByGroupID(ctx, groupID)
	}
	return nil
}

func (s *adminServiceImpl) UpdateGroupSortOrders(ctx context.Context, updates []GroupSortOrderUpdate) error {
	return s.groupRepo.UpdateSortOrders(ctx, updates)
}

// AdminUpdateAPIKeyGroupID 管理员修改 API Key 分组绑定
// groupID: nil=不修改, 指向0=解绑, 指向正整数=绑定到目标分组
func (s *adminServiceImpl) AdminUpdateAPIKeyGroupID(ctx context.Context, keyID int64, groupID *int64) (*AdminUpdateAPIKeyGroupIDResult, error) {
	apiKey, err := s.apiKeyRepo.GetByID(ctx, keyID)
	if err != nil {
		return nil, err
	}

	if groupID == nil {
		// nil 表示不修改，直接返回
		return &AdminUpdateAPIKeyGroupIDResult{APIKey: apiKey}, nil
	}

	if *groupID < 0 {
		return nil, infraerrors.BadRequest("INVALID_GROUP_ID", "group_id must be non-negative")
	}

	result := &AdminUpdateAPIKeyGroupIDResult{}

	if *groupID == 0 {
		// 0 表示解绑分组（不修改 user_allowed_groups，避免影响用户其他 Key）
		apiKey.GroupID = nil
		apiKey.Group = nil
	} else {
		// 验证目标分组存在且状态为 active
		group, err := s.groupRepo.GetByID(ctx, *groupID)
		if err != nil {
			return nil, err
		}
		if group.Status != StatusActive {
			return nil, infraerrors.BadRequest("GROUP_NOT_ACTIVE", "target group is not active")
		}
		// 订阅类型分组：用户须持有该分组的有效订阅才可绑定
		if group.IsSubscriptionType() {
			if s.userSubRepo == nil {
				return nil, infraerrors.InternalServer("SUBSCRIPTION_REPOSITORY_UNAVAILABLE", "subscription repository is not configured")
			}
			if _, err := s.userSubRepo.GetActiveByUserIDAndGroupID(ctx, apiKey.UserID, *groupID); err != nil {
				if errors.Is(err, ErrSubscriptionNotFound) {
					return nil, infraerrors.BadRequest("SUBSCRIPTION_REQUIRED", "user does not have an active subscription for this group")
				}
				return nil, err
			}
		}

		gid := *groupID
		apiKey.GroupID = &gid
		apiKey.Group = group

		// 专属标准分组：使用事务保证「添加分组权限」与「更新 API Key」的原子性
		if group.IsExclusive && !group.IsSubscriptionType() {
			opCtx := ctx
			var tx *dbent.Tx
			if s.entClient == nil {
				logger.LegacyPrintf("service.admin", "Warning: entClient is nil, skipping transaction protection for exclusive group binding")
			} else {
				var txErr error
				tx, txErr = s.entClient.Tx(ctx)
				if txErr != nil {
					return nil, fmt.Errorf("begin transaction: %w", txErr)
				}
				defer func() { _ = tx.Rollback() }()
				opCtx = dbent.NewTxContext(ctx, tx)
			}

			if addErr := s.userRepo.AddGroupToAllowedGroups(opCtx, apiKey.UserID, gid); addErr != nil {
				return nil, fmt.Errorf("add group to user allowed groups: %w", addErr)
			}
			if err := s.apiKeyRepo.Update(opCtx, apiKey); err != nil {
				return nil, fmt.Errorf("update api key: %w", err)
			}
			if tx != nil {
				if err := tx.Commit(); err != nil {
					return nil, fmt.Errorf("commit transaction: %w", err)
				}
			}

			result.AutoGrantedGroupAccess = true
			result.GrantedGroupID = &gid
			result.GrantedGroupName = group.Name

			// 失效认证缓存（在事务提交后执行）
			if s.authCacheInvalidator != nil {
				s.authCacheInvalidator.InvalidateAuthCacheByKey(ctx, apiKey.Key)
			}

			result.APIKey = apiKey
			return result, nil
		}
	}

	// 非专属分组 / 解绑：无需事务，单步更新即可
	if err := s.apiKeyRepo.Update(ctx, apiKey); err != nil {
		return nil, fmt.Errorf("update api key: %w", err)
	}

	// 失效认证缓存
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByKey(ctx, apiKey.Key)
	}

	result.APIKey = apiKey
	return result, nil
}

// AdminResetAPIKeyRateLimitUsage resets all API key rate-limit usage windows.
func (s *adminServiceImpl) AdminResetAPIKeyRateLimitUsage(ctx context.Context, keyID int64) (*APIKey, error) {
	apiKey, err := s.apiKeyRepo.GetByID(ctx, keyID)
	if err != nil {
		return nil, err
	}
	apiKey.Usage5h = 0
	apiKey.Usage1d = 0
	apiKey.Usage7d = 0
	apiKey.Window5hStart = nil
	apiKey.Window1dStart = nil
	apiKey.Window7dStart = nil
	if err := s.apiKeyRepo.Update(ctx, apiKey); err != nil {
		return nil, fmt.Errorf("reset api key rate limit usage: %w", err)
	}
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByKey(ctx, apiKey.Key)
	}
	if s.billingCacheService != nil {
		_ = s.billingCacheService.InvalidateAPIKeyRateLimit(ctx, apiKey.ID)
	}
	return apiKey, nil
}

// ReplaceUserGroup 替换用户的专属分组
func (s *adminServiceImpl) ReplaceUserGroup(ctx context.Context, userID, oldGroupID, newGroupID int64) (*ReplaceUserGroupResult, error) {
	if oldGroupID == newGroupID {
		return nil, infraerrors.BadRequest("SAME_GROUP", "old and new group must be different")
	}

	// 验证新分组存在且为活跃的专属标准分组
	newGroup, err := s.groupRepo.GetByID(ctx, newGroupID)
	if err != nil {
		return nil, err
	}
	if newGroup.Status != StatusActive {
		return nil, infraerrors.BadRequest("GROUP_NOT_ACTIVE", "target group is not active")
	}
	if !newGroup.IsExclusive {
		return nil, infraerrors.BadRequest("GROUP_NOT_EXCLUSIVE", "target group is not exclusive")
	}
	if newGroup.IsSubscriptionType() {
		return nil, infraerrors.BadRequest("GROUP_IS_SUBSCRIPTION", "subscription groups are not supported for replacement")
	}

	// 事务保证原子性
	if s.entClient == nil {
		return nil, fmt.Errorf("entClient is nil, cannot perform group replacement")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	opCtx := dbent.NewTxContext(ctx, tx)

	// 1. 授予新分组权限
	if err := s.userRepo.AddGroupToAllowedGroups(opCtx, userID, newGroupID); err != nil {
		return nil, fmt.Errorf("add new group to allowed groups: %w", err)
	}

	// 2. 迁移绑定旧分组的 Key 到新分组
	migrated, err := s.apiKeyRepo.UpdateGroupIDByUserAndGroup(opCtx, userID, oldGroupID, newGroupID)
	if err != nil {
		return nil, fmt.Errorf("migrate api keys: %w", err)
	}

	// 3. 移除旧分组权限
	if err := s.userRepo.RemoveGroupFromUserAllowedGroups(opCtx, userID, oldGroupID); err != nil {
		return nil, fmt.Errorf("remove old group from allowed groups: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	// 失效该用户所有 Key 的认证缓存
	if s.authCacheInvalidator != nil {
		keys, keyErr := s.apiKeyRepo.ListKeysByUserID(ctx, userID)
		if keyErr == nil {
			for _, k := range keys {
				s.authCacheInvalidator.InvalidateAuthCacheByKey(ctx, k)
			}
		}
	}

	return &ReplaceUserGroupResult{MigratedKeys: migrated}, nil
}

// Account management implementations
func (s *adminServiceImpl) ListAccounts(ctx context.Context, page, pageSize int, platform, accountType, status, search string, groupID int64, privacyMode string, sortBy, sortOrder string) ([]Account, int64, error) {
	params := pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: sortBy, SortOrder: sortOrder}
	accounts, result, err := s.accountRepo.ListWithFilters(ctx, params, platform, accountType, status, search, groupID, privacyMode)
	if err != nil {
		return nil, 0, err
	}
	return accounts, result.Total, nil
}

func (s *adminServiceImpl) GetAccount(ctx context.Context, id int64) (*Account, error) {
	return s.accountRepo.GetByID(ctx, id)
}

func (s *adminServiceImpl) GetAccountsByIDs(ctx context.Context, ids []int64) ([]*Account, error) {
	if len(ids) == 0 {
		return []*Account{}, nil
	}

	accounts, err := s.accountRepo.GetByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts by IDs: %w", err)
	}

	return accounts, nil
}

func (s *adminServiceImpl) CreateAccount(ctx context.Context, input *CreateAccountInput) (*Account, error) {
	// 绑定分组
	groupIDs := input.GroupIDs
	// 如果没有指定分组,自动绑定对应平台的默认分组
	if len(groupIDs) == 0 && !input.SkipDefaultGroupBind {
		defaultGroupName := input.Platform + "-default"
		groups, err := s.groupRepo.ListActiveByPlatform(ctx, input.Platform)
		if err == nil {
			for _, g := range groups {
				if g.Name == defaultGroupName {
					groupIDs = []int64{g.ID}
					break
				}
			}
		}
	}

	// 检查混合渠道风险（除非用户已确认）
	if len(groupIDs) > 0 && !input.SkipMixedChannelCheck {
		if err := s.checkMixedChannelRisk(ctx, 0, input.Platform, groupIDs); err != nil {
			return nil, err
		}
	}

	account := &Account{
		Name:        input.Name,
		Notes:       normalizeAccountNotes(input.Notes),
		Platform:    input.Platform,
		Type:        input.Type,
		Credentials: input.Credentials,
		Extra:       input.Extra,
		ProxyID:     input.ProxyID,
		Concurrency: input.Concurrency,
		Priority:    input.Priority,
		Status:      StatusActive,
		Schedulable: true,
	}
	// 预计算固定时间重置的下次重置时间
	if account.Extra != nil {
		if err := ValidateQuotaResetConfig(account.Extra); err != nil {
			return nil, err
		}
		ComputeQuotaResetAt(account.Extra)
	}
	if input.ExpiresAt != nil && *input.ExpiresAt > 0 {
		expiresAt := time.Unix(*input.ExpiresAt, 0)
		account.ExpiresAt = &expiresAt
	}
	if input.AutoPauseOnExpired != nil {
		account.AutoPauseOnExpired = *input.AutoPauseOnExpired
	} else {
		account.AutoPauseOnExpired = true
	}
	if input.RateMultiplier != nil {
		if *input.RateMultiplier < 0 {
			return nil, errors.New("rate_multiplier must be >= 0")
		}
		account.RateMultiplier = input.RateMultiplier
	}
	if input.LoadFactor != nil && *input.LoadFactor > 0 {
		if *input.LoadFactor > 10000 {
			return nil, errors.New("load_factor must be <= 10000")
		}
		account.LoadFactor = input.LoadFactor
	}
	if err := s.accountRepo.Create(ctx, account); err != nil {
		return nil, err
	}
	if input.ProxyID != nil && *input.ProxyID > 0 {
		if err := s.recordAccountProxyBinding(ctx, account, *input.ProxyID, ProxyBindingSourceManual, ProxyBindingStatusActive); err != nil {
			logger.LegacyPrintf("service.admin", "failed to record manual proxy binding for account %d: %v", account.ID, err)
		}
	} else if settings, err := s.GetProxyDispatchSettings(ctx); err == nil && settings.AutoAssignEnabled && s.entClient != nil {
		if _, err := s.assignProxyForAccount(ctx, account, false); err != nil {
			logger.LegacyPrintf("service.admin", "failed to auto assign proxy for account %d: %v", account.ID, err)
		}
	}

	// 绑定分组
	if len(groupIDs) > 0 {
		if err := s.accountRepo.BindGroups(ctx, account.ID, groupIDs); err != nil {
			return nil, err
		}
	}

	// OAuth 账号：创建后异步设置隐私。
	// 使用 Ensure（幂等）而非 Force：新建账号 Extra 为空时效果相同，但更安全。
	if account.Type == AccountTypeOAuth {
		switch account.Platform {
		case PlatformOpenAI:
			go func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("create_account_openai_privacy_panic", "account_id", account.ID, "recover", r)
					}
				}()
				s.EnsureOpenAIPrivacy(context.Background(), account)
			}()
		case PlatformAntigravity:
			go func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("create_account_antigravity_privacy_panic", "account_id", account.ID, "recover", r)
					}
				}()
				s.EnsureAntigravityPrivacy(context.Background(), account)
			}()
		}
	}

	return account, nil
}

func (s *adminServiceImpl) UpdateAccount(ctx context.Context, id int64, input *UpdateAccountInput) (*Account, error) {
	account, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	wasOveragesEnabled := account.IsOveragesEnabled()

	if input.Name != "" {
		account.Name = input.Name
	}
	if input.Type != "" {
		account.Type = input.Type
	}
	if input.Notes != nil {
		account.Notes = normalizeAccountNotes(input.Notes)
	}
	if len(input.Credentials) > 0 {
		account.Credentials = input.Credentials
	}
	// Extra 使用 map：需要区分“未提供(nil)”与“显式清空({})”。
	// 关闭配额限制时前端会删除 quota_* 键并提交 extra:{}，此时也必须落库。
	if input.Extra != nil {
		// 保留配额用量字段，防止编辑账号时意外重置
		for _, key := range []string{"quota_used", "quota_daily_used", "quota_daily_start", "quota_weekly_used", "quota_weekly_start"} {
			if v, ok := account.Extra[key]; ok {
				input.Extra[key] = v
			}
		}
		account.Extra = input.Extra
		if account.Platform == PlatformAntigravity && wasOveragesEnabled && !account.IsOveragesEnabled() {
			delete(account.Extra, "antigravity_credits_overages") // 清理旧版 overages 运行态
			// 清除 AICredits 限流 key
			if rawLimits, ok := account.Extra[modelRateLimitsKey].(map[string]any); ok {
				delete(rawLimits, creditsExhaustedKey)
			}
		}
		if account.Platform == PlatformAntigravity && !wasOveragesEnabled && account.IsOveragesEnabled() {
			delete(account.Extra, modelRateLimitsKey)
			delete(account.Extra, "antigravity_credits_overages") // 清理旧版 overages 运行态
		}
		// 校验并预计算固定时间重置的下次重置时间
		if err := ValidateQuotaResetConfig(account.Extra); err != nil {
			return nil, err
		}
		ComputeQuotaResetAt(account.Extra)
	}
	if input.ProxyID != nil {
		// 0 表示清除代理（前端发送 0 而不是 null 来表达清除意图）
		if *input.ProxyID == 0 {
			account.ProxyID = nil
		} else {
			account.ProxyID = input.ProxyID
		}
		account.Proxy = nil // 清除关联对象，防止 GORM Save 时根据 Proxy.ID 覆盖 ProxyID
	}
	// 只在指针非 nil 时更新 Concurrency（支持设置为 0）
	if input.Concurrency != nil {
		account.Concurrency = *input.Concurrency
	}
	// 只在指针非 nil 时更新 Priority（支持设置为 0）
	if input.Priority != nil {
		account.Priority = *input.Priority
	}
	if input.RateMultiplier != nil {
		if *input.RateMultiplier < 0 {
			return nil, errors.New("rate_multiplier must be >= 0")
		}
		account.RateMultiplier = input.RateMultiplier
	}
	if input.LoadFactor != nil {
		if *input.LoadFactor <= 0 {
			account.LoadFactor = nil // 0 或负数表示清除
		} else if *input.LoadFactor > 10000 {
			return nil, errors.New("load_factor must be <= 10000")
		} else {
			account.LoadFactor = input.LoadFactor
		}
	}
	if input.Status != "" {
		account.Status = input.Status
	}
	if input.ExpiresAt != nil {
		if *input.ExpiresAt <= 0 {
			account.ExpiresAt = nil
		} else {
			expiresAt := time.Unix(*input.ExpiresAt, 0)
			account.ExpiresAt = &expiresAt
		}
	}
	if input.AutoPauseOnExpired != nil {
		account.AutoPauseOnExpired = *input.AutoPauseOnExpired
	}

	// 先验证分组是否存在（在任何写操作之前）
	if input.GroupIDs != nil {
		if err := s.validateGroupIDsExist(ctx, *input.GroupIDs); err != nil {
			return nil, err
		}

		// 检查混合渠道风险（除非用户已确认）
		if !input.SkipMixedChannelCheck {
			if err := s.checkMixedChannelRisk(ctx, account.ID, account.Platform, *input.GroupIDs); err != nil {
				return nil, err
			}
		}
	}

	if err := s.accountRepo.Update(ctx, account); err != nil {
		return nil, err
	}
	if input.ProxyID != nil {
		if account.ProxyID != nil && *account.ProxyID > 0 {
			if err := s.recordAccountProxyBinding(ctx, account, *account.ProxyID, ProxyBindingSourceManual, ProxyBindingStatusActive); err != nil {
				logger.LegacyPrintf("service.admin", "failed to record manual proxy binding for account %d: %v", account.ID, err)
			}
		} else if err := s.deactivateAccountProxyBindings(ctx, account); err != nil {
			logger.LegacyPrintf("service.admin", "failed to deactivate proxy bindings for account %d: %v", account.ID, err)
		}
	}

	// 绑定分组
	if input.GroupIDs != nil {
		if err := s.accountRepo.BindGroups(ctx, account.ID, *input.GroupIDs); err != nil {
			return nil, err
		}
	}

	// 重新查询以确保返回完整数据（包括正确的 Proxy 关联对象）
	updated, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// BulkUpdateAccounts updates multiple accounts in one request.
// It merges credentials/extra keys instead of overwriting the whole object.
func (s *adminServiceImpl) BulkUpdateAccounts(ctx context.Context, input *BulkUpdateAccountsInput) (*BulkUpdateAccountsResult, error) {
	if len(input.AccountIDs) == 0 && input.Filters != nil {
		accountIDs, err := s.resolveBulkUpdateTargetIDs(ctx, input.Filters)
		if err != nil {
			return nil, err
		}
		input.AccountIDs = accountIDs
	}

	result := &BulkUpdateAccountsResult{
		SuccessIDs: make([]int64, 0, len(input.AccountIDs)),
		FailedIDs:  make([]int64, 0, len(input.AccountIDs)),
		Results:    make([]BulkUpdateAccountResult, 0, len(input.AccountIDs)),
	}

	if len(input.AccountIDs) == 0 {
		return result, nil
	}
	if input.GroupIDs != nil {
		if err := s.validateGroupIDsExist(ctx, *input.GroupIDs); err != nil {
			return nil, err
		}
	}

	needMixedChannelCheck := input.GroupIDs != nil && !input.SkipMixedChannelCheck

	// 预加载账号平台信息（混合渠道检查需要）。
	platformByID := map[int64]string{}
	if needMixedChannelCheck {
		accounts, err := s.accountRepo.GetByIDs(ctx, input.AccountIDs)
		if err != nil {
			return nil, err
		}
		for _, account := range accounts {
			if account != nil {
				platformByID[account.ID] = account.Platform
			}
		}
	}

	// 预检查混合渠道风险：在任何写操作之前，若发现风险立即返回错误。
	if needMixedChannelCheck {
		for _, accountID := range input.AccountIDs {
			platform := platformByID[accountID]
			if platform == "" {
				continue
			}
			if err := s.checkMixedChannelRisk(ctx, accountID, platform, *input.GroupIDs); err != nil {
				return nil, err
			}
		}
	}

	if input.RateMultiplier != nil {
		if *input.RateMultiplier < 0 {
			return nil, errors.New("rate_multiplier must be >= 0")
		}
	}

	// Prepare bulk updates for columns and JSONB fields.
	repoUpdates := AccountBulkUpdate{
		Credentials: input.Credentials,
		Extra:       input.Extra,
	}
	if input.Name != "" {
		repoUpdates.Name = &input.Name
	}
	if input.ProxyID != nil {
		repoUpdates.ProxyID = input.ProxyID
	}
	if input.Concurrency != nil {
		repoUpdates.Concurrency = input.Concurrency
	}
	if input.Priority != nil {
		repoUpdates.Priority = input.Priority
	}
	if input.RateMultiplier != nil {
		repoUpdates.RateMultiplier = input.RateMultiplier
	}
	if input.LoadFactor != nil {
		if *input.LoadFactor <= 0 {
			repoUpdates.LoadFactor = nil // 0 或负数表示清除
		} else if *input.LoadFactor > 10000 {
			return nil, errors.New("load_factor must be <= 10000")
		} else {
			repoUpdates.LoadFactor = input.LoadFactor
		}
	}
	if input.Status != "" {
		repoUpdates.Status = &input.Status
	}
	if input.Schedulable != nil {
		repoUpdates.Schedulable = input.Schedulable
	}

	// Run bulk update for column/jsonb fields first.
	if _, err := s.accountRepo.BulkUpdate(ctx, input.AccountIDs, repoUpdates); err != nil {
		return nil, err
	}

	// Handle group bindings per account (requires individual operations).
	for _, accountID := range input.AccountIDs {
		entry := BulkUpdateAccountResult{AccountID: accountID}

		if input.GroupIDs != nil {
			if err := s.accountRepo.BindGroups(ctx, accountID, *input.GroupIDs); err != nil {
				entry.Success = false
				entry.Error = err.Error()
				result.Failed++
				result.FailedIDs = append(result.FailedIDs, accountID)
				result.Results = append(result.Results, entry)
				continue
			}
		}

		entry.Success = true
		result.Success++
		result.SuccessIDs = append(result.SuccessIDs, accountID)
		result.Results = append(result.Results, entry)
	}

	return result, nil
}

func (s *adminServiceImpl) resolveBulkUpdateTargetIDs(ctx context.Context, filters *BulkUpdateAccountFilters) ([]int64, error) {
	if filters == nil {
		return nil, nil
	}

	groupID := int64(0)
	switch strings.TrimSpace(filters.Group) {
	case "":
	case "ungrouped":
		groupID = AccountListGroupUngrouped
	default:
		parsedGroupID, err := strconv.ParseInt(strings.TrimSpace(filters.Group), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid group filter: %w", err)
		}
		groupID = parsedGroupID
	}

	const pageSize = 500
	page := 1
	accountIDs := make([]int64, 0, pageSize)

	for {
		accounts, total, err := s.ListAccounts(
			ctx,
			page,
			pageSize,
			filters.Platform,
			filters.Type,
			filters.Status,
			filters.Search,
			groupID,
			filters.PrivacyMode,
			"",
			"",
		)
		if err != nil {
			return nil, err
		}
		for _, account := range accounts {
			accountIDs = append(accountIDs, account.ID)
		}
		if int64(len(accountIDs)) >= total || len(accounts) == 0 {
			return accountIDs, nil
		}
		page++
	}
}

func (s *adminServiceImpl) DeleteAccount(ctx context.Context, id int64) error {
	account, _ := s.accountRepo.GetByID(ctx, id)
	if err := s.accountRepo.Delete(ctx, id); err != nil {
		return err
	}
	if account != nil {
		if err := s.markAccountProxyBindingsDeleted(ctx, account); err != nil {
			logger.LegacyPrintf("service.admin", "failed to mark proxy bindings deleted for account %d: %v", id, err)
		}
	}
	return nil
}

func (s *adminServiceImpl) RefreshAccountCredentials(ctx context.Context, id int64) (*Account, error) {
	account, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// TODO: Implement refresh logic
	return account, nil
}

func (s *adminServiceImpl) ClearAccountError(ctx context.Context, id int64) (*Account, error) {
	if err := s.accountRepo.ClearError(ctx, id); err != nil {
		return nil, err
	}
	if err := s.accountRepo.ClearRateLimit(ctx, id); err != nil {
		return nil, err
	}
	if err := s.accountRepo.ClearAntigravityQuotaScopes(ctx, id); err != nil {
		return nil, err
	}
	if err := s.accountRepo.ClearModelRateLimits(ctx, id); err != nil {
		return nil, err
	}
	if err := s.accountRepo.ClearTempUnschedulable(ctx, id); err != nil {
		return nil, err
	}
	return s.accountRepo.GetByID(ctx, id)
}

func (s *adminServiceImpl) SetAccountError(ctx context.Context, id int64, errorMsg string) error {
	return s.accountRepo.SetError(ctx, id, errorMsg)
}

func (s *adminServiceImpl) SetAccountSchedulable(ctx context.Context, id int64, schedulable bool) (*Account, error) {
	if err := s.accountRepo.SetSchedulable(ctx, id, schedulable); err != nil {
		return nil, err
	}
	updated, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// Proxy management implementations
func (s *adminServiceImpl) ListProxies(ctx context.Context, page, pageSize int, protocol, status, search string, sortBy, sortOrder string) ([]Proxy, int64, error) {
	params := pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: sortBy, SortOrder: sortOrder}
	proxies, result, err := s.proxyRepo.ListWithFilters(ctx, params, protocol, status, search)
	if err != nil {
		return nil, 0, err
	}
	s.attachProxyMetadata(ctx, proxies)
	return proxies, result.Total, nil
}

func (s *adminServiceImpl) ListProxiesWithAccountCount(ctx context.Context, page, pageSize int, protocol, status, search string, sortBy, sortOrder string) ([]ProxyWithAccountCount, int64, error) {
	params := pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: sortBy, SortOrder: sortOrder}
	proxies, result, err := s.proxyRepo.ListWithFiltersAndAccountCount(ctx, params, protocol, status, search)
	if err != nil {
		return nil, 0, err
	}
	base := make([]Proxy, 0, len(proxies))
	for i := range proxies {
		base = append(base, proxies[i].Proxy)
	}
	s.attachProxyMetadata(ctx, base)
	for i := range proxies {
		proxies[i].Proxy = base[i]
		if proxies[i].QualityStatus == "" {
			proxies[i].QualityStatus = proxies[i].Proxy.QualityStatus
		}
		if proxies[i].IPAddress == "" {
			proxies[i].IPAddress = proxies[i].ExitIP
		}
	}
	s.attachProxyLatency(ctx, proxies)
	return proxies, result.Total, nil
}

func (s *adminServiceImpl) GetAllProxies(ctx context.Context) ([]Proxy, error) {
	proxies, err := s.proxyRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	s.attachProxyMetadata(ctx, proxies)
	return proxies, nil
}

func (s *adminServiceImpl) GetAllProxiesWithAccountCount(ctx context.Context) ([]ProxyWithAccountCount, error) {
	proxies, err := s.proxyRepo.ListActiveWithAccountCount(ctx)
	if err != nil {
		return nil, err
	}
	base := make([]Proxy, 0, len(proxies))
	for i := range proxies {
		base = append(base, proxies[i].Proxy)
	}
	s.attachProxyMetadata(ctx, base)
	for i := range proxies {
		proxies[i].Proxy = base[i]
		if proxies[i].QualityStatus == "" {
			proxies[i].QualityStatus = proxies[i].Proxy.QualityStatus
		}
		if proxies[i].IPAddress == "" {
			proxies[i].IPAddress = proxies[i].ExitIP
		}
	}
	s.attachProxyLatency(ctx, proxies)
	return proxies, nil
}

func (s *adminServiceImpl) GetProxy(ctx context.Context, id int64) (*Proxy, error) {
	proxy, err := s.proxyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	enriched := []Proxy{*proxy}
	s.attachProxyMetadata(ctx, enriched)
	return &enriched[0], nil
}

func (s *adminServiceImpl) GetProxiesByIDs(ctx context.Context, ids []int64) ([]Proxy, error) {
	proxies, err := s.proxyRepo.ListByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	s.attachProxyMetadata(ctx, proxies)
	return proxies, nil
}

func (s *adminServiceImpl) CreateProxy(ctx context.Context, input *CreateProxyInput) (*Proxy, error) {
	proxy := &Proxy{
		Name:     input.Name,
		Protocol: input.Protocol,
		Host:     input.Host,
		Port:     input.Port,
		Username: input.Username,
		Password: input.Password,
		Status:   StatusActive,
	}
	applyProxyInputMetadata(proxy, input)
	if err := s.proxyRepo.Create(ctx, proxy); err != nil {
		return nil, err
	}
	if err := s.saveProxyMetadata(ctx, proxy.ID, proxy); err != nil {
		return nil, err
	}
	// Probe latency asynchronously so creation isn't blocked by network timeout.
	go s.probeProxyLatency(context.Background(), proxy)
	return proxy, nil
}

func (s *adminServiceImpl) UpdateProxy(ctx context.Context, id int64, input *UpdateProxyInput) (*Proxy, error) {
	proxy, err := s.proxyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Name != "" {
		proxy.Name = input.Name
	}
	if input.Protocol != "" {
		proxy.Protocol = input.Protocol
	}
	if input.Host != "" {
		proxy.Host = input.Host
	}
	if input.Port != 0 {
		proxy.Port = input.Port
	}
	if input.Username != "" {
		proxy.Username = input.Username
	}
	if input.Password != "" {
		proxy.Password = input.Password
	}
	if input.Status != "" {
		proxy.Status = input.Status
	}
	applyProxyUpdateMetadata(proxy, input)

	if err := s.proxyRepo.Update(ctx, proxy); err != nil {
		return nil, err
	}
	if err := s.saveProxyMetadata(ctx, proxy.ID, proxy); err != nil {
		return nil, err
	}
	return proxy, nil
}

func (s *adminServiceImpl) DeleteProxy(ctx context.Context, id int64) error {
	return s.proxyRepo.Delete(ctx, id)
}

func (s *adminServiceImpl) BatchDeleteProxies(ctx context.Context, ids []int64) (*ProxyBatchDeleteResult, error) {
	result := &ProxyBatchDeleteResult{}
	if len(ids) == 0 {
		return result, nil
	}

	for _, id := range ids {
		if err := s.proxyRepo.Delete(ctx, id); err != nil {
			result.Skipped = append(result.Skipped, ProxyBatchDeleteSkipped{
				ID:     id,
				Reason: err.Error(),
			})
			continue
		}
		result.DeletedIDs = append(result.DeletedIDs, id)
	}

	return result, nil
}

func (s *adminServiceImpl) GetProxyAccounts(ctx context.Context, proxyID int64) ([]ProxyAccountSummary, error) {
	return s.proxyRepo.ListAccountSummariesByProxyID(ctx, proxyID)
}

func (s *adminServiceImpl) CheckProxyExists(ctx context.Context, host string, port int, username, password string) (bool, error) {
	return s.proxyRepo.ExistsByHostPortAuth(ctx, host, port, username, password)
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
	settings := &ProxyDispatchSettings{
		DirectFallbackMode: normalizeDirectFallbackMode(input.DirectFallbackMode),
		AutoAssignEnabled:  input.AutoAssignEnabled,
	}
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
		proxy, err := s.CreateProxy(ctx, &CreateProxyInput{
			Name:          name,
			Protocol:      item.Protocol,
			Host:          item.Host,
			Port:          item.Port,
			Username:      item.Username,
			Password:      item.Password,
			Source:        item.Source,
			ProxyType:     item.ProxyType,
			Provider:      item.Provider,
			Region:        item.Region,
			QualityStatus: item.QualityStatus,
			Weight:        100,
		})
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
	defer func() { _ = rows.Close() }()
	var out []ProxySubscriptionSource
	for rows.Next() {
		var item ProxySubscriptionSource
		var strategyRaw, scanResultRaw string
		if err := rows.Scan(
			&item.ID, &item.Name, &item.URL, &item.SourceType, &item.Provider, &item.SyncEnabled, &item.SyncIntervalMinutes,
			&strategyRaw, &item.SidecarEnabled, &item.Runtime, &item.PortStart, &item.PortEnd,
			&item.ScanEnabled, &item.ScanIntervalMinutes, &item.HealthCheckIntervalMinutes, &item.ReputationProvider,
			&item.ReputationAPIKeyRef, &item.LastSyncedAt, &item.LastScanAt, &scanResultRaw,
			&item.LastError, &item.Status, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
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
          COALESCE(last_scan_result::text, '{}'), COALESCE(last_error, ''), status, created_at, updated_at`,
		input.Name, input.URL, input.SourceType, input.Provider, *input.SyncEnabled, input.SyncIntervalMinutes,
		string(strategyRaw), *input.SidecarEnabled, input.Runtime, input.PortStart, input.PortEnd, *input.ScanEnabled,
		input.ScanIntervalMinutes, input.HealthCheckIntervalMinutes, input.ReputationProvider, input.ReputationAPIKeyRef, input.Status)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		var item ProxySubscriptionSource
		var strategyRaw, scanResultRaw string
		if err := rows.Scan(
			&item.ID, &item.Name, &item.URL, &item.SourceType, &item.Provider, &item.SyncEnabled, &item.SyncIntervalMinutes,
			&strategyRaw, &item.SidecarEnabled, &item.Runtime, &item.PortStart, &item.PortEnd,
			&item.ScanEnabled, &item.ScanIntervalMinutes, &item.HealthCheckIntervalMinutes, &item.ReputationProvider,
			&item.ReputationAPIKeyRef, &item.LastSyncedAt, &item.LastScanAt, &scanResultRaw,
			&item.LastError, &item.Status, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
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
          COALESCE(last_scan_result::text, '{}'), COALESCE(last_error, ''), status, created_at, updated_at`,
		id, input.Name, input.URL, input.SourceType, input.Provider, *input.SyncEnabled, input.SyncIntervalMinutes,
		string(strategyRaw), *input.SidecarEnabled, input.Runtime, input.PortStart, input.PortEnd, *input.ScanEnabled,
		input.ScanIntervalMinutes, input.HealthCheckIntervalMinutes, input.ReputationProvider, input.ReputationAPIKeyRef, input.Status)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		var item ProxySubscriptionSource
		var strategyRaw, scanResultRaw string
		if err := rows.Scan(
			&item.ID, &item.Name, &item.URL, &item.SourceType, &item.Provider, &item.SyncEnabled, &item.SyncIntervalMinutes,
			&strategyRaw, &item.SidecarEnabled, &item.Runtime, &item.PortStart, &item.PortEnd,
			&item.ScanEnabled, &item.ScanIntervalMinutes, &item.HealthCheckIntervalMinutes, &item.ReputationProvider,
			&item.ReputationAPIKeyRef, &item.LastSyncedAt, &item.LastScanAt, &scanResultRaw,
			&item.LastError, &item.Status, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
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
	rows, err := s.entClient.QueryContext(ctx, `SELECT url, COALESCE(provider, '') FROM proxy_subscription_sources WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
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
	result := &ProxySubscriptionScanResult{
		SourceID:  id,
		Total:     preview.Total,
		Parsed:    len(items),
		Strategy:  strategy,
		ScannedAt: time.Now(),
	}

	sidecarIndex := 0
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
			eval = proxySubscriptionNodeEvaluation{
				Key:     key,
				Country: inferProxySubscriptionCountry(item),
			}
		}
		status := defaultString(selectedStatuses[key], "candidate")
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
			if isSelected && source.SidecarEnabled && sidecarIndex < strategy.MaxActiveSidecarNodes {
				port := source.PortStart + sidecarIndex
				if port <= source.PortEnd {
					if err := s.reserveProxySidecarEndpoint(scanCtx, source, nodeID, port); err != nil {
						result.Errors = append(result.Errors, err.Error())
					} else {
						if err := s.upsertSidecarProxyForSubscriptionNode(scanCtx, source, nodeID, item, eval, port); err != nil {
							result.Errors = append(result.Errors, err.Error())
						}
						sidecarIndex++
					}
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
	defer func() { _ = rows.Close() }()
	var nodes []ProxySubscriptionNode
	for rows.Next() {
		var n ProxySubscriptionNode
		if err := rows.Scan(
			&n.ID, &n.SourceID, &n.NodeKey, &n.RawURI, &n.Name, &n.Protocol, &n.Server, &n.Port, &n.Username,
			&n.CountryHint, &n.ExitIP, &n.ExitCountry, &n.ExitCountryCode, &n.LatencyMs, &n.IPCleanScore,
			&n.ReputationProvider, &n.ReputationCheckedAt, &n.Score, &n.Status, &n.FailureCount, &n.TimeoutCount,
			&n.SleepUntil, &n.LastScannedAt, &n.LastError, &n.Selected, &n.SidecarRequired, &n.CreatedAt, &n.UpdatedAt,
		); err != nil {
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
WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, ErrProxyNotFound
	}
	var item ProxySubscriptionSource
	var strategyRaw, scanResultRaw string
	if err := rows.Scan(
		&item.ID, &item.Name, &item.URL, &item.SourceType, &item.Provider, &item.SyncEnabled, &item.SyncIntervalMinutes,
		&strategyRaw, &item.SidecarEnabled, &item.Runtime, &item.PortStart, &item.PortEnd,
		&item.ScanEnabled, &item.ScanIntervalMinutes, &item.HealthCheckIntervalMinutes, &item.ReputationProvider,
		&item.ReputationAPIKeyRef, &item.LastSyncedAt, &item.LastScanAt, &scanResultRaw,
		&item.LastError, &item.Status, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
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
		eval := proxySubscriptionNodeEvaluation{
			Key:     key,
			Country: inferProxySubscriptionCountry(item),
		}
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
		if !item.Valid || item.Duplicate {
			continue
		}
		key := item.Key
		if key == "" {
			key = proxyImportItemKey(item)
		}
		eval, ok := evaluations[key]
		if !ok {
			eval = proxySubscriptionNodeEvaluation{
				Key:     key,
				Country: inferProxySubscriptionCountry(item),
				Score:   scoreProxySubscriptionItem(item, strategy, nil, nil),
			}
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
		candidates = append(candidates, candidate{
			key:     key,
			item:    item,
			score:   eval.Score,
			country: defaultString(eval.Country, inferProxySubscriptionCountry(item)),
		})
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
RETURNING id`,
		sourceID, key, raw, item.Name, item.Protocol, item.Host, item.Port, item.Username,
		countryHint, evaluation.ExitIP, evaluation.ExitCountry, evaluation.ExitCountryCode, evaluation.IPCleanScore,
		nullIfBlank(evaluation.ReputationProvider), evaluation.ReputationCheckedAt, string(reputationRaw), evaluation.LatencyMs,
		evaluation.Score, status, evaluation.FailureCount, evaluation.TimeoutCount, evaluation.SleepUntil, selected,
		item.SidecarRequired, nullIfBlank(evaluation.LastError))
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
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
	_, err := s.entClient.ExecContext(ctx, `
INSERT INTO proxy_sidecar_endpoints (source_id, node_id, runtime, listen_host, listen_port, protocol, status, updated_at)
VALUES ($1, $2, $3, $4, $5, 'socks5', 'pending', NOW())
ON CONFLICT (node_id) WHERE deleted_at IS NULL
DO UPDATE SET runtime = EXCLUDED.runtime, listen_host = EXCLUDED.listen_host, listen_port = EXCLUDED.listen_port,
              status = 'pending', updated_at = NOW()`,
		source.ID, nodeID, defaultString(source.Runtime, "sing-box"), sidecarListenHost(), port)
	return err
}

func (s *adminServiceImpl) refreshProxySidecarEndpointReadiness(ctx context.Context, nodeID, proxyID int64, port int) error {
	endpointStatus := "pending"
	proxyStatus := StatusDisabled
	host := sidecarProbeHost()
	lastError := fmt.Sprintf("sidecar endpoint %s:%d is not ready", host, port)
	lastStartedAt := any(nil)
	if isLocalTCPPortReachable(ctx, host, port) {
		endpointStatus = "ready"
		proxyStatus = StatusActive
		lastError = ""
		lastStartedAt = time.Now()
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
	defer func() { _ = rows.Close() }()
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
		for _, candidate := range []string{
			filepath.Join(current, "key.md"),
			filepath.Join(current, "sub2api", "key.md"),
			filepath.Join(current, "..", "sub2api", "key.md"),
		} {
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
	client, err := httpclient.GetClient(httpclient.Options{
		Timeout:               15 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	})
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
	defer func() { _ = resp.Body.Close() }()

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
	return &proxyIPReputationResult{
		IP:          defaultString(payload.Data.IPAddress, ipAddress),
		CleanScore:  cleanScore,
		Country:     payload.Data.CountryName,
		CountryCode: payload.Data.CountryCode,
		Provider:    "abuseipdb",
		CheckedAt:   time.Now(),
		Raw:         raw,
	}, nil
}

func (s *adminServiceImpl) getCachedProxyIPReputation(ctx context.Context, ipAddress, provider string) (*proxyIPReputationResult, error) {
	rows, err := s.entClient.QueryContext(ctx, `
SELECT clean_score, raw::text, checked_at
FROM proxy_ip_reputation_cache
WHERE ip = $1 AND provider = $2 AND expires_at > NOW()`, ipAddress, provider)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
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
DO UPDATE SET clean_score = EXCLUDED.clean_score, raw = EXCLUDED.raw, checked_at = EXCLUDED.checked_at, expires_at = EXCLUDED.expires_at`,
		result.IP, result.Provider, result.CleanScore, string(raw), result.CheckedAt, result.CheckedAt.Add(time.Duration(cacheHours)*time.Hour))
	return err
}

func (s *adminServiceImpl) upsertDirectProxyFromSubscriptionNode(ctx context.Context, source *ProxySubscriptionSource, item ProxyImportPreviewItem, evaluation proxySubscriptionNodeEvaluation) error {
	proxy, exists, err := s.findProxyByAddress(ctx, item.Host, item.Port, item.Username, item.Password)
	if err != nil {
		return err
	}
	qualityStatus := ProxyQualityHealthy
	if (evaluation.IPCleanScore != nil && *evaluation.IPCleanScore < 50) ||
		evaluation.LastError != "" ||
		(evaluation.LatencyMs != nil && *evaluation.LatencyMs > 1500) {
		qualityStatus = ProxyQualityDegraded
	}
	name := strings.TrimSpace(item.Name)
	if name == "" {
		name = fmt.Sprintf("%s:%d", item.Host, item.Port)
	}
	if !exists {
		_, err := s.CreateProxy(ctx, &CreateProxyInput{
			Name:          name,
			Protocol:      item.Protocol,
			Host:          item.Host,
			Port:          item.Port,
			Username:      item.Username,
			Password:      item.Password,
			Source:        "subscription",
			ProxyType:     defaultString(item.ProxyType, "datacenter"),
			Provider:      source.Provider,
			Region:        defaultString(evaluation.ExitCountryCode, evaluation.Country),
			ExitIP:        evaluation.ExitIP,
			QualityStatus: qualityStatus,
			Weight:        maxInt(1, evaluation.Score),
		})
		return err
	}
	proxy.Name = name
	proxy.Protocol = item.Protocol
	proxy.Host = item.Host
	proxy.Port = item.Port
	proxy.Username = item.Username
	proxy.Password = item.Password
	proxy.Status = StatusActive
	applyProxyUpdateMetadata(proxy, &UpdateProxyInput{
		Source:        "subscription",
		ProxyType:     defaultString(item.ProxyType, "datacenter"),
		Provider:      source.Provider,
		Region:        defaultString(evaluation.ExitCountryCode, evaluation.Country),
		ExitIP:        evaluation.ExitIP,
		QualityStatus: qualityStatus,
		Weight:        intPtr(maxInt(1, evaluation.Score)),
	})
	_, err = s.UpdateProxy(ctx, proxy.ID, &UpdateProxyInput{
		Name:          proxy.Name,
		Protocol:      proxy.Protocol,
		Host:          proxy.Host,
		Port:          proxy.Port,
		Username:      proxy.Username,
		Password:      proxy.Password,
		Status:        StatusActive,
		Source:        proxy.Source,
		ProxyType:     proxy.ProxyType,
		Provider:      proxy.Provider,
		Region:        proxy.Region,
		ExitIP:        proxy.ExitIP,
		QualityStatus: proxy.QualityStatus,
		Weight:        intPtr(proxy.Weight),
	})
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
		proxy, err = s.CreateProxy(ctx, &CreateProxyInput{
			Name:          proxyName,
			Protocol:      "socks5",
			Host:          proxyHost,
			Port:          port,
			Source:        "subscription",
			ProxyType:     "sidecar",
			Provider:      source.Provider,
			Region:        defaultString(evaluation.ExitCountryCode, evaluation.Country),
			QualityStatus: qualityStatus,
			Weight:        maxInt(1, evaluation.Score),
		})
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
	if _, err := s.UpdateProxy(ctx, proxy.ID, &UpdateProxyInput{
		Name:          proxyName,
		Protocol:      "socks5",
		Host:          proxyHost,
		Port:          port,
		Status:        proxy.Status,
		Source:        "subscription",
		ProxyType:     "sidecar",
		Provider:      source.Provider,
		Region:        defaultString(evaluation.ExitCountryCode, evaluation.Country),
		ExitIP:        evaluation.ExitIP,
		QualityStatus: qualityStatus,
		Weight:        intPtr(maxInt(1, evaluation.Score)),
	}); err != nil {
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
	defer func() { _ = rows.Close() }()
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

// Redeem code management implementations
func (s *adminServiceImpl) ListRedeemCodes(ctx context.Context, page, pageSize int, codeType, status, search string, sortBy, sortOrder string) ([]RedeemCode, int64, error) {
	params := pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: sortBy, SortOrder: sortOrder}
	codes, result, err := s.redeemCodeRepo.ListWithFilters(ctx, params, codeType, status, search)
	if err != nil {
		return nil, 0, err
	}
	return codes, result.Total, nil
}

func (s *adminServiceImpl) GetRedeemCode(ctx context.Context, id int64) (*RedeemCode, error) {
	return s.redeemCodeRepo.GetByID(ctx, id)
}

func (s *adminServiceImpl) GenerateRedeemCodes(ctx context.Context, input *GenerateRedeemCodesInput) ([]RedeemCode, error) {
	// 如果是订阅类型，验证必须有 GroupID
	if input.Type == RedeemTypeSubscription {
		if input.GroupID == nil {
			return nil, errors.New("group_id is required for subscription type")
		}
		// 验证分组存在且为订阅类型
		group, err := s.groupRepo.GetByID(ctx, *input.GroupID)
		if err != nil {
			return nil, fmt.Errorf("group not found: %w", err)
		}
		if !group.IsSubscriptionType() {
			return nil, errors.New("group must be subscription type")
		}
	}

	codes := make([]RedeemCode, 0, input.Count)
	for i := 0; i < input.Count; i++ {
		codeValue, err := GenerateRedeemCode()
		if err != nil {
			return nil, err
		}
		code := RedeemCode{
			Code:   codeValue,
			Type:   input.Type,
			Value:  input.Value,
			Status: StatusUnused,
		}
		// 订阅类型专用字段
		if input.Type == RedeemTypeSubscription {
			code.GroupID = input.GroupID
			code.ValidityDays = input.ValidityDays
			if code.ValidityDays <= 0 {
				code.ValidityDays = 30 // 默认30天
			}
		}
		if err := s.redeemCodeRepo.Create(ctx, &code); err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, nil
}

func (s *adminServiceImpl) DeleteRedeemCode(ctx context.Context, id int64) error {
	return s.redeemCodeRepo.Delete(ctx, id)
}

func (s *adminServiceImpl) BatchDeleteRedeemCodes(ctx context.Context, ids []int64) (int64, error) {
	var deleted int64
	for _, id := range ids {
		if err := s.redeemCodeRepo.Delete(ctx, id); err == nil {
			deleted++
		}
	}
	return deleted, nil
}

func (s *adminServiceImpl) ExpireRedeemCode(ctx context.Context, id int64) (*RedeemCode, error) {
	code, err := s.redeemCodeRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	code.Status = StatusExpired
	if err := s.redeemCodeRepo.Update(ctx, code); err != nil {
		return nil, err
	}
	return code, nil
}

func (s *adminServiceImpl) TestProxy(ctx context.Context, id int64) (*ProxyTestResult, error) {
	proxy, err := s.proxyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	proxyURL := proxy.URL()
	exitInfo, latencyMs, err := s.proxyProber.ProbeProxy(ctx, proxyURL)
	if err != nil {
		s.saveProxyLatency(ctx, id, &ProxyLatencyInfo{
			Success:   false,
			Message:   err.Error(),
			UpdatedAt: time.Now(),
		})
		return &ProxyTestResult{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	latency := latencyMs
	s.saveProxyLatency(ctx, id, &ProxyLatencyInfo{
		Success:     true,
		LatencyMs:   &latency,
		Message:     "Proxy is accessible",
		IPAddress:   exitInfo.IP,
		Country:     exitInfo.Country,
		CountryCode: exitInfo.CountryCode,
		Region:      exitInfo.Region,
		City:        exitInfo.City,
		UpdatedAt:   time.Now(),
	})
	return &ProxyTestResult{
		Success:     true,
		Message:     "Proxy is accessible",
		LatencyMs:   latencyMs,
		IPAddress:   exitInfo.IP,
		City:        exitInfo.City,
		Region:      exitInfo.Region,
		Country:     exitInfo.Country,
		CountryCode: exitInfo.CountryCode,
	}, nil
}

func (s *adminServiceImpl) CheckProxyQuality(ctx context.Context, id int64) (*ProxyQualityCheckResult, error) {
	proxy, err := s.proxyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	result := &ProxyQualityCheckResult{
		ProxyID:   id,
		Score:     100,
		Grade:     "A",
		CheckedAt: time.Now().Unix(),
		Items:     make([]ProxyQualityCheckItem, 0, len(proxyQualityTargets)+1),
	}

	proxyURL := proxy.URL()
	if s.proxyProber == nil {
		result.Items = append(result.Items, ProxyQualityCheckItem{
			Target:  "base_connectivity",
			Status:  "fail",
			Message: "代理探测服务未配置",
		})
		result.FailedCount++
		finalizeProxyQualityResult(result)
		s.saveProxyQualitySnapshot(ctx, id, result, nil)
		return result, nil
	}

	exitInfo, latencyMs, err := s.proxyProber.ProbeProxy(ctx, proxyURL)
	if err != nil {
		result.Items = append(result.Items, ProxyQualityCheckItem{
			Target:    "base_connectivity",
			Status:    "fail",
			LatencyMs: latencyMs,
			Message:   err.Error(),
		})
		result.FailedCount++
		finalizeProxyQualityResult(result)
		s.saveProxyQualitySnapshot(ctx, id, result, nil)
		return result, nil
	}

	result.ExitIP = exitInfo.IP
	result.Country = exitInfo.Country
	result.CountryCode = exitInfo.CountryCode
	result.BaseLatencyMs = latencyMs
	result.Items = append(result.Items, ProxyQualityCheckItem{
		Target:    "base_connectivity",
		Status:    "pass",
		LatencyMs: latencyMs,
		Message:   "代理出口连通正常",
	})
	result.PassedCount++

	client, err := httpclient.GetClient(httpclient.Options{
		ProxyURL:              proxyURL,
		Timeout:               proxyQualityRequestTimeout,
		ResponseHeaderTimeout: proxyQualityResponseHeaderTimeout,
	})
	if err != nil {
		result.Items = append(result.Items, ProxyQualityCheckItem{
			Target:  "http_client",
			Status:  "fail",
			Message: fmt.Sprintf("创建检测客户端失败: %v", err),
		})
		result.FailedCount++
		finalizeProxyQualityResult(result)
		s.saveProxyQualitySnapshot(ctx, id, result, exitInfo)
		return result, nil
	}

	for _, target := range proxyQualityTargets {
		item := runProxyQualityTarget(ctx, client, target)
		result.Items = append(result.Items, item)
		switch item.Status {
		case "pass":
			result.PassedCount++
		case "warn":
			result.WarnCount++
		case "challenge":
			result.ChallengeCount++
		default:
			result.FailedCount++
		}
	}

	finalizeProxyQualityResult(result)
	s.saveProxyQualitySnapshot(ctx, id, result, exitInfo)
	return result, nil
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

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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
	defer func() { _ = rows.Close() }()
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
WHERE id = $1`,
		id,
		defaultString(proxy.Source, "manual"),
		defaultString(proxy.ProxyType, "datacenter"),
		proxy.Provider,
		proxy.Region,
		proxy.ExitIP,
		normalizeProxyQualityStatus(proxy.QualityStatus),
		proxy.MaxBoundAccounts,
		proxy.MaxActiveAccounts,
		positiveOrDefaultInt(proxy.Weight, 100),
		proxy.FailureCount,
	)
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
	cred := func(key string) string { return strings.TrimSpace(account.GetCredential(key)) }
	lowerCred := func(key string) string { return strings.ToLower(cred(key)) }
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
	defer func() { _ = rows.Close() }()
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
	defer func() { _ = rows.Close() }()
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
	defer func() { _ = rows.Close() }()
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
              updated_at = NOW()`,
		identityKey, account.Platform, account.ID, proxyID, status, source)
	return err
}

func (s *adminServiceImpl) deactivateAccountProxyBindings(ctx context.Context, account *Account) error {
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
	defer func() { _ = rows.Close() }()
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
	rel := &ProxyRelationship{
		AccountID:     account.ID,
		AccountName:   account.Name,
		Platform:      account.Platform,
		AccountType:   account.Type,
		AccountStatus: account.Status,
		IdentityKey:   identityKey,
		ProxySource:   ProxyBindingSourceAuto,
	}
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
	defer func() { _ = rows.Close() }()
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
			item.Valid = true
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
		item := ProxyImportPreviewItem{
			Name:          strings.TrimSpace(fmt.Sprint(p["name"])),
			Protocol:      typ,
			Host:          strings.TrimSpace(fmt.Sprint(p["server"])),
			Username:      strings.TrimSpace(fmt.Sprint(p["username"])),
			Password:      strings.TrimSpace(fmt.Sprint(p["password"])),
			Provider:      provider,
			Source:        "clash",
			QualityStatus: ProxyQualityHealthy,
		}
		item.Port, _ = strconv.Atoi(fmt.Sprint(p["port"]))
		switch typ {
		case "http", "https", "socks5", "socks5h":
			item.ProxyType = "direct"
			item.Valid = item.Host != "" && item.Port > 0
		default:
			item.ProxyType = "sidecar"
			item.SidecarRequired = true
			item.SidecarHint = "Clash/Mihomo 节点需要通过 sidecar 暴露本地 http/socks5 出口"
			item.Valid = true
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
		item := ProxyImportPreviewItem{
			Name:          strings.TrimSpace(fmt.Sprint(o["tag"])),
			Protocol:      typ,
			Host:          strings.TrimSpace(fmt.Sprint(o["server"])),
			Provider:      provider,
			Source:        "sing-box",
			QualityStatus: ProxyQualityHealthy,
		}
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
			item.Valid = true
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
		return "", infraerrors.BadRequest(
			"PROXY_SUBSCRIPTION_FETCH_FAILED",
			subscriptionFetchErrorMessage(parsedURL.Host, err),
		).WithCause(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", infraerrors.BadRequest(
			"PROXY_SUBSCRIPTION_FETCH_FAILED",
			fmt.Sprintf("subscription URL returned HTTP %d", resp.StatusCode),
		)
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
	return ProxySubscriptionStrategy{
		MaxParsedNodes:          300,
		MaxEnabledNodes:         30,
		MaxActiveSidecarNodes:   3,
		MaxProbeConcurrency:     1,
		ScanBatchSize:           5,
		StandbyNodes:            10,
		MinCountryCount:         3,
		MaxCountryCount:         8,
		MaxNodesPerCountry:      5,
		MaxLatencyMs:            1200,
		MinIPCleanScore:         70,
		MinQualityScore:         65,
		SelectionMode:           "balanced",
		ReputationCacheHours:    24,
		ScanBudgetMinutes:       30,
		ScanBudgetMaxMinutes:    40,
		ResourceAdaptiveScan:    true,
		MinFreeMemoryMB:         800,
		PauseFreeMemoryMB:       500,
		TimeoutSleepAfter:       3,
		SleepMinutes:            60,
		ReplaceSameCountryFirst: true,
	}
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
	type timeoutError interface {
		Timeout() bool
	}
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

func runProxyQualityTarget(ctx context.Context, client *http.Client, target proxyQualityTarget) ProxyQualityCheckItem {
	item := ProxyQualityCheckItem{
		Target: target.Target,
	}

	req, err := http.NewRequestWithContext(ctx, target.Method, target.URL, nil)
	if err != nil {
		item.Status = "fail"
		item.Message = fmt.Sprintf("构建请求失败: %v", err)
		return item
	}
	req.Header.Set("Accept", "application/json,text/html,*/*")
	req.Header.Set("User-Agent", proxyQualityClientUserAgent)

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		item.Status = "fail"
		item.LatencyMs = time.Since(start).Milliseconds()
		item.Message = fmt.Sprintf("请求失败: %v", err)
		return item
	}
	defer func() { _ = resp.Body.Close() }()
	item.LatencyMs = time.Since(start).Milliseconds()
	item.HTTPStatus = resp.StatusCode

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, proxyQualityMaxBodyBytes+1))
	if readErr != nil {
		item.Status = "fail"
		item.Message = fmt.Sprintf("读取响应失败: %v", readErr)
		return item
	}
	if int64(len(body)) > proxyQualityMaxBodyBytes {
		body = body[:proxyQualityMaxBodyBytes]
	}

	// Cloudflare challenge 检测
	if httputil.IsCloudflareChallengeResponse(resp.StatusCode, resp.Header, body) {
		item.Status = "challenge"
		item.CFRay = httputil.ExtractCloudflareRayID(resp.Header, body)
		item.Message = "命中 Cloudflare challenge"
		return item
	}

	if _, ok := target.AllowedStatuses[resp.StatusCode]; ok {
		if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			item.Status = "pass"
			item.Message = fmt.Sprintf("HTTP %d", resp.StatusCode)
		} else {
			item.Status = "warn"
			item.Message = fmt.Sprintf("HTTP %d（目标可达，但鉴权或方法受限）", resp.StatusCode)
		}
		return item
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		item.Status = "warn"
		item.Message = "目标返回 429，可能存在频控"
		return item
	}

	item.Status = "fail"
	item.Message = fmt.Sprintf("非预期状态码: %d", resp.StatusCode)
	return item
}

func finalizeProxyQualityResult(result *ProxyQualityCheckResult) {
	if result == nil {
		return
	}
	score := 100 - result.WarnCount*10 - result.FailedCount*22 - result.ChallengeCount*30
	if score < 0 {
		score = 0
	}
	result.Score = score
	result.Grade = proxyQualityGrade(score)
	result.Summary = fmt.Sprintf(
		"通过 %d 项，告警 %d 项，失败 %d 项，挑战 %d 项",
		result.PassedCount,
		result.WarnCount,
		result.FailedCount,
		result.ChallengeCount,
	)
}

func proxyQualityGrade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 75:
		return "B"
	case score >= 60:
		return "C"
	case score >= 40:
		return "D"
	default:
		return "F"
	}
}

func proxyQualityOverallStatus(result *ProxyQualityCheckResult) string {
	if result == nil {
		return ""
	}
	if result.ChallengeCount > 0 {
		return "challenge"
	}
	if result.FailedCount > 0 {
		return "failed"
	}
	if result.WarnCount > 0 {
		return "warn"
	}
	if result.PassedCount > 0 {
		return "healthy"
	}
	return "failed"
}

func proxyQualityFirstCFRay(result *ProxyQualityCheckResult) string {
	if result == nil {
		return ""
	}
	for _, item := range result.Items {
		if item.CFRay != "" {
			return item.CFRay
		}
	}
	return ""
}

func proxyQualityBaseConnectivityPass(result *ProxyQualityCheckResult) bool {
	if result == nil {
		return false
	}
	for _, item := range result.Items {
		if item.Target == "base_connectivity" {
			return item.Status == "pass"
		}
	}
	return false
}

func (s *adminServiceImpl) saveProxyQualitySnapshot(ctx context.Context, proxyID int64, result *ProxyQualityCheckResult, exitInfo *ProxyExitInfo) {
	if result == nil {
		return
	}
	score := result.Score
	checkedAt := result.CheckedAt
	info := &ProxyLatencyInfo{
		Success:          proxyQualityBaseConnectivityPass(result),
		Message:          result.Summary,
		QualityStatus:    proxyQualityOverallStatus(result),
		QualityScore:     &score,
		QualityGrade:     result.Grade,
		QualitySummary:   result.Summary,
		QualityCheckedAt: &checkedAt,
		QualityCFRay:     proxyQualityFirstCFRay(result),
		UpdatedAt:        time.Now(),
	}
	if result.BaseLatencyMs > 0 {
		latency := result.BaseLatencyMs
		info.LatencyMs = &latency
	}
	if exitInfo != nil {
		info.IPAddress = exitInfo.IP
		info.Country = exitInfo.Country
		info.CountryCode = exitInfo.CountryCode
		info.Region = exitInfo.Region
		info.City = exitInfo.City
	}
	if s != nil && s.entClient != nil {
		qualityStatus := normalizeProxyQualityStatus(info.QualityStatus)
		exitIP := ""
		region := ""
		if exitInfo != nil {
			exitIP = exitInfo.IP
			region = exitInfo.Region
		}
		if result.FailedCount > 0 && result.PassedCount == 0 {
			_, _ = s.entClient.ExecContext(ctx, `UPDATE proxies SET quality_status = $2, exit_ip = NULLIF($3, ''), region = NULLIF($4, ''), last_checked_at = NOW(), failure_count = failure_count + 1 WHERE id = $1`, proxyID, qualityStatus, exitIP, region)
		} else {
			_, _ = s.entClient.ExecContext(ctx, `UPDATE proxies SET quality_status = $2, exit_ip = NULLIF($3, ''), region = NULLIF($4, ''), last_checked_at = NOW(), failure_count = 0 WHERE id = $1`, proxyID, qualityStatus, exitIP, region)
		}
	}
	s.saveProxyLatency(ctx, proxyID, info)
}

func (s *adminServiceImpl) probeProxyLatency(ctx context.Context, proxy *Proxy) {
	if s.proxyProber == nil || proxy == nil {
		return
	}
	exitInfo, latencyMs, err := s.proxyProber.ProbeProxy(ctx, proxy.URL())
	if err != nil {
		s.saveProxyLatency(ctx, proxy.ID, &ProxyLatencyInfo{
			Success:   false,
			Message:   err.Error(),
			UpdatedAt: time.Now(),
		})
		return
	}

	latency := latencyMs
	s.saveProxyLatency(ctx, proxy.ID, &ProxyLatencyInfo{
		Success:     true,
		LatencyMs:   &latency,
		Message:     "Proxy is accessible",
		IPAddress:   exitInfo.IP,
		Country:     exitInfo.Country,
		CountryCode: exitInfo.CountryCode,
		Region:      exitInfo.Region,
		City:        exitInfo.City,
		UpdatedAt:   time.Now(),
	})
}

// checkMixedChannelRisk 检查分组中是否存在混合渠道（Antigravity + Anthropic）
// 如果存在混合，返回错误提示用户确认
func (s *adminServiceImpl) checkMixedChannelRisk(ctx context.Context, currentAccountID int64, currentAccountPlatform string, groupIDs []int64) error {
	// 判断当前账号的渠道类型（基于 platform 字段，而不是 type 字段）
	currentPlatform := getAccountPlatform(currentAccountPlatform)
	if currentPlatform == "" {
		// 不是 Antigravity 或 Anthropic，无需检查
		return nil
	}

	// 检查每个分组中的其他账号
	for _, groupID := range groupIDs {
		accounts, err := s.accountRepo.ListByGroup(ctx, groupID)
		if err != nil {
			return fmt.Errorf("get accounts in group %d: %w", groupID, err)
		}

		// 检查是否存在不同渠道的账号
		for _, account := range accounts {
			if currentAccountID > 0 && account.ID == currentAccountID {
				continue // 跳过当前账号
			}

			otherPlatform := getAccountPlatform(account.Platform)
			if otherPlatform == "" {
				continue // 不是 Antigravity 或 Anthropic，跳过
			}

			// 检测混合渠道
			if currentPlatform != otherPlatform {
				group, _ := s.groupRepo.GetByID(ctx, groupID)
				groupName := fmt.Sprintf("Group %d", groupID)
				if group != nil {
					groupName = group.Name
				}

				return &MixedChannelError{
					GroupID:         groupID,
					GroupName:       groupName,
					CurrentPlatform: currentPlatform,
					OtherPlatform:   otherPlatform,
				}
			}
		}
	}

	return nil
}

func (s *adminServiceImpl) validateGroupIDsExist(ctx context.Context, groupIDs []int64) error {
	if len(groupIDs) == 0 {
		return nil
	}
	if s.groupRepo == nil {
		return errors.New("group repository not configured")
	}

	if batchReader, ok := s.groupRepo.(groupExistenceBatchReader); ok {
		existsByID, err := batchReader.ExistsByIDs(ctx, groupIDs)
		if err != nil {
			return fmt.Errorf("check groups exists: %w", err)
		}
		for _, groupID := range groupIDs {
			if groupID <= 0 || !existsByID[groupID] {
				return fmt.Errorf("get group: %w", ErrGroupNotFound)
			}
		}
		return nil
	}

	for _, groupID := range groupIDs {
		if _, err := s.groupRepo.GetByID(ctx, groupID); err != nil {
			return fmt.Errorf("get group: %w", err)
		}
	}
	return nil
}

// CheckMixedChannelRisk checks whether target groups contain mixed channels for the current account platform.
func (s *adminServiceImpl) CheckMixedChannelRisk(ctx context.Context, currentAccountID int64, currentAccountPlatform string, groupIDs []int64) error {
	return s.checkMixedChannelRisk(ctx, currentAccountID, currentAccountPlatform, groupIDs)
}

func (s *adminServiceImpl) attachProxyLatency(ctx context.Context, proxies []ProxyWithAccountCount) {
	if s.proxyLatencyCache == nil || len(proxies) == 0 {
		return
	}

	ids := make([]int64, 0, len(proxies))
	for i := range proxies {
		ids = append(ids, proxies[i].ID)
	}

	latencies, err := s.proxyLatencyCache.GetProxyLatencies(ctx, ids)
	if err != nil {
		logger.LegacyPrintf("service.admin", "Warning: load proxy latency cache failed: %v", err)
		return
	}

	for i := range proxies {
		info := latencies[proxies[i].ID]
		if info == nil {
			continue
		}
		if info.Success {
			proxies[i].LatencyStatus = "success"
			proxies[i].LatencyMs = info.LatencyMs
		} else {
			proxies[i].LatencyStatus = "failed"
		}
		proxies[i].LatencyMessage = info.Message
		proxies[i].IPAddress = info.IPAddress
		proxies[i].Country = info.Country
		proxies[i].CountryCode = info.CountryCode
		proxies[i].Region = info.Region
		proxies[i].City = info.City
		proxies[i].QualityStatus = info.QualityStatus
		proxies[i].QualityScore = info.QualityScore
		proxies[i].QualityGrade = info.QualityGrade
		proxies[i].QualitySummary = info.QualitySummary
		proxies[i].QualityChecked = info.QualityCheckedAt
	}
}

func (s *adminServiceImpl) saveProxyLatency(ctx context.Context, proxyID int64, info *ProxyLatencyInfo) {
	if s == nil || s.proxyLatencyCache == nil || info == nil {
		if s != nil && s.entClient != nil && info != nil {
			status := ProxyQualityHealthy
			if !info.Success {
				status = ProxyQualityDegraded
			}
			_, _ = s.entClient.ExecContext(ctx, `UPDATE proxies SET quality_status = $2, exit_ip = NULLIF($3, ''), region = NULLIF($4, ''), last_checked_at = NOW(), failure_count = CASE WHEN $5 THEN 0 ELSE failure_count + 1 END WHERE id = $1`, proxyID, status, info.IPAddress, info.Region, info.Success)
		}
		return
	}

	merged := *info
	if latencies, err := s.proxyLatencyCache.GetProxyLatencies(ctx, []int64{proxyID}); err == nil {
		if existing := latencies[proxyID]; existing != nil {
			if merged.QualityCheckedAt == nil &&
				merged.QualityScore == nil &&
				merged.QualityGrade == "" &&
				merged.QualityStatus == "" &&
				merged.QualitySummary == "" &&
				merged.QualityCFRay == "" {
				merged.QualityStatus = existing.QualityStatus
				merged.QualityScore = existing.QualityScore
				merged.QualityGrade = existing.QualityGrade
				merged.QualitySummary = existing.QualitySummary
				merged.QualityCheckedAt = existing.QualityCheckedAt
				merged.QualityCFRay = existing.QualityCFRay
			}
		}
	}

	if err := s.proxyLatencyCache.SetProxyLatency(ctx, proxyID, &merged); err != nil {
		logger.LegacyPrintf("service.admin", "Warning: store proxy latency cache failed: %v", err)
	}
	if s.entClient != nil {
		status := ProxyQualityHealthy
		if !merged.Success {
			status = ProxyQualityDegraded
		}
		if merged.QualityStatus != "" {
			status = normalizeProxyQualityStatus(merged.QualityStatus)
		}
		_, _ = s.entClient.ExecContext(ctx, `UPDATE proxies SET quality_status = $2, exit_ip = NULLIF($3, ''), region = NULLIF($4, ''), last_checked_at = NOW(), failure_count = CASE WHEN $5 THEN 0 ELSE failure_count + 1 END WHERE id = $1`, proxyID, status, merged.IPAddress, merged.Region, merged.Success)
	}
}

// getAccountPlatform 根据账号 platform 判断混合渠道检查用的平台标识
func getAccountPlatform(accountPlatform string) string {
	switch strings.ToLower(strings.TrimSpace(accountPlatform)) {
	case PlatformAntigravity:
		return "Antigravity"
	case PlatformAnthropic, "claude":
		return "Anthropic"
	default:
		return ""
	}
}

// MixedChannelError 混合渠道错误
type MixedChannelError struct {
	GroupID         int64
	GroupName       string
	CurrentPlatform string
	OtherPlatform   string
}

func (e *MixedChannelError) Error() string {
	return fmt.Sprintf("mixed_channel_warning: Group '%s' contains both %s and %s accounts. Using mixed channels in the same context may cause thinking block signature validation issues, which will fallback to non-thinking mode for historical messages.",
		e.GroupName, e.CurrentPlatform, e.OtherPlatform)
}

func (s *adminServiceImpl) ResetAccountQuota(ctx context.Context, id int64) error {
	return s.accountRepo.ResetQuotaUsed(ctx, id)
}

// EnsureOpenAIPrivacy 检查 OpenAI OAuth 账号是否已设置 privacy_mode，
// 未设置则调用 disableOpenAITraining 并持久化到 Extra，返回设置的 mode 值。
func (s *adminServiceImpl) EnsureOpenAIPrivacy(ctx context.Context, account *Account) string {
	if account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth {
		return ""
	}
	if s.privacyClientFactory == nil {
		return ""
	}
	if shouldSkipOpenAIPrivacyEnsure(account.Extra) {
		return ""
	}

	token, _ := account.Credentials["access_token"].(string)
	if token == "" {
		return ""
	}

	var proxyURL string
	if account.ProxyID != nil {
		if p, err := s.proxyRepo.GetByID(ctx, *account.ProxyID); err == nil && p != nil {
			proxyURL = p.URL()
		}
	}

	mode := disableOpenAITraining(ctx, s.privacyClientFactory, token, proxyURL)
	if mode == "" {
		return ""
	}

	_ = s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{"privacy_mode": mode})
	return mode
}

// ForceOpenAIPrivacy 强制重新设置 OpenAI OAuth 账号隐私，无论当前状态。
func (s *adminServiceImpl) ForceOpenAIPrivacy(ctx context.Context, account *Account) string {
	if account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth {
		return ""
	}
	if s.privacyClientFactory == nil {
		return ""
	}

	token, _ := account.Credentials["access_token"].(string)
	if token == "" {
		return ""
	}

	var proxyURL string
	if account.ProxyID != nil {
		if p, err := s.proxyRepo.GetByID(ctx, *account.ProxyID); err == nil && p != nil {
			proxyURL = p.URL()
		}
	}

	mode := disableOpenAITraining(ctx, s.privacyClientFactory, token, proxyURL)
	if mode == "" {
		return ""
	}

	if err := s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{"privacy_mode": mode}); err != nil {
		logger.LegacyPrintf("service.admin", "force_update_openai_privacy_mode_failed: account_id=%d err=%v", account.ID, err)
		return mode
	}
	if account.Extra == nil {
		account.Extra = make(map[string]any)
	}
	account.Extra["privacy_mode"] = mode
	return mode
}

// EnsureAntigravityPrivacy 检查 Antigravity OAuth 账号隐私状态。
// 仅当 privacy_mode 已成功设置（"privacy_set"）时跳过；
// 未设置或之前失败（"privacy_set_failed"）均会重试。
func (s *adminServiceImpl) EnsureAntigravityPrivacy(ctx context.Context, account *Account) string {
	if account.Platform != PlatformAntigravity || account.Type != AccountTypeOAuth {
		return ""
	}
	if account.Extra != nil {
		if existing, ok := account.Extra["privacy_mode"].(string); ok && existing == AntigravityPrivacySet {
			return existing
		}
	}

	token, _ := account.Credentials["access_token"].(string)
	if token == "" {
		return ""
	}

	projectID, _ := account.Credentials["project_id"].(string)

	var proxyURL string
	if account.ProxyID != nil {
		if p, err := s.proxyRepo.GetByID(ctx, *account.ProxyID); err == nil && p != nil {
			proxyURL = p.URL()
		}
	}

	mode := setAntigravityPrivacy(ctx, token, projectID, proxyURL)
	if mode == "" {
		return ""
	}

	if err := s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{"privacy_mode": mode}); err != nil {
		logger.LegacyPrintf("service.admin", "update_antigravity_privacy_mode_failed: account_id=%d err=%v", account.ID, err)
		return mode
	}
	applyAntigravityPrivacyMode(account, mode)
	return mode
}

// ForceAntigravityPrivacy 强制重新设置 Antigravity OAuth 账号隐私，无论当前状态。
func (s *adminServiceImpl) ForceAntigravityPrivacy(ctx context.Context, account *Account) string {
	if account.Platform != PlatformAntigravity || account.Type != AccountTypeOAuth {
		return ""
	}

	token, _ := account.Credentials["access_token"].(string)
	if token == "" {
		return ""
	}

	projectID, _ := account.Credentials["project_id"].(string)

	var proxyURL string
	if account.ProxyID != nil {
		if p, err := s.proxyRepo.GetByID(ctx, *account.ProxyID); err == nil && p != nil {
			proxyURL = p.URL()
		}
	}

	mode := setAntigravityPrivacy(ctx, token, projectID, proxyURL)
	if mode == "" {
		return ""
	}

	if err := s.accountRepo.UpdateExtra(ctx, account.ID, map[string]any{"privacy_mode": mode}); err != nil {
		logger.LegacyPrintf("service.admin", "force_update_antigravity_privacy_mode_failed: account_id=%d err=%v", account.ID, err)
		return mode
	}
	applyAntigravityPrivacyMode(account, mode)
	return mode
}
