package main

import "testing"

func TestRequireLoopback(t *testing.T) {
	for _, address := range []string{"127.0.0.1:0", "localhost:8080", "[::1]:3000"} {
		if err := requireLoopback(address); err != nil {
			t.Errorf("requireLoopback(%q): %v", address, err)
		}
	}
	for _, address := range []string{"0.0.0.0:8080", "192.168.1.10:8080", ":8080"} {
		if err := requireLoopback(address); err == nil {
			t.Errorf("requireLoopback(%q) unexpectedly succeeded", address)
		}
	}
}
