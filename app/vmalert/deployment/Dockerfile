ARG base_image
FROM $base_image

EXPOSE 8880

ENTRYPOINT ["/vmalert-prod"]
ARG src_binary
COPY $src_binary ./vmalert-prod
