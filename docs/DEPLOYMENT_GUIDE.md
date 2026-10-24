# Koolo Bot - Deployment Guide

## Overview

This guide covers deployment best practices for Koolo Bot, including log rotation, graceful shutdown, and process management.

## Starting Koolo

### Using the Improved Start Script

The new `scripts/start_koolo_rotw.sh` provides better control and error handling:

```bash
# Start Koolo
./scripts/start_koolo_rotw.sh start

# Check status
./scripts/start_koolo_rotw.sh status

# View logs
./scripts/start_koolo_rotw.sh logs

# Stop Koolo (graceful shutdown)
./scripts/start_koolo_rotw.sh stop

# Restart
./scripts/start_koolo_rotw.sh restart
```

### Configuration

The script uses environment variables for configuration:

```bash
# Custom installation path
export KOOLO_HOME=/path/to/koolo

# Custom Steam path  
export STEAM_HOME=/path/to/steam

# Custom log directory
export LOG_DIR=/var/log/koolo

# CPU cores to bind to (for performance)
export TASKSET_CPUS=6-7,14-15

# Process nice level (higher = lower priority)
export NICE_LEVEL=15

# Max log size before rotation (default: 100MB)
export MAX_LOG_SIZE=$((100 * 1024 * 1024))

./scripts/start_koolo_rotw.sh start
```

## Log Rotation

### Automatic Log Rotation

The startup script automatically rotates log files when they exceed the configured size limit (default: 100MB).

Rotated logs are:
- Renamed with timestamp: `koolo-rotw.log.20240101_120000`
- Compressed with gzip (if available): `koolo-rotw.log.20240101_120000.gz`

### Manual Log Rotation

The `LogRotationConfig` in `internal/utils/log_rotation.go` can be used programmatically:

```go
import "github.com/hectorgimenez/koolo/internal/utils"

config := &utils.LogRotationConfig{
    MaxFileSize: 104857600, // 100MB
    MaxBackups:  5,         // Keep 5 backups
    MaxAgeDays:  30,        // Delete logs older than 30 days
    Compress:    true,      // Compress rotated logs
}

if err := utils.RotateLogsIfNeeded(logPath, config); err != nil {
    log.Printf("rotation failed: %v", err)
}
```

## Graceful Shutdown

### Implementation

The `ShutdownManager` in `internal/utils/shutdown.go` provides coordinated shutdown:

```go
import "github.com/hectorgimenez/koolo/internal/utils"

// Create shutdown manager with 30-second timeout
shutdownMgr := utils.NewShutdownManager(30 * time.Second)

// Register shutdown handlers (called in reverse order)
shutdownMgr.RegisterHandler(func(ctx context.Context) error {
    // Close database connections
    return db.Close()
})

shutdownMgr.RegisterHandler(func(ctx context.Context) error {
    // Stop HTTP server
    return httpServer.Shutdown(ctx)
})

// Wait for shutdown signal (SIGINT, SIGTERM)
<-shutdownMgr.WaitForShutdownSignal(context.Background())
```

### Features

- **Graceful termination**: Handlers are called in reverse registration order
- **Timeout protection**: Shutdown must complete within timeout (default: 30s)
- **Signal handling**: Responds to SIGINT (Ctrl+C) and SIGTERM
- **Safe channel closing**: Helper function prevents panic on double-close

### Signal Handling

The bot responds to:
- **SIGINT** (Ctrl+C): Graceful shutdown
- **SIGTERM** (termination signal): Graceful shutdown

During shutdown:
1. All registered handlers are called in reverse order
2. Each handler has a timeout period to complete
3. Errors are logged but don't prevent other handlers from running
4. Process exits when all handlers complete or timeout is exceeded

## Process Management

### Systemd Service (Optional)

Create `/etc/systemd/system/koolo-rotw.service`:

```ini
[Unit]
Description=Koolo - Diablo II Reign of the Warlock Bot
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=yourusername
WorkingDirectory=/home/yourusername/projekte/koolo\ bot/koolo-rotw
ExecStart=/home/yourusername/projekte/koolo\ bot/koolo-rotw/scripts/start_koolo_rotw.sh start
ExecStop=/home/yourusername/projekte/koolo\ bot/koolo-rotw/scripts/start_koolo_rotw.sh stop
Restart=on-failure
RestartSec=5
StandardOutput=append:/tmp/koolo-rotw.log
StandardError=append:/tmp/koolo-rotw.log

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable koolo-rotw
sudo systemctl start koolo-rotw
sudo systemctl status koolo-rotw
```

### Monitoring

Monitor Koolo with systemd:
```bash
# Check service status
systemctl status koolo-rotw

# View recent logs
journalctl -u koolo-rotw -n 50 -f

# Check resource usage
systemctl status koolo-rotw | grep Memory
```

## Logging Best Practices

### Log Levels

Use appropriate log levels in your code:

```go
import "log/slog"

// Debug: Verbose information for diagnosing problems
slog.Debug("detailed debug info", "key", value)

// Info: General informational messages
slog.Info("process started", "pid", pid)

// Warn: Warning conditions that may need attention
slog.Warn("high memory usage detected", "memory_mb", used)

// Error: Error conditions
slog.Error("failed to connect", "error", err)
```

### Structured Logging

Always use structured logging (key-value pairs) instead of string formatting:

```go
// Good
slog.Info("item dropped", "item_name", "Windforce", "rarity", "unique")

// Avoid
slog.Info(fmt.Sprintf("item dropped: %s (%s)", name, rarity))
```

### Log Filtering

In development, you can set log level via environment variables. The logger configuration is in `cmd/koolo/log/logger.go`.

## Troubleshooting

### Koolo doesn't start
1. Check prerequisites with `./scripts/start_koolo_rotw.sh start`
2. Review logs: `./scripts/start_koolo_rotw.sh logs`
3. Verify Proton installation
4. Check WINEPREFIX is accessible

### High disk usage from logs
1. Check log size: `du -sh /tmp/koolo-rotw.log*`
2. Manually rotate: `./scripts/start_koolo_rotw.sh` (this auto-rotates)
3. Adjust `MAX_LOG_SIZE` environment variable
4. Delete old compressed logs: `rm /tmp/koolo-rotw.log.*.gz`

### Process doesn't shut down gracefully
1. Check for stuck handlers blocking shutdown
2. Review logs for shutdown phase errors
3. Increase shutdown timeout if needed: `ShutdownManager(60 * time.Second)`
4. Force kill as last resort: `pkill -9 -f koolo.exe`

## Security Considerations

1. **CORS Protection**: WebSocket connections are restricted to localhost only
2. **Log Files**: Remove or archive old logs to prevent information leakage
3. **Process Isolation**: Run as unprivileged user when possible
4. **File Permissions**: Ensure log directory is readable only by the service user
5. **Graceful Shutdown**: Proper cleanup prevents resource leaks

## Performance Tuning

### CPU Binding
The start script binds Koolo to specific CPU cores for performance:
```bash
export TASKSET_CPUS=6-7,14-15  # Adjust based on your system
```

### Priority Management
Process nice level affects CPU scheduling:
- Higher values = lower priority
- Typical: `NICE_LEVEL=15` (below-normal priority)
- For interactive performance: `NICE_LEVEL=0` (normal priority)

### Memory Management
Monitor memory usage with:
```bash
ps aux | grep koolo.exe
```

## Related Files

- `internal/utils/log_rotation.go` - Log rotation implementation
- `internal/utils/shutdown.go` - Graceful shutdown implementation  
- `scripts/start_koolo_rotw.sh` - Start/stop/restart script
- `internal/server/http_server.go` - CORS security settings
