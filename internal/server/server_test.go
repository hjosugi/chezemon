package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hjosugi/chezemon/internal/state"
)

func TestIsLoopbackHost(t *testing.T) {
	allowed := []string{
		"127.0.0.1:38721",
		"127.0.0.1",
		"localhost:8080",
		"LocalHost:8080",
		"[::1]:3000",
		"::1",
		"127.5.5.5:80",
	}
	for _, host := range allowed {
		if !isLoopbackHost(host) {
			t.Errorf("isLoopbackHost(%q) = false, want true", host)
		}
	}

	// A DNS-rebinding attacker controls the Host header, so anything that is
	// not a loopback name must be rejected even though the socket is bound to
	// 127.0.0.1.
	rejected := []string{
		"",
		"evil.example.com",
		"evil.example.com:38721",
		"192.168.1.10:38721",
		"0.0.0.0:38721",
		"chezemon.localhost.evil.com",
	}
	for _, host := range rejected {
		if isLoopbackHost(host) {
			t.Errorf("isLoopbackHost(%q) = true, want false", host)
		}
	}
}

// The API is GET-only; the asset handler is registered the same way so that a
// write method is refused rather than quietly answered with the page.
func TestStaticAssetsRejectNonGET(t *testing.T) {
	handler := New(state.NewService(nil, time.Second), slog.New(slog.DiscardHandler))

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(method, "http://127.0.0.1/", nil))
		if recorder.Code != http.StatusOK {
			t.Errorf("%s / = %d, want 200", method, recorder.Code)
		}
	}

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(method, "http://127.0.0.1/", nil))
		if recorder.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s / = %d, want 405", method, recorder.Code)
		}
	}
}

func TestNonLoopbackHostIsRefused(t *testing.T) {
	handler := New(state.NewService(nil, time.Second), slog.New(slog.DiscardHandler))
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/", nil)
	request.Host = "evil.example.com"

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Errorf("got %d, want 403", recorder.Code)
	}
}
