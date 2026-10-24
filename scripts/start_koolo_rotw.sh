#!/usr/bin/env bash
# Startet Koolo (Warlock / Reign of the Warlock) INNERHALB der D2R-Proton-Prefix.
# Nur so liegen Bot, Spiel-Saves und Game-Settings im selben Prefix.

set -euo pipefail

# Configuration with defaults
: "${KOOLO_HOME:=${HOME}/.var/app/com.usebottles.bottles/data/bottles/bottles/Gaming/drive_c/koolo-rotw}"
: "${STEAM_HOME:=${HOME}/.local/share/Steam}"
: "${STEAM_GAME_ID:=2998511494}"
: "${LOG_DIR:=/tmp}"
: "${LOG_FILE:=${LOG_DIR}/koolo-rotw.log}"
: "${PID_FILE:=${LOG_DIR}/koolo-rotw.pid}"
: "${TASKSET_CPUS:=6-7,14-15}"
: "${NICE_LEVEL:=15}"
: "${DISPLAY:=:0}"
: "${MAX_LOG_SIZE:=$((100 * 1024 * 1024))}"  # 100MB

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging function
log_info() {
    echo -e "${GREEN}[INFO]${NC} $*" >&2
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*" >&2
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
}

# Check prerequisites
check_prerequisites() {
    if [[ ! -d "$KOOLO_HOME" ]]; then
        log_error "Koolo directory not found: $KOOLO_HOME"
        log_error "Please set KOOLO_HOME environment variable"
        return 1
    fi

    if [[ ! -f "$KOOLO_HOME/koolo.exe" ]]; then
        log_error "koolo.exe not found in $KOOLO_HOME"
        return 1
    fi

    if [[ ! -d "$STEAM_HOME" ]]; then
        log_error "Steam directory not found: $STEAM_HOME"
        return 1
    fi

    local proton_path="$STEAM_HOME/steamapps/common/Proton - Experimental/proton"
    if [[ ! -f "$proton_path" ]]; then
        log_warn "Proton not found at expected location, trying to find it..."
        if ! which proton >/dev/null 2>&1; then
            log_error "Proton executable not found"
            return 1
        fi
    fi

    if ! command -v taskset &>/dev/null; then
        log_error "taskset command not found. Please install util-linux"
        return 1
    fi

    return 0
}

# Rotate log file if it exceeds max size
rotate_log() {
    if [[ -f "$LOG_FILE" ]]; then
        local log_size
        log_size=$(stat -f%z "$LOG_FILE" 2>/dev/null || stat -c%s "$LOG_FILE" 2>/dev/null || echo 0)
        
        if [[ $log_size -gt $MAX_LOG_SIZE ]]; then
            local timestamp
            timestamp=$(date +%Y%m%d_%H%M%S)
            local backup_file="${LOG_FILE}.${timestamp}"
            
            log_info "Rotating log file (size: $((log_size / 1024 / 1024))MB)"
            mv "$LOG_FILE" "$backup_file"
            
            # Compress old log if gzip is available
            if command -v gzip &>/dev/null; then
                gzip "$backup_file" &
            fi
        fi
    fi
}

# Kill existing Koolo process
kill_existing() {
    if [[ -f "$PID_FILE" ]]; then
        local old_pid
        old_pid=$(cat "$PID_FILE" 2>/dev/null || echo "")
        if [[ -n "$old_pid" ]] && kill -0 "$old_pid" 2>/dev/null; then
            log_info "Killing existing Koolo process (PID: $old_pid)"
            kill "$old_pid" 2>/dev/null || true
            sleep 2
        fi
    fi

    # Also try pkill as fallback
    pkill -f "proton.*koolo\.exe" 2>/dev/null || true
    sleep 1
}

# Start Koolo
start_koolo() {
    check_prerequisites || return 1
    
    log_info "Starting Koolo RotW..."
    log_info "  Home: $KOOLO_HOME"
    log_info "  CPUs: $TASKSET_CPUS"
    log_info "  Nice Level: $NICE_LEVEL"
    log_info "  Log File: $LOG_FILE"

    rotate_log

    local compatdata="$STEAM_HOME/steamapps/compatdata/$STEAM_GAME_ID"
    local proton_path="$STEAM_HOME/steamapps/common/Proton - Experimental/proton"

    # Use Proton if available, fallback to system proton
    if [[ ! -f "$proton_path" ]]; then
        proton_path=$(which proton 2>/dev/null || echo "proton")
    fi

    # Set up environment
    export WINEPREFIX="${compatdata}/pfx"
    export STEAM_COMPAT_DATA_PATH="$compatdata"
    export STEAM_COMPAT_CLIENT_INSTALL_PATH="$STEAM_HOME"
    export SteamAppId="$STEAM_GAME_ID"
    export SteamGameId="$STEAM_GAME_ID"
    export KOOLO_NO_INJECTOR=1
    export PROTON_LOG=0
    export DISPLAY="$DISPLAY"

    cd "$KOOLO_HOME" || { log_error "Failed to change directory to $KOOLO_HOME"; return 1; }

    # Start Koolo with process substitution for better PID tracking
    nohup taskset -c "$TASKSET_CPUS" nice -n "$NICE_LEVEL" \
        "$proton_path" run "$KOOLO_HOME/koolo.exe" \
        >> "$LOG_FILE" 2>&1 &
    
    local koolo_pid=$!
    echo "$koolo_pid" > "$PID_FILE"

    log_info "Koolo started successfully (PID: $koolo_pid)"
    log_info "Dashboard: http://localhost:8087"
    
    # Give it a moment to start and check if it actually started
    sleep 2
    if ! kill -0 "$koolo_pid" 2>/dev/null; then
        log_error "Koolo process died immediately. Check logs:"
        tail -20 "$LOG_FILE"
        return 1
    fi

    return 0
}

# Stop Koolo
stop_koolo() {
    if [[ -f "$PID_FILE" ]]; then
        local pid
        pid=$(cat "$PID_FILE" 2>/dev/null || echo "")
        if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
            log_info "Stopping Koolo (PID: $pid)..."
            kill -TERM "$pid" 2>/dev/null || true
            
            # Wait for graceful shutdown
            local timeout=10
            while [[ $timeout -gt 0 ]] && kill -0 "$pid" 2>/dev/null; do
                sleep 1
                ((timeout--))
            done
            
            # Force kill if necessary
            if kill -0 "$pid" 2>/dev/null; then
                log_warn "Process didn't stop gracefully, forcing kill..."
                kill -9 "$pid" 2>/dev/null || true
            fi
            
            rm -f "$PID_FILE"
            log_info "Koolo stopped"
        fi
    else
        log_info "No PID file found, attempting pkill..."
        pkill -f "proton.*koolo\.exe" || true
    fi
}

# Show status
show_status() {
    if [[ -f "$PID_FILE" ]]; then
        local pid
        pid=$(cat "$PID_FILE" 2>/dev/null || echo "")
        if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
            log_info "Koolo is running (PID: $pid)"
            return 0
        fi
    fi
    
    log_info "Koolo is not running"
    return 1
}

# Show logs
show_logs() {
    if [[ -f "$LOG_FILE" ]]; then
        tail -f "$LOG_FILE"
    else
        log_error "Log file not found: $LOG_FILE"
        return 1
    fi
}

# Main command handling
main() {
    local command="${1:-start}"
    
    case "$command" in
        start)
            kill_existing
            start_koolo
            ;;
        stop)
            stop_koolo
            ;;
        restart)
            stop_koolo
            sleep 1
            kill_existing
            start_koolo
            ;;
        status)
            show_status
            ;;
        logs)
            show_logs
            ;;
        *)
            echo "Usage: $0 {start|stop|restart|status|logs}"
            echo ""
            echo "Environment variables:"
            echo "  KOOLO_HOME       - Koolo installation directory"
            echo "  STEAM_HOME       - Steam directory"
            echo "  STEAM_GAME_ID    - Steam game ID (default: 2998511494)"
            echo "  LOG_DIR          - Log directory (default: /tmp)"
            echo "  LOG_FILE         - Log file path"
            echo "  TASKSET_CPUS     - CPU cores to bind to (default: 6-7,14-15)"
            echo "  NICE_LEVEL       - Process nice level (default: 15)"
            echo "  MAX_LOG_SIZE     - Max log size before rotation (default: 100MB)"
            exit 1
            ;;
    esac
}

main "$@"
