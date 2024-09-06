# See https://medium.com/on-docker/use-multi-stage-builds-to-inject-ca-certs-ad1e8f01de1b
ARG certs_image=non-existing
ARG root_image=non-existing
FROM $certs_image AS certs
RUN apk update && apk upgrade && apk --update --no-cache add ca-certificates

FROM $root_image
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
EXPOSE 8880
ENTRYPOINT ["/vmalert-prod"]
ARG TARGETARCH
COPY vmalert-linux-${TARGETARCH}-prod ./vmalert-prod
