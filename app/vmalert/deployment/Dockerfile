ARG base_image=non-existing
FROM $base_image

EXPOSE 8880

ENTRYPOINT ["/vmalert-prod"]
ARG src_binary=non-existing
COPY $src_binary ./vmalert-prod
