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

func TestFetchProxySubscription_SendsSubscriptionHeaders(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, proxySubscriptionClientUserAgent, r.Header.Get("User-Agent"))
		require.NotEqual(t, proxyQualityClientUserAgent, r.Header.Get("User-Agent"))
		require.Equal(t, "text/yaml, application/x-yaml, text/plain, application/json;q=0.9, */*;q=0.8", r.Header.Get("Accept"))
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	body, err := fetchProxySubscription(context.Background(), server.URL)
	require.NoError(t, err)
	require.Equal(t, "ok", body)
}

func TestFetchProxySubscription_RejectsHTMLResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><html><body>download subscription</body></html>"))
	}))
	defer server.Close()

	_, err := fetchProxySubscription(context.Background(), server.URL)
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
	require.Contains(t, infraerrors.Message(err), "is HTML, not proxy configuration")
}

func TestParseProxyImportItems_ParsesClashYAML(t *testing.T) {
	t.Parallel()

	items := parseProxyImportItems(`proxies:
  - name: ss-node
    type: ss
    server: 198.51.100.10
    port: 443
    cipher: aes-128-gcm
    password: secret
  - name: http-node
    type: http
    server: 198.51.100.20
    port: 8080
    username: user
    password: pass
`, "provider")

	require.Len(t, items, 2)
	require.Equal(t, "clash", items[0].Source)
	require.Equal(t, "ss", items[0].Protocol)
	require.Equal(t, "198.51.100.10", items[0].Host)
	require.Equal(t, 443, items[0].Port)
	require.True(t, items[0].Valid)
	require.True(t, items[0].SidecarRequired)
	require.Equal(t, "http", items[1].Protocol)
	require.Equal(t, "198.51.100.20", items[1].Host)
	require.Equal(t, 8080, items[1].Port)
	require.True(t, items[1].Valid)
	require.False(t, items[1].SidecarRequired)
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

func TestParseProxyLine_SupportedSidecarRequiresCredentials(t *testing.T) {
	t.Parallel()

	item := parseProxyLine("vless://@example.com:443?type=tcp", "")
	require.False(t, item.Valid)
	require.True(t, item.SidecarRequired)
	require.Equal(t, "sidecar proxy URL is missing credentials", item.Error)
}

func TestUnsupportedSidecarProtocolCannotBeSelected(t *testing.T) {
	t.Parallel()

	item := parseProxyLine("ss://secret@example.com:443", "")
	statuses := selectProxySubscriptionItems([]ProxyImportPreviewItem{item}, &ProxySubscriptionSource{SidecarEnabled: true}, defaultProxySubscriptionStrategy(), map[string]proxySubscriptionNodeEvaluation{
		item.Key: {Key: item.Key, Score: 100},
	})
	require.Empty(t, statuses)
	require.False(t, isSupportedSubscriptionSidecarProtocol(item.Protocol))
}

func TestConnectivityFailedNodeCannotBeSelected(t *testing.T) {
	t.Parallel()
	item := parseProxyLine("http://proxy.example.com:8080", "")
	statuses := selectProxySubscriptionItems([]ProxyImportPreviewItem{item}, &ProxySubscriptionSource{SidecarEnabled: true}, defaultProxySubscriptionStrategy(), map[string]proxySubscriptionNodeEvaluation{
		item.Key: {Key: item.Key, Score: 100, ConnectivityFailed: true},
	})
	require.Empty(t, statuses)
}

func TestProbeProxySubscriptionConnectivityEmptyTargetAllows(t *testing.T) {
	t.Parallel()
	item := parseProxyLine("http://proxy.example.com:8080", "")
	require.NoError(t, probeProxySubscriptionConnectivity(context.Background(), item, &ProxySubscriptionSource{}, defaultProxySubscriptionStrategy()))
}
