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
