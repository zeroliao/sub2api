//go:build unit

package service

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
