# CLAUDE.md

Notes for anyone (human or AI) picking this repo up cold.

## What this repo is for

The user configures **Jenny Science XENAX** servo controllers (LINAX/ELAX axes).
Older XENAX firmware serves its setup GUI, *WebMotion*, as a **Java applet**.
The user's company uses **Windows laptops that cannot install Java**. The
goal is to configure the drive anyway. Nobody has run any of this against real
hardware yet.

Two independent tools:

| Dir | What | Runs on |
|---|---|---|
| `xenax-bridge/` | Go single-binary: talks the XENAX ASCII protocol (TCP 10001) and serves its own HTML5 UI on `127.0.0.1` | The Windows laptop (primary path) |
| `webmotion-vnc/` | Docker image: OpenJDK 8 `appletviewer` runs the original applet on Xvfb, streamed to a browser via noVNC | A separate Linux box / Raspberry Pi (not the laptop) |

## Architecture decisions (and why)

- **Talk to the drive's ASCII port instead of proxying the applet.** A plain HTTP
  proxy can't help: the browser still has no JVM. The applet itself drives the
  axis over the ASCII port 10001, so speaking that protocol directly removes Java
  entirely.
- **Go, single static `.exe`, stdlib only.** Corporate Windows laptops can't
  install runtimes (Java, Python, Node). Go cross-compiles to one file that needs
  no install or admin rights. The UI is `//go:embed`ded (`web/index.html`), so
  there's nothing to unpack. Keep it dependency-free; `go.mod` has no requires.
- **One shared, mutex-serialised TCP connection** (`drive.go`). The drive accepts
  a single ASCII client at a time, and replies have no request IDs, so commands
  must never interleave. The browser also serialises its own calls (`busy`
  promise chain in `index.html`), except STOP, which deliberately bypasses the
  JS queue (it still waits on the Go mutex).
- **Reply parsing:** the drive echoes the command, optionally sends a value, and
  ends with a prompt char: `>` ok, `?` unknown, `#` not executable now.
  `parseReply` strips the echo and waits for one of those as the last non-space
  char. Before each command `drain()` discards stray bytes (async event
  messages). **On timeout the connection is closed and reopened**, so a late
  reply can't be read as the answer to the next command.
- **Local-only HTTP server with CSRF + DNS-rebinding guard** (`sameOrigin` in
  `main.go`). It listens on `127.0.0.1` by default. POSTs need a matching
  `Origin` (or the `X-Requested-With: xenax-bridge` header), and the `Host`
  header must be an IP literal or `localhost`. Reason: any website open in the
  same browser could otherwise move the motor.
- **`-read-only` allows `T…` (tell) commands plus `SM` (stop).** Stop is always
  safe to allow.
- **Settings persisted in `os.UserConfigDir()`** (`%APPDATA%\xenax-bridge\settings.json`
  on Windows), editable from the page. Flags override them for that run. Reason:
  users double-click the `.exe`; they don't use a command line.
- **Network check** (`netcheck.go`): lists local IPv4 adapters and flags if none
  shares a subnet with the drive. The expected #1 support issue is a laptop
  Ethernet adapter without a static IP on the drive's subnet (factory default
  drive IP `192.168.2.100`).
- **Windows ergonomics:** `fatal()` waits for Enter on Windows so a double-clicked
  console doesn't vanish with the error. If 8080 is taken (and `-listen` wasn't
  given) it falls back to a random free port. The initial drive connect runs in
  the background so the browser opens immediately.
- **`/drive/` reverse proxy** to the drive's own web server. The applet won't
  run through it. It's there to read the original page and find applet
  `code`/`archive` names for `webmotion-vnc`'s `APPLET_PAGE` fallback. It uses
  `httputil.ReverseProxy.Rewrite` so the target follows settings changes.
- **`webmotion-vnc` uses `appletviewer`, not an old browser + plug-in.** OpenJDK 8
  still ships `appletviewer` (Ubuntu 24.04 `openjdk-8-jdk`), which runs
  `<applet>` tags without any browser. That's far simpler than Firefox ESR 52 +
  the Oracle plug-in. `TRUST_APPLET=1` loads an AllPermission policy, because
  industrial applets are often signed and need permissions. Even sandboxed
  applets may connect back to their codebase host (the drive).

## Conventions

- `gofmt`, `go vet ./...`, `go test ./...` from `xenax-bridge/` must be clean
  before pushing (CI runs the same).
- Conventional-ish commit subjects (`feat:`, `docs:`, `fix:` …, or a
  `component: summary` prefix).
- No model identifiers in commits, code or docs.
- UI text: be explicit that ASCII command names must be checked against the
  user's firmware manual. Never present them as verified.
- The UI uses `localStorage` only for per-browser conveniences (history,
  custom buttons, poll list), always wrapped in try/catch.

## Build, test, run

```
cd xenax-bridge
go test ./...
go run ./cmd/fakedrive              # simulator on 127.0.0.1:10001
go run . -drive 127.0.0.1 -no-browser
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o xenax-bridge.exe .
```

CI (`.github/workflows/build.yml`) tests, cross-compiles Windows/macOS/Linux
binaries (artifact `xenax-bridge`, plus a GitHub Release on `v*` tags) and builds
the Docker image. Version is injected with `-ldflags "-X main.version=…"`.

`cmd/fakedrive` is **not** a faithful drive model. It answers a few commands so
the UI can be exercised.

## Gotchas / things that didn't work

- **Official manuals were unreachable** from the dev sandbox (jennyscience.com,
  manualslib, mirrors all egress-blocked). Only port 10001 and the commands
  `PWC`, `TP`, `SP`, `AC` were confirmed from search snippets. `PW`, `PQ`, `REF`,
  `G`, `SM`, `TPS`, `TE` and the `> ? #` prompt format come from memory. Treat
  them as unverified.
- **Newer XENAX firmware has an HTML5 WebMotion** (no Java). A firmware update
  may make this whole repo unnecessary. Say so to users.
- **Docker Hub rate-limited (429)** in the sandbox. `mirror.gcr.io/library/ubuntu:24.04`
  re-tagged as `ubuntu:24.04` worked. Builds there need `--network host` and the
  proxy build-args. `dockerd` isn't running by default; start it manually.
- **`pkill -f <pattern>` killed the calling shell** whenever the pattern also
  appeared in the command line itself (exit 144). Use `pkill -f '/path/x[b]'`
  bracket tricks or PIDs.
- **Host-network container + a leftover local Xvfb** clashed on display `:0`
  (`server already running`). Clean `/tmp/.X0-lock` and old Xvfb/x11vnc first.
- `appletviewer` logs `Cannot run program "dbus-send"` in the container. That's
  harmless.
- Playwright from pip expects a newer bundled Chromium. Launch with
  `executable_path='/opt/pw-browsers/chromium'`.
- No Windows or wine in the sandbox: the `.exe` is cross-compiled and CI-built
  but has **never been executed on Windows**.
- An applet page that injects `<applet>` via JavaScript (e.g. `deployJava.js`)
  is invisible to `appletviewer`. Use `APPLET_PAGE` with
  `applet-page-template.html`.

## Open items

See `TODO.md`.
