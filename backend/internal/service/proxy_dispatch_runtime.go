package service

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/gin-gonic/gin"
)

const (
	proxyDispatchErrorOpenAI    = "openai"
	proxyDispatchErrorAnthropic = "anthropic"
	proxyDispatchErrorGeneric   = "generic"
)

func resolveRuntimeProxyURL(ctx context.Context, account *Account, settingService *SettingService) (string, error) {
	if account != nil && account.ProxyID != nil && account.Proxy != nil {
		return account.Proxy.URL(), nil
	}
	mode := runtimeDirectFallbackMode(ctx, settingService)
	if mode == DirectFallbackGlobal {
		return "", nil
	}
	return "", noAvailableProxyError(account, mode)
}

func resolveRuntimeProxyURLAllowCustomRelay(ctx context.Context, account *Account, settingService *SettingService) (string, error) {
	if account != nil && account.IsCustomBaseURLEnabled() && account.GetCustomBaseURL() != "" {
		return "", nil
	}
	return resolveRuntimeProxyURL(ctx, account, settingService)
}

func runtimeDirectFallbackMode(ctx context.Context, settingService *SettingService) string {
	// Keep legacy in-memory unit-test services working when they do not wire settings.
	if settingService == nil || settingService.settingRepo == nil {
		return DirectFallbackGlobal
	}
	raw, err := settingService.settingRepo.GetValue(ctx, SettingKeyProxyDispatchSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DirectFallbackGlobal
		}
		return DirectFallbackOff
	}
	var settings ProxyDispatchSettings
	if strings.TrimSpace(raw) == "" || json.Unmarshal([]byte(raw), &settings) != nil {
		return DirectFallbackOff
	}
	return normalizeDirectFallbackMode(settings.DirectFallbackMode)
}

func noAvailableProxyError(account *Account, mode string) *infraerrors.ApplicationError {
	md := map[string]string{
		"direct_fallback_mode": normalizeDirectFallbackMode(mode),
	}
	if account != nil {
		md["account_id"] = strconv.FormatInt(account.ID, 10)
		md["platform"] = account.Platform
	}
	return infraerrors.ServiceUnavailable("NO_AVAILABLE_PROXY", "no available proxy and direct fallback is disabled").WithMetadata(md)
}

func writeProxyDispatchError(c *gin.Context, err error, format string) bool {
	if err == nil || c == nil || c.Writer == nil || c.Writer.Written() {
		return false
	}
	statusCode, status := infraerrors.ToHTTP(err)
	if status.Reason != "NO_AVAILABLE_PROXY" {
		return false
	}
	switch format {
	case proxyDispatchErrorOpenAI:
		c.JSON(statusCode, gin.H{
			"error": gin.H{
				"type":    "server_error",
				"code":    status.Reason,
				"message": status.Message,
			},
		})
	case proxyDispatchErrorAnthropic:
		c.JSON(statusCode, gin.H{
			"type": "error",
			"error": gin.H{
				"type":    "api_error",
				"message": status.Message,
			},
		})
	default:
		c.JSON(statusCode, gin.H{
			"code":     statusCode,
			"reason":   status.Reason,
			"message":  status.Message,
			"metadata": status.Metadata,
		})
	}
	return true
}

func handleProxyDispatchError(c *gin.Context, err error, format string) error {
	if writeProxyDispatchError(c, err, format) {
		return err
	}
	return err
}
