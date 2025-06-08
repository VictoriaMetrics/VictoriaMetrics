ARG go_builder_image=non-existing
FROM $go_builder_image
STOPSIGNAL SIGINT
RUN apk add git gcc musl-dev make wget --no-cache && \
    mkdir /opt/cross-builder && \
    cd /opt/cross-builder && \
    for arch in aarch64 x86_64; do \
        wget \
            https://github.com/VictoriaMetrics/muslcc-mirror/releases/download/v1.0.0/${arch}-linux-musl-cross.tgz \
            -O /opt/cross-builder/${arch}-musl.tgz \
            --no-verbose && \
        tar zxf ${arch}-musl.tgz -C ./  && \
        rm /opt/cross-builder/${arch}-musl.tgz; \
    done
