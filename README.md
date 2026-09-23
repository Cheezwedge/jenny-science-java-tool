# jenny-science-java-tool

Ways to configure a **Jenny Science XENAX** servo controller (LINAX / ELAX axes)
from a PC that **cannot install Java**.

Older XENAX firmware serves the *WebMotion* setup tool as a Java applet.
Modern browsers can't run applets, and many corporate laptops can't install
Java. This repo gives you two workarounds:

| | [A. `xenax-bridge`](#a-xenax-bridge-no-java-at-all) | [B. `webmotion-vnc`](#b-webmotion-vnc-the-original-java-gui-streamed-to-your-browser) |
|---|---|---|
| What you see | A new HTML5 control page (terminal, power/reference, move, live status, custom buttons) | The **original** WebMotion Java GUI, shown in a browser tab |
| Needs on the laptop | One small `.exe` file, nothing installed | Just a web browser |
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

### Get it

* **Download:** GitHub → *Actions* → latest **build** run → artifact
  `xenax-bridge` (or a *Release* if one is tagged). Pick
  `xenax-bridge-windows-amd64.exe` for a normal Windows laptop.
* **Or build it** (Go 1.24+): `cd xenax-bridge && go build .`

### Run it

1. Connect the laptop to the drive (Ethernet). Give your network adapter a static
   IP in the drive's subnet, e.g. `192.168.2.10 / 255.255.255.0` if the drive is
   at the factory default `192.168.2.100`. (Changing adapter settings may itself
   need admin rights; a USB‑Ethernet adapter your IT has approved helps.)
2. Start the bridge. Double-clicking uses the default drive IP; otherwise run from a
   command prompt:

   ```
   xenax-bridge-windows-amd64.exe -drive 192.168.2.100
   ```

3. Your browser opens `http://127.0.0.1:8080`. The green dot shows the ASCII
   connection is up.

| Option | Default | Meaning |
|---|---|---|
| `-drive` | `192.168.2.100` | Drive IP / hostname |
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
go run ./cmd/fakedrive &          # tiny simulator on 127.0.0.1:10001
go run . -drive 127.0.0.1
```

---

## B. `webmotion-vnc` (the original Java GUI, streamed to your browser)

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
