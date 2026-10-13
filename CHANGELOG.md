# Changelog

All notable changes to this project. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### 2026-10-13
#### Added
- `CLAUDE.md`: architecture decisions, conventions, gotchas and failed
  approaches for future contributors.
- `TODO.md`: open items (unverified commands, untested on hardware/Windows,
  code signing, …).
- This changelog.

### 2026-09-23
#### Added
- **xenax-bridge**: Go single-binary bridge to the XENAX ASCII interface
  (TCP 10001) with an embedded HTML5 UI on `127.0.0.1:8080`. It has a terminal,
  power/reference, speed/accel/go/jog, live status polling, custom buttons and a
  STOP button.
  - `/drive/` reverse proxy to the drive's own web server.
  - `-read-only` mode (only `T…` queries and `SM`).
  - CSRF and DNS-rebinding protection on the local API.
  - `cmd/fakedrive` simulator and unit tests.
- **xenax-bridge, Windows workflow**:
  - Drive IP and ports can be set from the page and are remembered in
    `%APPDATA%\xenax-bridge\settings.json`.
  - A network check warns when no adapter is in the drive's subnet, and the page
    shows Windows static-IP steps.
  - Falls back to a free port when 8080 is busy.
  - The console stays open on fatal errors.
  - The drive connects in the background so the browser opens immediately.
- **webmotion-vnc**: Docker image that runs the original WebMotion applet in
  OpenJDK 8 `appletviewer` and streams it to any browser via noVNC.
  - Options: `TRUST_APPLET`, `VNC_PASSWORD`, `APPLET_PAGE`.
  - The viewer restarts on its own if it closes.
- CI: tests, cross-compiled Windows/macOS/Linux binaries (GitHub Release on `v*`
  tags), Docker image build.
- README covering both options, with Windows-specific setup.
