package service

import (
	"context"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestFetchProxySubscription_InvalidURLReturnsBadRequest(t *testing.T) {
	t.Parallel()

	_, err := fetchProxySubscription(context.Background(), "not a url")
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
	require.Equal(t, "invalid subscription URL", infraerrors.Message(err))
}

func TestFetchProxySubscription_Non2xxReturnsBadRequest(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	_, err := fetchProxySubscription(context.Background(), server.URL)
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
	require.Equal(t, "subscription URL returned HTTP 403", infraerrors.Message(err))
}

func TestFetchProxySubscription_SendsBrowserLikeHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, proxyQualityClientUserAgent, r.Header.Get("User-Agent"))
		require.Equal(t, "*/*", r.Header.Get("Accept"))
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	body, err := fetchProxySubscription(context.Background(), server.URL)
	require.NoError(t, err)
	require.Equal(t, "ok", body)
}

func TestSubscriptionFetchErrorMessage_RedactsURLQuery(t *testing.T) {
	t.Parallel()

	err := &url.Error{
		Op:  http.MethodGet,
		URL: "https://example.com/api/v1/client/subscribe?token=secret-token",
		Err: stderrors.New("EOF"),
	}

	message := subscriptionFetchErrorMessage("example.com", err)
	require.Contains(t, message, "example.com")
	require.Contains(t, message, "EOF")
	require.False(t, strings.Contains(message, "secret-token"))
	require.False(t, strings.Contains(message, "/api/v1/client/subscribe"))
}

func TestParseProxyLine_AnyTLSIsMarkedAsSidecar(t *testing.T) {
	t.Parallel()

	item := parseProxyLine("anytls://secret@example.com:443?type=tcp#us-node", "")
	require.True(t, item.Valid)
	require.True(t, item.SidecarRequired)
	require.Equal(t, "sidecar", item.ProxyType)
	require.Equal(t, "anytls", item.Protocol)
	require.Equal(t, "us-node", item.Name)
}
