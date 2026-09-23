#!/bin/sh
# Start a virtual X display, a VNC server on it, noVNC (HTML5 VNC client)
# on port 6080, and the JDK 8 appletviewer pointed at the drive's web page.
set -eu

DISPLAY_NUM=:0
export DISPLAY=$DISPLAY_NUM

Xvfb "$DISPLAY_NUM" -screen 0 "${SCREEN:-1280x960x24}" -nolisten tcp &
for _ in $(seq 1 50); do
    [ -e /tmp/.X11-unix/X0 ] && break
    sleep 0.1
done
openbox >/dev/null 2>&1 &

set -- -display "$DISPLAY_NUM" -forever -shared -localhost -rfbport 5900 -quiet
if [ -n "${VNC_PASSWORD:-}" ]; then
    mkdir -p "$HOME/.vnc"
    x11vnc -storepasswd "$VNC_PASSWORD" "$HOME/.vnc/passwd" >/dev/null
    set -- "$@" -rfbauth "$HOME/.vnc/passwd"
else
    set -- "$@" -nopw
fi
x11vnc "$@" &

websockify --web /usr/share/novnc 6080 localhost:5900 >/dev/null 2>&1 &

JOPTS=""
if [ "${TRUST_APPLET:-1}" = "1" ]; then
    JOPTS="-J-Djava.security.policy=/opt/webmotion/all-permissions.policy"
fi

# APPLET_PAGE (optional) lets you point at a hand-written HTML file with an
# <applet> tag if the drive's own page builds the tag with JavaScript.
PAGE="${APPLET_PAGE:-$DRIVE_URL}"

echo "WebMotion viewer: open http://<this-host>:6080/vnc.html?autoconnect=1&resize=scale"
echo "Loading applet page: $PAGE"
# Restart the viewer if the applet window is closed or the drive reboots.
while true; do
    # shellcheck disable=SC2086
    appletviewer $JOPTS -J-Dsun.java2d.xrender=false "$PAGE" || true
    echo "appletviewer exited; restarting in 3 s"
    sleep 3
done
