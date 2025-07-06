ARG base_image=non-existing
FROM $base_image

EXPOSE 9429

ENTRYPOINT ["/vlagent-prod"]
ARG src_binary=non-existing
COPY $src_binary ./vlagent-prod
