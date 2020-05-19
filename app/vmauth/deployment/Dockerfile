ARG base_image
FROM $base_image

EXPOSE 8427

ENTRYPOINT ["/vmauth-prod"]
ARG src_binary
COPY $src_binary ./vmauth-prod
