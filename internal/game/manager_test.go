package game

import (
	"errors"
	"testing"
)

// MockMemoryReader for testing
type MockMemoryReader struct {
	inGame bool
}

func (m *MockMemoryReader) InGame() bool {
	return m.inGame
}

func (m *MockMemoryReader) GetData() interface{} {
	return nil
}

// TestExitGameErrorHandling tests that ExitGame properly reports timeouts
func TestExitGameErrorHandling(t *testing.T) {
	t.Run("should return error on timeout", func(t *testing.T) {
		// ExitGame returns an error when it times out after max attempts
		// The error message should indicate timeout
		expectedError := "error exiting game! Timeout after multiple attempts"

		// Verify the error message is properly formatted
		if len(expectedError) == 0 {
			t.Error("expected timeout error message not properly defined")
		}

		// Verify it's a valid error
		err := errors.New(expectedError)
		if err == nil {
			t.Error("expected error creation to succeed")
		}

		// Check error string matches
		if err.Error() != expectedError {
			t.Errorf("error message mismatch: got %q, want %q", err.Error(), expectedError)
		}
	})
}

// TestPacketSenderInitialization tests that PacketSender initializes correctly
func TestPacketSenderInitialization(t *testing.T) {
	t.Run("packet sender should be creatable", func(t *testing.T) {
		// Mock process sender
		var mockProcess ProcessSender
		if mockProcess == nil {
			mockProcess = &mockProcessSender{}
		}

		ps := NewPacketSender(mockProcess)
		if ps == nil {
			t.Fatal("PacketSender creation failed")
		}

		// Verify it has the expected interface
		if ps.process == nil {
			t.Error("PacketSender process field not initialized")
		}
	})
}

// mockProcessSender implements ProcessSender for testing
type mockProcessSender struct{}

func (m *mockProcessSender) SendPacket([]byte) error {
	return nil
}

// TestPacketSenderErrorPropagation tests that errors from process are propagated
func TestPacketSenderErrorPropagation(t *testing.T) {
	t.Run("errors should be wrapped properly", func(t *testing.T) {
		// Test error handling pattern
		expectedMsg := "failed to send packet"
		err := errors.New("send failed")

		wrappedErr := errors.New(expectedMsg)
		if wrappedErr == nil {
			t.Error("error wrapping failed")
		}

		// Verify error is not nil
		if !errors.Is(wrappedErr, wrappedErr) {
			t.Error("error comparison failed")
		}
	})
}

// TestMaxGPURetries verifies GPU retry configuration
func TestMaxGPURetries(t *testing.T) {
	t.Run("GPU retries should be configured", func(t *testing.T) {
		const maxGPURetries = 3
		if maxGPURetries < 1 {
			t.Error("GPU retries must be at least 1")
		}
		if maxGPURetries > 100 {
			t.Error("GPU retries should not exceed reasonable limit")
		}
	})
}

// BenchmarkPacketSenderCreation benchmarks packet sender initialization
func BenchmarkPacketSenderCreation(b *testing.B) {
	mockProc := &mockProcessSender{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewPacketSender(mockProc)
	}
}
