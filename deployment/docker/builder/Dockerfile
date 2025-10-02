ARG go_builder_image=non-existing
FROM $go_builder_image
STOPSIGNAL SIGINT
RUN apt update && apt install -y \
    gcc-x86-64-linux-gnu \
    gcc-aarch64-linux-gnu
