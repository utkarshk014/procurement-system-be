package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	rest_utils "github.com/utkarshk014/procurement-system-be/src/delivery/rest/utils"
)

func seenIP(trusted []string, remoteAddr string, headers map[string]string) string {
	var got string
	h := clientIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = rest_utils.ClientIP(r) }))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = remoteAddr
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	h.ServeHTTP(httptest.NewRecorder(), r)
	return got
}

func TestClientIPCannotBeSpoofed(t *testing.T) {
	spoof := map[string]string{"True-Client-IP": "1.2.3.4", "X-Real-IP": "1.2.3.4", "X-Forwarded-For": "1.2.3.4"}

	// No trusted proxies: headers are ignored, the TCP peer is the client.
	assert.Equal(t, "203.0.113.7", seenIP(nil, "203.0.113.7:5000", spoof))

	// Trusted proxy list, but the request did not come from it: still the TCP peer.
	assert.Equal(t, "203.0.113.7", seenIP([]string{"10.0.0.0/8"}, "203.0.113.7:5000", spoof))

	// Through the trusted proxy: the rightmost untrusted X-Forwarded-For hop, not a client-chosen first entry.
	assert.Equal(t, "198.51.100.20", seenIP([]string{"10.0.0.0/8"}, "10.0.0.5:443",
		map[string]string{"X-Forwarded-For": "1.2.3.4, 198.51.100.20", "True-Client-IP": "9.9.9.9"}))
}
