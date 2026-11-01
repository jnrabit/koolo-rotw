# Koolo RotW — Verbesserungsstatus (Wahrheitsstand)

Dieses Dokument ist der aktuelle Stand der Verbesserungswelle auf Branch
`improve/security-and-quality`. Es ersetzt die veraltete Teildokumentation
(`KOOLO_BOT_IMPROVEMENTS_SUMMARY.md` deckte nur Commits 1–5 ab) und bildet
die Ausgangsanalyse (`ANALYSIS_ORIGINAL.md`) gegen den realen Code ab.

Stand: 2026-11-01 · Branch: `improve/security-and-quality` · Diff vs. `main`:
37 Dateien, +1.946 / −184 Zeilen, 9 neue Dateien

Remotes: `origin` = `rubensadev/koolo` (Upstream), `own` = `jnrabit/koolo-rotw` (Fork)

---

## 1. Commit-Log (main..HEAD)

| # | Commit | Datum | Inhalt |
|---|--------|-------|--------|
| 1 | `136b37b` | 2026-10-24 | Security: CORS-Whitelist für WebSocket, Panic-Recovery wird geloggt statt verschluckt |
| 2 | `aebf75b` | 2026-10-24 | `stash.go`: 7× `fmt.Printf` → `ctx.Logger.Debug` |
| 3 | `1fb2b52` | 2026-10-24 | `game`-Paket: `fmt.*` → `slog.Debug/Warn` |
| 4 | `e2af795` | 2026-10-24 | Test-Framework: `bot_test.go`, `manager_test.go`, `websocket_test.go` |
| 5 | `c34058c` | 2026-10-24 | Deployment: `log_rotation.go`, `shutdown.go`, `scripts/start_koolo_rotw.sh`, `docs/DEPLOYMENT_GUIDE.md` |
| 6 | `1b3be9c` | 2026-10-24 | Fixes aus Code-Review |
| 7 | `b250b48` | 2026-10-24 | Tests mit echten Testfällen verbessert (bot_test auf 205 Z., websocket_test 200 Z.) |
| 8 | `24de7c0` | 2026-10-27 | gofmt-Ausrichtung `websocket_test.go` |
| 9 | `e3f3fef` | 2026-10-28 | **Wine/Proton/Linux-Kompatibilität** (siehe Abschnitt 2) |

Commit 9 ist der inhaltlich größte und war in der ursprünglichen
PR-Zusammenfassung nicht abgebildet.

---

## 2. Wine/Proton/Linux-Welle (`e3f3fef`) im Detail

- **Input:** `SendInput`/`mouse_event` statt `PostMessage`/`SendMessage` —
  D2Rs DirectInput empfängt unter Wine hardware-Level-Events
  (`mouse.go`, `keyboard.go`, `utils/winproc/user32.go`)
- **Memory:** `/proc/<pid>/mem`-Fallback + Linux-PID-Discovery — der
  Spielreader hängt sich unter Wine/Proton an D2R
  (`memory_reader.go`, `bot/manager.go`, `server/http_server.go`)
- **Koordinaten:** fixiertes virtuelles 1280×720-Zentrum, Skalierung auf die
  echte Fenstergröße bei Klick/Bewegung (`ui/screen_cords.go`,
  `pather/utils.go`, `action/run_walk.go`, Game-Manager-Exit-Flow)
- **Robustheit:** Headless-WebView-Fallback, Panic-Recovery im Manager und
  Crash-Detector, `KOOLO_NO_INJECTOR`-Modus, Keybinding-Save-Dir-Fallback,
  Legacy-Mode-Kurzschluss für D2R 2.4+
- **DIAG-Logging** zum Trace von Startup/Attach/Menu-Flow

Wichtig: Die Linux-Anbindung lebt zu einem großen Teil in `../d2go-local`
(Fork der d2go-Library, per `replace` in `go.mod` eingebunden). Dieser Fork
ist **kein Git-Repo** — er ist damit die einzige wichtige Komponente des
Setups ohne Versionsgeschichte.

---

## 3. Status-Matrix: Analyse → Ist

Legende: ✅ erledigt · ⚠️ teilweise · ❌ offen

### 3.1 Security

| Punkt | Nachweis | Status |
|---|---|---|
| WebSocket-CORS: `CheckOrigin` gab `true` zurück | `http_server.go:80–90` — Origin-Parsing, nur localhost/127.0.0.1/::1 | ✅ |
| Bare `recover()` in `bot.go` verschluckt Panics | `bot.go:223–225` — recovered + `slog.Error` | ✅ |
| Kein Auth auf Local-Server | unverändert | ❌ |
| Kein Rate-Limiting auf API | unverändert | ❌ |
| Pprof-Import in `main.go` | unverändert (kein sichtbarer Endpoint, Import aber noch da) | ⚠️ |

### 3.2 Logging

| Punkt | Nachweis | Status |
|---|---|---|
| `fmt.Printf` in `stash.go` | 8× entfernt | ✅ |
| `fmt.*` in `game/manager.go`, `packet_sender.go` | durch `slog` ersetzt | ✅ |
| `fmt.*` in `fireeye.go`, `tristramearlygoldfarm.go`, `sorceress_leveling.go` | durch `slog` ersetzt | ✅ |
| Restbestand `fmt.Print*` im Code | noch **5 Stellen** (ohne Tests) | ⚠️ |
| HTTP-Request-Logging-Middleware | kein Middleware-Muster im Server vorhanden | ❌ |

### 3.3 Fehlerbehandlung / Panics

| Punkt | Nachweis | Status |
|---|---|---|
| `scheduler.go`: State-Save ignoriert Fehler | `scheduler.go:688/694` behandelt `MarshalIndent` + `WriteFile` | ✅ |
| `scheduler.go`: History-Save ignoriert Fehler | `scheduler.go:744/745` — weiterhin `data, _ = json.MarshalIndent(...)` und `os.WriteFile` ohne Check | ❌ |
| `panic()` als Control-Flow in `chicken.go` | `chicken.go:19,23,27,31,69` unverändert | ❌ |
| `panic("Bot is stopped")` in `context.go` | `context.go:229` unverändert | ❌ |

### 3.4 Concurrency / Performance

| Punkt | Nachweis | Status |
|---|---|---|
| Panic-Recovery im High-Priority-Loop | `bot.go:223–225` | ✅ |
| Goroutine-Lecks in `main.go` | `main.go:128` `go scheduler.Start()` ohne Context; zwei anonyme `go func(){ for {`-Schleifen (`196`, `237`) ohne `ctx.Done()`-Select | ❌ |
| `context.Get()` via `runtime.Stack()` | `context.go:157,168–170,194,200` — 430+ Aufrufe pro Lauf, jedes Mal Stack-Parse | ❌ |
| `logFileHandler` ohne Mutex | `cmd/koolo/log/logger.go:12,15–23` — Race bei Flush/Close | ❌ |
| Pfad-Konkatenation im Logger | `logger.go:46` — `logDir + "/" + fileName` statt `filepath.Join` | ❌ |

### 3.5 Architektur

| Punkt | Nachweis | Status |
|---|---|---|
| `http_server.go` als Monolith | jetzt **4.776 Zeilen** in einer Datei (in der Analyse noch 4.700+) | ❌ |
| `replace d2go => ../d2go-local` | `go.mod:50` — Build bricht ohne Geschwister-Ordner; Fork nicht versioniert | ❌ |
| Config-Validierung beim Start | unverändert | ❌ |
| Totcode `buildID`/`buildTime` | nicht adressiert | ❌ |
| Ungeseedetes `math/rand` (Fenster-Titel-Rotation) | `main.go:240,245` | ❌ |

### 3.6 Deployment / Startskripte

| Punkt | Nachweis | Status |
|---|---|---|
| Log-Rotation | `internal/utils/log_rotation.go` (124 Z.) — Size/Backups/Age/gzip | ✅ |
| Graceful Shutdown (Go) | `internal/utils/shutdown.go` (137 Z.) — Handler-Stack, SIGINT/SIGTERM, Timeout | ✅ |
| Startskript mit Env-Vars, PID-File, Rotation | `scripts/start_koolo_rotw.sh` (325 Z.) — incl. Proton-Zombie-Schutz (`pkill -x koolo.exe`), Port-Warte-Loop, graceful `/stop` | ✅ |
| Deployment-Doku | `docs/DEPLOYMENT_GUIDE.md` (270 Z.) — noch ohne Wine-Weg aus Commit 9 | ⚠️ |
| Root-Skript (`projekte/koolo bot/start_koolo_rotw.sh`) | durch 3-Zeilen-Wrapper auf Repo-Skript ersetzt; Original unter `start_koolo_rotw.sh.old` | ✅ |
| Persönliche Pfade im Repo | `config/koolo-rotw.env.example` + `~/.config/koolo-rotw.env` (per `source`) | ✅ |

### 3.7 Tests

| Punkt | Nachweis | Status |
|---|---|---|
| Minimalabdeckung (nur `astar_test.go`) | jetzt 4 Testdateien: `bot_test.go` (205 Z.), `manager_test.go` (114 Z.), `websocket_test.go` (200 Z.), `astar_test.go` | ✅ |
| Tests kompilieren unter Linux | `go vet`/Build schlägt fehl: `golang.org/x/sys/windows` — `build constraints exclude all Go files`. Die Wine/Linux-Arbeit ist von keinem Test gedeckt | ❌ |
| Integrationstests | keine | ❌ |

---

## 4. Offene Punkte, priorisiert

**Quick Wins (klein, sicher):**
1. `scheduler.go:744/745` — Fehlerbehandlung wie in 688/694
2. Verbliebene 5× `fmt.Print*` aufräumen
3. `cmd/koolo/log/logger.go`: Mutex um `logFileHandler` + `filepath.Join`

**Mittelfristig:**
4. Tests Linux-fähig machen (Build-Tags `windows`/`linux`, Interfaces für
   Input/Memory, erste logiknahe Tests ohne Syscalls)
5. Goroutine-Lecks in `main.go` an `ctx.Done()` binden
6. `d2go-local` als eigenes Git-Repo initialisieren (Fork versionieren)
7. Deployment-Guide um den Wine/Linux-Weg (`e3f3fef`, `scripts/start_koolo_rotw.sh`) ergänzen

**Große Refactors (nur mit Testnetz):**
8. `context.Get()` schrittweise migrieren (zuerst goroutine-ID cachen, dann
   expliziten Context parametrisieren)
9. `http_server.go` aufspalten
10. Panic-als-Flow in `chicken.go`/`context.go` durch Fehler-Rückgaben ersetzen

**Merge-Strategie:** `improve/security-and-quality` → `main`. Vor dem Merge
sinnvoll: Quick Wins 1–3, Doku-Punkt 7. Upstream-tauglich (`rubensadev/koolo`)
sind CORS-, Logging- und Shutdown-Fixes; Wine/Linux und `d2go-local` bleiben
fork-spezifisch.

---

## 5. Verwandte Dokumentation

- `docs/DEPLOYMENT_GUIDE.md` — Start/Stop/Rotation/Systemd
- `docs/STREAMING.md` — Schwesterprojekt `d2r_streaming` (GPU-Streaming, separat)
- `docs/ANALYSIS_ORIGINAL.md` — Ausgangsanalyse (opencode-Arbeitsstand 2026-10-24)
- `docs/IMPROVEMENTS_SUMMARY_ORIGINAL.md` — PR-Zusammenfassung Commits 1–5 (Teilmenge dieses Dokuments)
- `config/koolo-rotw.env.example` — Vorlage für `~/.config/koolo-rotw.env` (inkl. Hook-/STREAM_-Beispiele)
