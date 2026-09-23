# Runs the drive's original Java WebMotion applet inside a container and
# streams the screen to any modern browser via noVNC (HTML5, no Java needed
# on the viewing PC).
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      openjdk-8-jdk xvfb x11vnc novnc websockify openbox fonts-dejavu-core ca-certificates \
 && rm -rf /var/lib/apt/lists/*

ENV PATH=/usr/lib/jvm/java-8-openjdk-amd64/bin:$PATH \
    DRIVE_URL=http://192.168.2.100/ \
    SCREEN=1280x960x24 \
    TRUST_APPLET=1

COPY start.sh /usr/local/bin/start.sh
COPY all-permissions.policy /opt/webmotion/all-permissions.policy
RUN chmod +x /usr/local/bin/start.sh \
 && useradd -m webmotion
USER webmotion
WORKDIR /home/webmotion

EXPOSE 6080
CMD ["/usr/local/bin/start.sh"]
