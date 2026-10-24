package utils

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// ShutdownManager handles graceful shutdown of the application
type ShutdownManager struct {
	mu              sync.Mutex
	handlers        []ShutdownHandler
	shutdownTimeout time.Duration
	isShuttingDown  bool
}

// ShutdownHandler is a function that will be called during graceful shutdown
type ShutdownHandler func(ctx context.Context) error

// NewShutdownManager creates a new shutdown manager
func NewShutdownManager(timeout time.Duration) *ShutdownManager {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &ShutdownManager{
		handlers:        make([]ShutdownHandler, 0),
		shutdownTimeout: timeout,
	}
}

// RegisterHandler registers a shutdown handler to be called during graceful shutdown
func (sm *ShutdownManager) RegisterHandler(handler ShutdownHandler) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.handlers = append(sm.handlers, handler)
}

// WaitForShutdownSignal blocks until a shutdown signal is received
// It returns a channel that will be closed when shutdown should occur
func (sm *ShutdownManager) WaitForShutdownSignal(ctx context.Context) <-chan struct{} {
	shutdownChan := make(chan struct{})
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan,
			syscall.SIGINT,   // Ctrl+C
			syscall.SIGTERM,  // Termination signal
		)

		select {
		case <-ctx.Done():
			close(shutdownChan)
		case sig := <-sigChan:
			slog.Info("received shutdown signal", "signal", sig.String())
			sm.Shutdown(ctx)
			close(shutdownChan)
		}
	}()
	return shutdownChan
}

// Shutdown gracefully shuts down all registered handlers
func (sm *ShutdownManager) Shutdown(ctx context.Context) error {
	sm.mu.Lock()
	if sm.isShuttingDown {
		sm.mu.Unlock()
		return nil // Already shutting down
	}
	sm.isShuttingDown = true
	handlers := make([]ShutdownHandler, len(sm.handlers))
	copy(handlers, sm.handlers)
	sm.mu.Unlock()

	// Create a context with timeout for shutdown operations
	shutdownCtx, cancel := context.WithTimeout(ctx, sm.shutdownTimeout)
	defer cancel()

	slog.Info("starting graceful shutdown", "timeout", sm.shutdownTimeout.String())

	// Call handlers in reverse order
	errorCount := 0
	for i := len(handlers) - 1; i >= 0; i-- {
		handler := handlers[i]
		handlerName := getFunctionName(handler)

		if err := handler(shutdownCtx); err != nil {
			slog.Error("shutdown handler failed", "handler", handlerName, "error", err)
			errorCount++
		} else {
			slog.Debug("shutdown handler completed", "handler", handlerName)
		}

		// Check if context was canceled (timeout)
		if shutdownCtx.Err() != nil {
			slog.Warn("shutdown timeout exceeded", "remaining_handlers", i)
			break
		}
	}

	if errorCount > 0 {
		slog.Warn("shutdown completed with errors", "error_count", errorCount)
		return fmt.Errorf("%d shutdown handlers failed", errorCount)
	}

	slog.Info("graceful shutdown completed")
	return nil
}

// IsShuttingDown returns whether the manager is currently shutting down
func (sm *ShutdownManager) IsShuttingDown() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.isShuttingDown
}

// getFunctionName gets the name of a function (basic implementation)
func getFunctionName(fn ShutdownHandler) string {
	if fn == nil {
		return "unknown"
	}
	return "handler"
}

// SafeClose safely closes a channel, preventing panic on double close
func SafeClose(ch chan struct{}) {
	defer func() {
		if r := recover(); r != nil {
			slog.Debug("channel already closed", "panic", r)
		}
	}()
	close(ch)
}
