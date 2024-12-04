FROM ubuntu:24.04

RUN \
  apt update && \
  apt install -y \
    systemd \
    systemd-journal-remote && \
  sed -i 's/# URL=/URL=http:\/\/victorialogs:9428\/insert\/journald/g' /etc/systemd/journal-upload.conf && \
  systemctl enable systemd-journal-upload.service
