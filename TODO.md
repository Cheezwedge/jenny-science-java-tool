# Open items

No `TODO`/`FIXME` markers exist in the code. These are the known gaps, roughly
in priority order.

- [ ] **Verify ASCII command names and reply format against the XENAX manual**
      for the user's firmware: `PW`, `PQ`, `REF`, `G`, `SM`, `TPS`, `TE`, and the
      `>`/`?`/`#` prompt characters. Only port 10001 and `PWC`, `TP`, `SP`, `AC`
      are confirmed. Adjust button labels in `xenax-bridge/web/index.html` and
      `isQuery` in `main.go` if needed.
- [ ] **First run on real hardware.** Neither tool has talked to a real drive.
      Start with `-read-only`.
- [ ] **First run of the `.exe` on Windows** (double-click flow, browser launch
      via `rundll32`, `%APPDATA%` settings, Enter-to-close on fatal errors).
- [ ] **Asynchronous event messages.** `drain()` drops unsolicited bytes before
      each command, but an event arriving *during* a reply would be mixed into
      it. Revisit once real drive output is seen.
- [ ] **Code-sign the Windows `.exe`** (or get it allow-listed by IT) to avoid
      SmartScreen warnings. Needs a certificate the user's company provides.
- [ ] **Check the drive's firmware version.** If an update to the HTML5
      WebMotion firmware is possible, it removes the need for both tools.
- [ ] `webmotion-vnc`: confirm the drive's page uses a plain `<applet>`/`<object>`
      tag that `appletviewer` can see; otherwise fill in
      `applet-page-template.html`.
- [ ] `webmotion-vnc`: the arm64 (Raspberry Pi) image build hasn't been tried; CI
      builds amd64 only.
