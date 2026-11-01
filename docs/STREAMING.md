# Streaming (d2r_streaming)

Separates Projekt für GPU-beschleunigtes Streaming von D2R-Fenstern unter
Linux (Wayland/X11, AMD ROCm bzw. NVIDIA). **Kein Teil des Koolo-Bots** —
gleichrangiger Nachbar-Ordner, eigene Sprache (Python/C++/OpenCL), eigene
Git-Historie.

## Repo

- Lokal: `~/d2r_streaming/`
- Remote: `https://github.com/jnrabit/d2r-streaming` (privat)
- Geschwister von `koolo-rotw` und `d2go-local` unter `~/projekte/` bzw. Home

## Was es kann (Kurzfassung)

- **Chaoskern V3.5**: hybride C++/OpenCL-Engine (8D-Hyperchaos, fraktale
  Kompression, thermisch-adaptives Throttling)
- **Video**: unified Stream (Video+Audio), Wayland-Capture (Portal/PipeWire),
  RTSP/RTMP-Ausgabe, H.264-Recording
- **Audio**: GStreamer-Pipeline
- **Monitoring**: TUI-Dashboard, Web-Dashboard (Port 5000, standardmäßig aus)

Details: `README.md`, `FEATURES.md`, `CHAOSKERN_EXPLAINED.md` im Streaming-Repo.

## Start (manuell)

```bash
cd ~/d2r_streaming
make build          # libhyper_v3_5.so bauen
make run            # Testlauf 720p30
make stream-1080p   # voller Stream
```

## Ports

| Port | Zweck |
|------|--------|
| 5000 | optionales Web-Dashboard des Streamers |
| 8554 | RTSP (falls aktiviert) |
| 8087 | Koolo-API/Dashboard (anderes Projekt!) |

## Kopplung an Koolo (bewusst locker)

Es gibt **keinen** gemeinsamen Code und **keine** gemeinsamen Abhängigkeiten.
Die einzige Verbindungsstelle ist der generische Hook im Koolo-Startskript
(`scripts/start_koolo_rotw.sh`), gesteuert über `~/.config/koolo-rotw.env`:

```bash
# Beispiele — an eigene Pfade/Prozesse anpassen:
#POST_START_CMD="cd $HOME/d2r_streaming && nohup python3 d2r_unified_stream.py --mode monitoring &"
#PRE_STOP_CMD="pkill -f d2r_unified_stream || true"
```

Regeln:

- Stream-Variablen immer mit `STREAM_`-Präfix benennen (keine Namenskollision
  mit Koolo-Env).
- Kein Streaming-Status-Endpoint im Go-Server, solange der lokale Server ohne
  Auth läuft.
- Der Bot priorisiert Stabilität (taskset CPUs 6–7,14–15, nice 15). Ein
  GPU-Encoder konkurriert um Ressourcen — Ruckler zuerst am Stream- oder
  Capture-Parametern suchen, nicht am Bot.

## Warum kein Git-Submodul / keine tiefe Integration

- Go vs. Python/C++: keine Compile-Time-Kopplung möglich.
- Streaming ist optional; der Bot muss ohne davon stabil laufen.
- Prozess-Hooks reichen für Start/Stop und halten beide Repos unabhängig
  deploybar.
