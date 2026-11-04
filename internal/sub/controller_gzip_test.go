package sub

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// Large rule sets push profiles past 1 MB; on slow links an uncompressed body
// stalls mid-transfer and clients show only the profile title with no nodes.
func TestClashSubscriptionResponseUsesGzip(t *testing.T) {
	seedSubDB(t)
	seedSubInbound(t, "s1", "vl", 4488, 1, `{"network":"tcp","security":"none"}`)
	gin.SetMode(gin.TestMode)
	router := newSubscriptionTestRouter(subscriptionTestRouterConfig{})

	plainResp := httptest.NewRecorder()
	router.ServeHTTP(plainResp, httptest.NewRequest(http.MethodGet, "http://sub.example.com/clash/s1", nil))
	if plainResp.Code != http.StatusOK {
		t.Fatalf("plain status = %d, body=%s", plainResp.Code, plainResp.Body.String())
	}
	if got := plainResp.Header().Get("Content-Encoding"); got != "" {
		t.Fatalf("plain Content-Encoding = %q, want identity", got)
	}

	req := httptest.NewRequest(http.MethodGet, "http://sub.example.com/clash/s1", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("gzip status = %d", resp.Code)
	}
	if got := resp.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	zr, err := gzip.NewReader(resp.Body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	body, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("gzip read: %v", err)
	}
	if string(body) != plainResp.Body.String() {
		t.Fatal("decompressed gzip body differs from the identity response")
	}
}
