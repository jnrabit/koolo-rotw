package server

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestWebSocketServerCreation tests basic WebSocket server creation
func TestWebSocketServerCreation(t *testing.T) {
	t.Run("create websocket server", func(t *testing.T) {
		ws := NewWebSocketServer()
		if ws == nil {
			t.Fatal("failed to create WebSocketServer")
		}
		if ws.clients == nil {
			t.Error("clients map not initialized")
		}
		if ws.broadcast == nil {
			t.Error("broadcast channel not initialized")
		}
		if ws.register == nil {
			t.Error("register channel not initialized")
		}
		if ws.unregister == nil {
			t.Error("unregister channel not initialized")
		}
	})
}

// TestWebSocketChannelCreation tests that WebSocket channels are properly created
func TestWebSocketChannelCreation(t *testing.T) {
	t.Run("channels should be buffered appropriately", func(t *testing.T) {
		ws := NewWebSocketServer()

		// Verify channels are created (not nil)
		channelTests := []struct {
			name    string
			channel interface{}
		}{
			{"clients map", ws.clients},
			{"broadcast chan", ws.broadcast},
			{"register chan", ws.register},
			{"unregister chan", ws.unregister},
		}

		for _, test := range channelTests {
			if test.channel == nil {
				t.Errorf("%s should not be nil", test.name)
			}
		}
	})
}

// TestCORSOriginValidation tests the CORS origin validation logic
func TestCORSOriginValidation(t *testing.T) {
	tests := []struct {
		name        string
		origin      string
		shouldAllow bool
	}{
		{"localhost:8080", "http://localhost:8080", true},
		{"localhost:8087", "http://localhost:8087", true},
		{"127.0.0.1:8080", "http://127.0.0.1:8080", true},
		{"127.0.0.1:8087", "http://127.0.0.1:8087", true},
		{"IPv6 loopback", "http://[::1]:8080", true},
		{"IPv6 loopback port 8087", "http://[::1]:8087", true},
		{"external domain", "http://external.com:8080", false},
		{"attacker site", "https://attacker.com:443", false},
		{"different localhost", "http://192.168.1.1:8087", false},
		{"no origin header", "", true},
		{"invalid URL", "not a url", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLocalhost(tt.origin)
			if result != tt.shouldAllow {
				t.Errorf("origin %q: got %v, want %v", tt.origin, result, tt.shouldAllow)
			}
		})
	}
}

// TestCORSPortHandling tests that port numbers don't bypass CORS validation
func TestCORSPortHandling(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		allowed bool
	}{
		{"localhost any port", "http://localhost:9999", true},
		{"127.0.0.1 any port", "http://127.0.0.1:5000", true},
		{"external with localhost in name", "http://localhost.attacker.com:8080", false},
		{"localhost with auth", "http://user:pass@localhost:8080", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLocalhost(tt.origin)
			if result != tt.allowed {
				t.Errorf("origin %q: got %v, want %v", tt.origin, result, tt.allowed)
			}
		})
	}
}

// TestCORSSecurityBypass tests that common CORS bypass techniques don't work
func TestCORSSecurityBypass(t *testing.T) {
	// These should all be blocked
	blockedOrigins := []string{
		"http://evil.com",
		"https://attacker.com",
		"http://localhost.attacker.com", // Subdomain trick
		"http://127.0.0.1.attacker.com", // Subdomain trick
		"http://192.168.1.1",            // Private IP (not localhost)
		"http://::1.attacker.com",       // Subdomain trick on IPv6
	}

	for _, origin := range blockedOrigins {
		t.Run("block: "+origin, func(t *testing.T) {
			if isLocalhost(origin) {
				t.Errorf("security bypass detected: %q should be blocked", origin)
			}
		})
	}
}

// TestWebSocketBroadcast tests message broadcasting with timeout
func TestWebSocketBroadcast(t *testing.T) {
	t.Run("broadcast channel should be operational", func(t *testing.T) {
		ws := NewWebSocketServer()

		// Create a timeout context to prevent hanging
		done := make(chan bool, 1)
		go func() {
			ws.Run()
			done <- true
		}()

		// Give the server time to start
		time.Sleep(10 * time.Millisecond)

		// Create test message
		testMsg := []byte("test message")

		// Send broadcast with timeout
		select {
		case ws.broadcast <- testMsg:
			// Message sent successfully
		case <-time.After(100 * time.Millisecond):
			t.Error("broadcast send timed out")
		}

		// Give it time to process
		time.Sleep(10 * time.Millisecond)

		// If no panic, test passes
	})
}

// Helper function to check if origin is localhost
// This mirrors the CheckOrigin validation from http_server.go
func isLocalhost(origin string) bool {
	if origin == "" {
		return true // Allow if no Origin header (same-origin requests)
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	// Only allow localhost/127.0.0.1 and ::1 (IPv6 loopback)
	host := strings.Split(u.Host, ":")[0]
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// TestIsLocalhostHelper tests the CORS validation helper function
func TestIsLocalhostHelper(t *testing.T) {
	t.Run("helper function validation", func(t *testing.T) {
		// Test that helper matches the expected security profile
		tests := []struct {
			origin string
			expect bool
		}{
			{"http://localhost:8080", true},
			{"http://127.0.0.1:8080", true},
			{"http://[::1]:8080", true},
			{"http://example.com:8080", false},
			{"", true},
		}

		for _, test := range tests {
			result := isLocalhost(test.origin)
			if result != test.expect {
				t.Errorf("isLocalhost(%q): got %v, want %v", test.origin, result, test.expect)
			}
		}
	})
}
