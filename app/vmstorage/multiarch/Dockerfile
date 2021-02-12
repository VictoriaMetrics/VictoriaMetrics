# See https://medium.com/on-docker/use-multi-stage-builds-to-inject-ca-certs-ad1e8f01de1b
ARG certs_image
ARG root_image
FROM $certs_image as certs
RUN apk --update --no-cache add ca-certificates

FROM $root_image
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
EXPOSE 8482
EXPOSE 8400
EXPOSE 8401
ENTRYPOINT ["/vmstorage-prod"]
ARG TARGETARCH
COPY vmstorage-${TARGETARCH}-prod ./vmstorage-prod
