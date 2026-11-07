package rest_utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/utkarshk014/procurement-system-be/src/internal/domain"
)

func decode(contentType, body string) error {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set("Content-Type", contentType)
	var dst struct {
		Name string `json:"name"`
	}
	return DecodeJSON(httptest.NewRecorder(), r, &dst)
}

func TestDecodeJSONIsStrict(t *testing.T) {
	assert.NoError(t, decode("application/json", `{"name":"x"}`))
	assert.NoError(t, decode("application/json; charset=utf-8", `{"name":"x"}`+"\n"))

	for name, tc := range map[string][2]string{
		"trailing brace":       {"application/json", `{"name":"x"}}`},
		"second object":        {"application/json", `{"name":"x"}{"name":"y"}`},
		"lookalike type":       {"application/jsonx", `{"name":"x"}`},
		"missing type":         {"", `{"name":"x"}`},
		"unknown field":        {"application/json", `{"name":"x","sl":1}`},
		"empty body":           {"application/json", ``},
		"wrong value type":     {"application/json", `{"name":5}`},
		"truncated object":     {"application/json", `{"name":`},
		"plain text instead":   {"text/plain", `{"name":"x"}`},
		"top-level not object": {"application/json", `["x"]`},
	} {
		assert.Error(t, decode(tc[0], tc[1]), name)
	}
}

func writeErr(err error) (int, map[string]any) {
	rec := httptest.NewRecorder()
	WriteError(rec, httptest.NewRequest(http.MethodGet, "/", nil), slog.New(slog.NewTextHandler(io.Discard, nil)), err)
	var body map[string]map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body["error"]
}

func TestWriteErrorSeparatesOutagesFromBugs(t *testing.T) {
	timeout := domain.Internal("Could not load user", fmt.Errorf("timeout: %w", context.DeadlineExceeded))
	code, body := writeErr(timeout)
	assert.Equal(t, http.StatusGatewayTimeout, code)
	assert.Equal(t, "timeout", body["code"])

	down := domain.Internal("Could not load user", &net.OpError{Op: "dial", Err: fmt.Errorf("connection refused")})
	code, body = writeErr(down)
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, "service_unavailable", body["code"])

	code, body = writeErr(domain.Internal("Could not save", fmt.Errorf("boom")))
	require.Equal(t, http.StatusInternalServerError, code)
	assert.Equal(t, "Something went wrong, please try again", body["message"], "cause stays hidden")

	code, _ = writeErr(domain.MethodNotAllowed("Method not allowed"))
	assert.Equal(t, http.StatusMethodNotAllowed, code)
}
