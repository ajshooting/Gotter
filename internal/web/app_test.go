package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gotter/internal/config"
)

func TestSafeReturnPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "/", want: "/"},
		{input: "/users/tester?before=10", want: "/users/tester?before=10"},
		{input: "", want: "/"},
		{input: "https://evil.example", want: "/"},
		{input: "//evil.example", want: "/"},
		{input: `/\\evil.example`, want: "/"},
		{input: "/%5c%5cevil.example", want: "/"},
		{input: "/%2f%2fevil.example", want: "/"},
	}

	for _, tt := range tests {
		if got := safeReturnPath(tt.input); got != tt.want {
			t.Errorf("safeReturnPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

	app := &App{cfg: config.Config{CookieSecure: true}}
	handler := app.securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/login", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	header := response.Header()
	for _, name := range []string{
		"Content-Security-Policy",
		"Permissions-Policy",
		"Referrer-Policy",
		"Strict-Transport-Security",
		"X-Content-Type-Options",
		"X-Frame-Options",
	} {
		if header.Get(name) == "" {
			t.Errorf("%s header is missing", name)
		}
	}
	if got := header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}
