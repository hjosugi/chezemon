package server

import "testing"

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
