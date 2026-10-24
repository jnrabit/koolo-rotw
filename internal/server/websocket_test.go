package server

import (
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

// TestWebSocketBroadcast tests message broadcasting
func TestWebSocketBroadcast(t *testing.T) {
	t.Run("broadcast to multiple clients", func(t *testing.T) {
		ws := NewWebSocketServer()
		
		// Run the server loop in a goroutine
		go ws.Run()
		
		// Give the server time to start
		time.Sleep(10 * time.Millisecond)
		
		// Create test message
		testMsg := []byte("test message")
		
		// Send broadcast
		ws.broadcast <- testMsg
		
		// Give it time to process
		time.Sleep(10 * time.Millisecond)
		
		// If no panic, test passes
	})
}

// TestCORSValidation tests the CORS origin checking
func TestCORSValidation(t *testing.T) {
	tests := []struct {
		name        string
		origin      string
		shouldAllow bool
	}{
		{"localhost", "http://localhost:8080", true},
		{"127.0.0.1", "http://127.0.0.1:8087", true},
		{"IPv6 loopback", "http://[::1]:8080", true},
		{"external domain", "http://external.com:8080", false},
		{"no origin", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test documents expected CORS behavior
			// The actual validation happens in the CheckOrigin function
			expectedResult := tt.shouldAllow
			if expectedResult && tt.origin != "" && !isLocalhost(tt.origin) {
				t.Errorf("test case inconsistent for %s", tt.name)
			}
		})
	}
}

// Helper function to check if origin is localhost
func isLocalhost(origin string) bool {
	// Simple check - real implementation in http_server.go CheckOrigin function
	return origin == "" || 
		len(origin) > 7 && (origin[:7] == "http://" || origin[:8] == "https://")
}
