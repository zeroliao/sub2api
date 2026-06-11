//go:build unit

package service

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type proxySubscriptionSettingRepoStub struct {
	values map[string]string
}

func (r *proxySubscriptionSettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	value, err := r.GetValue(ctx, key)
	if err != nil {
		return nil, err
	}
	return &Setting{Key: key, Value: value}, nil
}

func (r *proxySubscriptionSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if r != nil && r.values != nil {
		if value, ok := r.values[key]; ok {
			return value, nil
		}
	}
	return "", errors.New("setting not found")
}

func (r *proxySubscriptionSettingRepoStub) Set(ctx context.Context, key, value string) error {
	if r.values == nil {
		r.values = map[string]string{}
	}
	r.values[key] = value
	return nil
}

func (r *proxySubscriptionSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for _, key := range keys {
		if value, err := r.GetValue(ctx, key); err == nil {
			out[key] = value
		}
	}
	return out, nil
}

func (r *proxySubscriptionSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	for key, value := range settings {
		if err := r.Set(ctx, key, value); err != nil {
			return err
		}
	}
	return nil
}

func (r *proxySubscriptionSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	out := map[string]string{}
	for key, value := range r.values {
		out[key] = value
	}
	return out, nil
}

func (r *proxySubscriptionSettingRepoStub) Delete(ctx context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func TestIsLocalTCPPortReachable(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()

	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok)
	require.True(t, isLocalTCPPortReachable(context.Background(), "127.0.0.1", addr.Port))
}

func TestIsLocalTCPPortReachable_ReturnsFalseWhenClosed(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr, ok := ln.Addr().(*net.TCPAddr)
	require.True(t, ok)
	require.NoError(t, ln.Close())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.False(t, isLocalTCPPortReachable(ctx, "127.0.0.1", addr.Port))
}

func TestSidecarProxyHostDefaultsToComposeService(t *testing.T) {
	t.Setenv("SUB2API_SIDECAR_PROXY_HOST", "")
	t.Setenv("SUB2API_SIDECAR_USE_LOCALHOST", "")

	require.Equal(t, "sing-box", sidecarProxyHost())
	require.Equal(t, "sing-box", sidecarProbeHost())
	require.Equal(t, "0.0.0.0", sidecarListenHost())
}

func TestSidecarProxyHostCanUseLocalhostForSingleProcessDeploy(t *testing.T) {
	t.Setenv("SUB2API_SIDECAR_PROXY_HOST", "")
	t.Setenv("SUB2API_SIDECAR_USE_LOCALHOST", "true")
	t.Setenv("SUB2API_SIDECAR_LISTEN_HOST", "127.0.0.1")

	require.Equal(t, "127.0.0.1", sidecarProxyHost())
	require.Equal(t, "127.0.0.1", sidecarProbeHost())
	require.Equal(t, "127.0.0.1", sidecarListenHost())
}

func TestSidecarProbeHostCanOverrideProxyHost(t *testing.T) {
	t.Setenv("SUB2API_SIDECAR_PROXY_HOST", "sidecar-proxy")
	t.Setenv("SUB2API_SIDECAR_PROBE_HOST", "127.0.0.1")

	require.Equal(t, "sidecar-proxy", sidecarProxyHost())
	require.Equal(t, "127.0.0.1", sidecarProbeHost())
}

func TestResolveProxySubscriptionAPIKeyUsesGlobalSetting(t *testing.T) {
	t.Setenv("ABUSEIPDB_API_KEY", "env-key")
	svc := &adminServiceImpl{
		settingService: NewSettingService(&proxySubscriptionSettingRepoStub{
			values: map[string]string{SettingKeyAbuseIPDBAPIKey: "db-key"},
		}, nil),
	}

	got, err := svc.resolveProxySubscriptionAPIKey(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, "db-key", got)

	got, err = svc.resolveProxySubscriptionAPIKey(context.Background(), "literal:explicit-key")
	require.NoError(t, err)
	require.Equal(t, "explicit-key", got)
}

func TestResolveProxySubscriptionAPIKeyDefaultKeymdRefUsesGlobalSetting(t *testing.T) {
	t.Setenv("ABUSEIPDB_API_KEY", "env-key")
	svc := &adminServiceImpl{
		settingService: NewSettingService(&proxySubscriptionSettingRepoStub{
			values: map[string]string{SettingKeyAbuseIPDBAPIKey: "db-key"},
		}, nil),
	}

	got, err := svc.resolveProxySubscriptionAPIKey(context.Background(), "keymd:AbuseIPDB API Key")
	require.NoError(t, err)
	require.Equal(t, "db-key", got)
}
