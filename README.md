# jenny-science-java-tool

Ways to configure a **Jenny Science XENAX** servo controller (LINAX / ELAX axes)
from a PC that **cannot install Java**.

Older XENAX firmware serves the *WebMotion* setup tool as a Java applet.
Modern browsers can't run applets, and many corporate laptops can't install
Java. This repo gives you two workarounds:

| | [A. `xenax-bridge`](#a-xenax-bridge-no-java-at-all) | [B. `webmotion-vnc`](#b-webmotion-vnc-the-original-java-gui-streamed-to-your-browser) |
|---|---|---|
| What you see | A new HTML5 control page (terminal, power/reference, move, live status, custom buttons) | The **original** WebMotion Java GUI, shown in a browser tab |
| Needs on the Windows laptop | One small `.exe` file, nothing installed, no admin rights | Just a web browser |
| Needs elsewhere | Nothing | A machine that runs Docker (Linux PC, Raspberry Pi 4/5, VM, colleague's PC) on the drive's network |
| Covers | Everything the ASCII command set can do | Everything WebMotion can do (graphs, tuning, parameter pages…) |

> **First check your firmware.** Newer XENAX firmware ships an **HTML5
> WebMotion** that needs no Java at all. If you can get a firmware update
> (from Jenny Science support, or on a PC that can run the current tool), that
> is the cleanest fix.

> **Corporate IT:** running an unsigned `.exe` or connecting a Docker box to the
> machine network may still need IT approval. Both tools only talk to the drive's
> IP address, and `xenax-bridge` only listens on `127.0.0.1` by default.

---

## A. `xenax-bridge` (no Java at all)

The XENAX drive has an ASCII command interface on **TCP port 10001** (the same
channel the Java applet uses). `xenax-bridge` connects to that port and serves a
control page to your browser at `http://127.0.0.1:8080`.

```
 browser ──HTTP──▶ xenax-bridge (your laptop) ──TCP 10001──▶ XENAX drive
                         └──── /drive/ ──HTTP 80──▶ drive's own web pages
```

### Get it (Windows)

* **Download:** GitHub → *Actions* → latest **build** run → artifact
  `xenax-bridge` (or a *Release* if one is tagged). Use
  `xenax-bridge-windows-amd64.exe` (`-arm64` only for ARM laptops such as
  Surface Pro X / Snapdragon). It's a single file. Put it anywhere, e.g. your
  Desktop or a USB stick. Nothing is installed and no admin rights are needed.
* **Or build it** (Go 1.24+): `cd xenax-bridge && go build .`

The `.exe` isn't code-signed, so Windows SmartScreen may say *"Windows protected
your PC"*. If your IT policy allows it, click **More info → Run anyway**.
Otherwise ask IT to allow the file (the `SHA256SUMS.txt` next to it helps them
check it).

### Run it

1. **Connect the laptop to the drive** with an Ethernet cable (or a USB‑Ethernet
   adapter), directly or through a switch.
2. **Give that adapter an address in the drive's subnet.** The factory default
   drive IP is `192.168.2.100`, so use for example `192.168.2.10`, mask
   `255.255.255.0`:
   *Settings → Network & internet → Ethernet → (adapter) → IP assignment → Edit →
   Manual → IPv4 on*. Leave gateway and DNS empty.
   This step may need admin rights. If it's blocked, ask IT to set it once for that
   adapter. Your normal Wi‑Fi/VPN connection is not affected.
3. **Double-click `xenax-bridge-windows-amd64.exe`.** A console window opens
   (keep it open; closing it quits the bridge) and your default browser opens
   the control page, usually `http://127.0.0.1:8080`. If port 8080 is taken,
   another free port is chosen and shown in the console.
4. In the **Connection** box, enter the drive's IP and click **Save & connect**.
   It is remembered next time (in `%APPDATA%\xenax-bridge\settings.json`).
   The box lists this PC's network adapters and warns with ✗ if none is in the
   drive's subnet, which is the most common reason it won't connect.

Windows Firewall won't prompt, because the bridge only listens on `127.0.0.1` and
only makes outgoing connections to the drive.

Command-line options (optional; e.g. for a desktop shortcut with
`-read-only` added to the *Target*):

| Option | Default | Meaning |
|---|---|---|
| `-drive` | last used, else `192.168.2.100` | Drive IP / hostname |
| `-ascii-port` | `10001` | ASCII command port |
| `-http-port` | `80` | Drive web server port (for `/drive/`) |
| `-listen` | `127.0.0.1:8080` | Where the local page is served |
| `-read-only` | off | Only allow `T…` (tell/query) commands and `SM` (stop) |
| `-timeout` | `2s` | Reply timeout per command |
| `-no-browser` | off | Don't open the browser automatically |

### What's on the page

* **Terminal:** type any XENAX ASCII command, see the reply. `>` = accepted,
  `?` = unknown command, `#` = not executable right now. ↑/↓ recalls history.
* **Power & reference:** `PW`, `PWC`, `PQ`, `REF`, `TE`.
* **Motion:** set speed (`SP`), acceleration (`AC`), go to position (`G`), jog ±.
* **Live status:** polls a list of queries (default `TP, TPS, TE`); edit the list.
* **Custom buttons:** save your own commands as buttons.
* **STOP:** sends `SM` straight away, even while polling.
* **Drive's original web page ↗** (`/drive/`): the drive's own web server,
  passed through. The Java applet itself still won't run there. It's useful for
  reading the page and finding the applet's file names for option B.

> ⚠️ Command names follow the XENAX ASCII protocol as published in the
> *XENAX Xvi* manuals. Check them against the manual for **your** firmware
> before sending motion commands, and keep the axis clear. Use `-read-only` to
> explore safely.

### Try it without hardware

```
cd xenax-bridge
go run ./cmd/fakedrive            # tiny simulator on 127.0.0.1:10001 (leave running)
go run . -drive 127.0.0.1         # in a second terminal
```

---

## B. `webmotion-vnc` (the original Java GUI, streamed to your browser)

> **Windows note:** the Windows laptops only need a browser for this option. The
> container runs on a *separate* always-on box (a small Linux PC or Raspberry Pi
> by the machine is ideal). Running it on a Windows laptop would need Docker
> Desktop + WSL2, which requires admin rights and has the same IT hurdle as Java.

If you need the full WebMotion GUI, run Java on **another** machine and view it
from the laptop. The container runs the OpenJDK 8 `appletviewer` (it runs applets
without a browser plug-in) on a virtual screen and streams it with noVNC.

```
 laptop browser ──HTTP/WebSocket 6080──▶ Docker host: appletviewer (Java 8) ──▶ XENAX drive
```

```bash
cd webmotion-vnc
# edit DRIVE_URL in docker-compose.yml if the drive is not 192.168.2.100
docker compose up -d --build
```

Then on the laptop open
`http://<docker-host-ip>:6080/vnc.html?autoconnect=1&resize=scale`.

The Docker host needs network access to the drive. A Raspberry Pi with two
network ports, or a Pi on the machine network plus Wi‑Fi to the office, works well.

Settings (in `docker-compose.yml`):

* `DRIVE_URL`: the drive's web address, loaded into `appletviewer`.
* `TRUST_APPLET=1`: grants the applet full permissions, the same as clicking
  *Run* on a signed applet in an old browser. Set to `0` for the standard sandbox.
  The sandbox still lets the applet connect back to the drive.
* `VNC_PASSWORD`: set this if others share the network.
* `APPLET_PAGE`: if the drive's page writes its `<applet>` tag with JavaScript,
  `appletviewer` won't find it. Copy `applet-page-template.html`, fill in the
  `code`/`archive` names (look at the page source via `xenax-bridge`'s
  `/drive/` link), mount it and point `APPLET_PAGE` at it.

If the applet window is closed or the drive reboots, the viewer restarts on its own
after 3 seconds.
