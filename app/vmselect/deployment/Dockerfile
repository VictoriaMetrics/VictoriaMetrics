ARG base_image=non-existing
FROM $base_image

EXPOSE 8481

ENTRYPOINT ["/vmselect-prod"]
ARG src_binary=non-existing
COPY $src_binary ./vmselect-prod
