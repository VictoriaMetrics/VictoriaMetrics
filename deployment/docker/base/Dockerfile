# See https://medium.com/on-docker/use-multi-stage-builds-to-inject-ca-certs-ad1e8f01de1b
ARG certs_image=non-existing
ARG root_image=non-existing
FROM $certs_image AS certs

RUN apk update && apk upgrade && apk --update --no-cache add ca-certificates

FROM $root_image

# Temporary fix for CVE-2025-9230, CVE-2025-9231, CVE-2025-9232 until Alpine releases a fixed image.
RUN apk add --no-cache 'libcrypto3=3.5.4-r0' 'libssl3=3.5.4-r0'

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
