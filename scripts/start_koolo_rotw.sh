#!/usr/bin/env bash
# Startet Koolo (Warlock / Reign of the Warlock) INNERHALB der D2R-Proton-Prefix.
# Nur so liegen Bot, Spiel-Saves und Game-Settings im selben Prefix.
#
# Verwendung: start_koolo_rotw.sh {start|stop|restart|status|logs}
#
# Persoenliche Pfade/Einstellungen gehoeren NICHT ins Repo, sondern in
#   ~/.config/koolo-rotw.env   (oder Pfad via KOOLO_ENV_FILE)
# Vorlage: config/koolo-rotw.env.example

set -euo pipefail

# --- Optionale persoenliche Konfiguration laden --------------------------------
ENV_FILE="${KOOLO_ENV_FILE:-${XDG_CONFIG_HOME:-$HOME/.config}/koolo-rotw.env}"
if [[ -f "$ENV_FILE" ]]; then
    # shellcheck disable=SC1090
    source "$ENV_FILE"
fi

# --- Defaults (per Env ueberschreibbar) ----------------------------------------
: "${KOOLO_HOME:=${HOME}/.var/app/com.usebottles.bottles/data/bottles/bottles/Gaming/drive_c/koolo-rotw}"
: "${STEAM_HOME:=${HOME}/.local/share/Steam}"
: "${STEAM_GAME_ID:=2998511494}"
: "${PROTON_PATH:=}"                       # leer = Steam "Proton - Experimental", dann `proton` im PATH
: "${API_PORT:=8087}"
: "${LOG_DIR:=/tmp}"
: "${LOG_FILE:=${LOG_DIR}/koolo-rotw.log}"
: "${PID_FILE:=${LOG_DIR}/koolo-rotw.pid}"
: "${TASKSET_CPUS:=6-7,14-15}"             # inkl. SMT-Threads; D2R.exe erbt Bindung + Nice
: "${NICE_LEVEL:=15}"
: "${DISPLAY:=:0}"
: "${MAX_LOG_SIZE:=$((100 * 1024 * 1024))}"  # 100 MB
: "${STOP_GRACE_SECONDS:=4}"               # Wartezeit nach graceful /stop
: "${KILL_WAIT_SECONDS:=5}"                # Wartezeit nach SIGTERM, danach SIGKILL
: "${PORT_WAIT_SECONDS:=20}"               # max. Wartezeit bis API-Port frei ist
: "${POST_START_CMD:=}"                    # Hook: nach erfolgreichem Start (z.B. Stream)
: "${PRE_STOP_CMD:=}"                      # Hook: vor dem Stoppen (z.B. Stream beenden)

# --- Ausgabe -------------------------------------------------------------------
if [[ -t 2 ]]; then
    RED=$'\033[0;31m'; GREEN=$'\033[0;32m'; YELLOW=$'\033[1;33m'; NC=$'\033[0m'
else
    RED=""; GREEN=""; YELLOW=""; NC=""
fi
log_info()  { echo "${GREEN}[INFO]${NC} $*" >&2; }
log_warn()  { echo "${YELLOW}[WARN]${NC} $*" >&2; }
log_error() { echo "${RED}[ERROR]${NC} $*" >&2; }

# --- Helfer --------------------------------------------------------------------
read_pid() {
    local pid=""
    if [[ -f "$PID_FILE" ]]; then
        pid=$(cat "$PID_FILE" 2>/dev/null || true)
    fi
    [[ "$pid" =~ ^[0-9]+$ ]] && printf '%s\n' "$pid"
    return 0
}

pid_alive() {
    [[ -n "${1:-}" ]] && kill -0 "$1" 2>/dev/null
}

# Fuehrt einen optionalen Hook-Befehl aus (leer = kein Hook).
# Fehler im Hook blockieren Start/Stop nicht.
run_hook() {
    local name="$1" cmd="$2"
    [[ -n "$cmd" ]] || return 0
    log_info "${name}: ${cmd}"
    # shellcheck disable=SC2086
    if ! bash -c "$cmd"; then
        log_warn "${name} fehlgeschlagen (Start/Stop faehrt trotzdem fort)"
    fi
}

# Wie run_hook, aber hoechstens einmal pro Skriptlauf (restart-Fall:
# stop_koolo + kill_existing duerfen den Hook nicht doppelt ausloesen).
PRE_STOP_FIRED=0
run_hook_once() {
    local name="$1" cmd="$2"
    [[ -n "$cmd" ]] || return 0
    if [[ "$name" == "PRE_STOP_CMD" && "$PRE_STOP_FIRED" == "1" ]]; then
        return 0
    fi
    run_hook "$name" "$cmd"
    if [[ "$name" == "PRE_STOP_CMD" ]]; then
        PRE_STOP_FIRED=1
    fi
}

# Ermittelt den Proton-Starter: PROTON_PATH > Steam Experimental > `proton` im PATH
resolve_proton() {
    local candidate
    for candidate in "$PROTON_PATH" "$STEAM_HOME/steamapps/common/Proton - Experimental/proton"; do
        if [[ -n "$candidate" && -f "$candidate" ]]; then
            printf '%s\n' "$candidate"
            return 0
        fi
    done
    if command -v proton >/dev/null 2>&1; then
        command -v proton
        return 0
    fi
    return 1
}

port_in_use() {
    if command -v ss >/dev/null 2>&1; then
        local out
        out=$(ss -tln 2>/dev/null || true)   # nicht direkt pipen: pipefail + grep -q => SIGPIPE-Falle
        grep -Eq "[:.]${API_PORT}[[:space:]]" <<<"$out"
    else
        (exec 3<>"/dev/tcp/127.0.0.1/${API_PORT}") 2>/dev/null
    fi
}

# Wartet, bis der API-Port frei ist. Return 0 = frei, 1 = noch belegt.
wait_port_free() {
    local i
    for ((i = 0; i < PORT_WAIT_SECONDS; i++)); do
        port_in_use || return 0
        sleep 1
    done
    port_in_use && return 1
    return 0
}

# --- Voraussetzungen -----------------------------------------------------------
check_prerequisites() {
    if [[ ! -d "$KOOLO_HOME" ]]; then
        log_error "Koolo-Verzeichnis nicht gefunden: $KOOLO_HOME"
        log_error "KOOLO_HOME setzen (Env oder $ENV_FILE)"
        return 1
    fi
    if [[ ! -f "$KOOLO_HOME/koolo.exe" ]]; then
        log_error "koolo.exe nicht gefunden in $KOOLO_HOME"
        return 1
    fi
    if [[ ! -d "$STEAM_HOME" ]]; then
        log_error "Steam-Verzeichnis nicht gefunden: $STEAM_HOME"
        return 1
    fi
    if [[ -n "$PROTON_PATH" && ! -f "$PROTON_PATH" ]]; then
        log_warn "PROTON_PATH zeigt ins Leere: $PROTON_PATH - versuche Fallbacks"
    fi
    if ! resolve_proton >/dev/null; then
        log_error "Proton nicht gefunden (PROTON_PATH setzen oder Proton - Experimental installieren)"
        return 1
    fi
    local tool
    for tool in taskset nice; do
        if ! command -v "$tool" >/dev/null 2>&1; then
            log_error "'$tool' nicht gefunden (util-linux / coreutils installieren)"
            return 1
        fi
    done
    command -v curl >/dev/null 2>&1 || log_warn "curl fehlt - graceful /stop wird uebersprungen"
    command -v ss   >/dev/null 2>&1 || log_warn "ss fehlt - Port-Check nutzt /dev/tcp-Fallback"
    return 0
}

# --- Log-Rotation --------------------------------------------------------------
rotate_log() {
    [[ -f "$LOG_FILE" ]] || return 0
    local log_size
    # GNU stat zuerst: dort bedeutet `stat -f` "Dateisystem-Info", nicht Dateigroesse
    log_size=$(stat -c%s "$LOG_FILE" 2>/dev/null || stat -f%z "$LOG_FILE" 2>/dev/null || echo 0)
    if [[ "$log_size" =~ ^[0-9]+$ ]] && (( log_size > MAX_LOG_SIZE )); then
        local backup_file="${LOG_FILE}.$(date +%Y%m%d_%H%M%S)"
        log_info "Rotiere Log (Groesse: $((log_size / 1024 / 1024)) MB)"
        mv "$LOG_FILE" "$backup_file"
        if command -v gzip >/dev/null 2>&1; then
            gzip "$backup_file" &
        fi
    fi
}

# --- Stoppen -------------------------------------------------------------------
# 1) Graceful ueber die Koolo-API
graceful_stop() {
    command -v curl >/dev/null 2>&1 || return 0
    if curl -s -m 2 "http://localhost:${API_PORT}/stop" >/dev/null 2>&1; then
        log_info "Graceful-Stop gesendet, warte ${STOP_GRACE_SECONDS}s..."
        sleep "$STOP_GRACE_SECONDS"
    fi
}

# 2) Launcher-Prozess aus dem PID-File (+ direkte Kinder)
kill_pidfile() {
    local pid timeout="$KILL_WAIT_SECONDS"
    pid=$(read_pid)
    if pid_alive "$pid"; then
        log_info "Beende Koolo-Launcher (PID: $pid)"
        pkill -TERM -P "$pid" 2>/dev/null || true
        kill -TERM "$pid" 2>/dev/null || true
        while (( timeout > 0 )) && pid_alive "$pid"; do
            sleep 1
            timeout=$((timeout - 1))
        done
        if pid_alive "$pid"; then
            log_warn "Prozess reagiert nicht auf TERM - SIGKILL"
            pkill -KILL -P "$pid" 2>/dev/null || true
            kill -KILL "$pid" 2>/dev/null || true
        fi
    fi
    rm -f "$PID_FILE"
}

# 3) Wine-Seite: pkill auf proton allein laesst koolo.exe samt Fenster und API-Port
#    als Zombie zurueck - eine zweite Instanz wuerde parallel Input senden
#    (Char faehrt in die Ecken der Map). Daher zusaetzlich koolo.exe direkt.
kill_wine_side() {
    pkill -f 'proton.*koolo\.exe' 2>/dev/null || true
    pkill -x koolo.exe 2>/dev/null || true
}

# Vor dem Start: alles Alte sicher weg, sonst harter Abbruch
kill_existing() {
    run_hook_once "PRE_STOP_CMD" "$PRE_STOP_CMD"
    graceful_stop
    kill_pidfile
    kill_wine_side
    if ! wait_port_free; then
        log_error "Port ${API_PORT} ist noch belegt (alte Koolo-Instanz lebt?) - Start abgebrochen."
        return 1
    fi
    return 0
}

stop_koolo() {
    run_hook_once "PRE_STOP_CMD" "$PRE_STOP_CMD"
    graceful_stop
    kill_pidfile
    kill_wine_side
    if wait_port_free; then
        log_info "Koolo gestoppt"
    else
        log_warn "Koolo gestoppt, aber Port ${API_PORT} ist noch belegt"
        return 1
    fi
}

# --- Starten -------------------------------------------------------------------
start_koolo() {
    check_prerequisites || return 1

    local compatdata="$STEAM_HOME/steamapps/compatdata/$STEAM_GAME_ID"
    local proton_path
    proton_path=$(resolve_proton)

    log_info "Starte Koolo RotW..."
    log_info "  Home:   $KOOLO_HOME"
    log_info "  Proton: $proton_path"
    log_info "  CPUs:   $TASKSET_CPUS (nice $NICE_LEVEL)"
    log_info "  Log:    $LOG_FILE"

    mkdir -p "$LOG_DIR"
    rotate_log

    export WINEPREFIX="${compatdata}/pfx"
    export STEAM_COMPAT_DATA_PATH="$compatdata"
    export STEAM_COMPAT_CLIENT_INSTALL_PATH="$STEAM_HOME"
    export SteamAppId="$STEAM_GAME_ID"
    export SteamGameId="$STEAM_GAME_ID"
    export KOOLO_NO_INJECTOR=1
    export PROTON_LOG=0
    export DISPLAY

    cd "$KOOLO_HOME" || { log_error "Wechsel nach $KOOLO_HOME fehlgeschlagen"; return 1; }

    # Start via Proton, damit derselbe wineserver wie beim Steam-gestarteten D2R genutzt wird.
    # Direktes `wine` erzeugt einen eigenen wineserver, der D2R nicht per OpenProcess oeffnen kann.
    nohup taskset -c "$TASKSET_CPUS" nice -n "$NICE_LEVEL" \
        "$proton_path" run "$KOOLO_HOME/koolo.exe" \
        >> "$LOG_FILE" 2>&1 &
    local koolo_pid=$!
    echo "$koolo_pid" > "$PID_FILE"

    sleep 2
    if ! pid_alive "$koolo_pid"; then
        log_error "Koolo-Prozess ist sofort beendet. Letzte Logzeilen:"
        tail -20 "$LOG_FILE" >&2 || true
        rm -f "$PID_FILE"
        return 1
    fi

    log_info "Koolo gestartet (PID: $koolo_pid)"
    log_info "Dashboard: http://localhost:${API_PORT}"
    run_hook "POST_START_CMD" "$POST_START_CMD"
}

# --- Status / Logs -------------------------------------------------------------
show_status() {
    local pid rc=1
    pid=$(read_pid)
    if pid_alive "$pid"; then
        log_info "Launcher laeuft (PID: $pid)"
        rc=0
    elif [[ -n "$pid" ]]; then
        log_warn "PID-File veraltet (PID $pid lebt nicht)"
    fi
    if pgrep -x koolo.exe >/dev/null 2>&1; then
        log_info "koolo.exe-Prozess aktiv"
        rc=0
    fi
    if port_in_use; then
        log_info "API-Port ${API_PORT} belegt"
    else
        log_info "API-Port ${API_PORT} frei"
    fi
    (( rc == 0 )) || log_info "Koolo laeuft nicht"
    return "$rc"
}

show_logs() {
    if [[ -f "$LOG_FILE" ]]; then
        tail -f "$LOG_FILE"
    else
        log_error "Logdatei nicht gefunden: $LOG_FILE"
        return 1
    fi
}

usage() {
    cat <<USAGE
Usage: $0 {start|stop|restart|status|logs}

Konfiguration: Env-Variablen oder $ENV_FILE
  KOOLO_HOME          Koolo-Installationsverzeichnis (in der Prefix)
  STEAM_HOME          Steam-Verzeichnis            (default: ~/.local/share/Steam)
  STEAM_GAME_ID       Steam-App-ID der Prefix      (default: 2998511494)
  PROTON_PATH         Proton-Starter ueberschreiben (default: Proton - Experimental, dann PATH)
  API_PORT            Koolo-API/Dashboard-Port     (default: 8087)
  LOG_DIR / LOG_FILE / PID_FILE
  TASKSET_CPUS        CPU-Bindung                  (default: 6-7,14-15)
  NICE_LEVEL          Prioritaet                   (default: 15)
  MAX_LOG_SIZE        Rotation ab Bytes            (default: 100 MB)
  STOP_GRACE_SECONDS, KILL_WAIT_SECONDS, PORT_WAIT_SECONDS
  POST_START_CMD      Hook nach erfolgreichem Start (z.B. Stream starten)
  PRE_STOP_CMD        Hook vor dem Stoppen (z.B. Stream beenden)
USAGE
}

main() {
    local command="${1:-start}"
    case "$command" in
        start)   kill_existing || exit 1; start_koolo ;;
        stop)    stop_koolo ;;
        restart) stop_koolo || true; kill_existing || exit 1; start_koolo ;;
        status)  show_status ;;
        logs)    show_logs ;;
        -h|--help|help) usage ;;
        *)       usage; exit 1 ;;
    esac
}

# Nur ausfuehren, wenn direkt aufgerufen (erlaubt `source` fuer Tests)
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi
