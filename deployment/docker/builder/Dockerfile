ARG go_builder_image
FROM $go_builder_image
STOPSIGNAL SIGINT
RUN apk add git gcc musl-dev make wget --no-cache && \
    mkdir /opt/cross-builder && \
    wget https://musl.cc/aarch64-linux-musl-cross.tgz -O /opt/cross-builder/aarch64-musl.tgz && \
    cd /opt/cross-builder && \
    tar zxf aarch64-musl.tgz -C ./  && \
    rm /opt/cross-builder/aarch64-musl.tgz
