# NOTE: Once the changes made in this file commited, run make publish-base

# See https://medium.com/on-docker/use-multi-stage-builds-to-inject-ca-certs-ad1e8f01de1b
FROM alpine:3.22.1 AS certs

RUN apk update && apk upgrade && apk --update --no-cache add ca-certificates

FROM alpine:3.22.1

# Temporary fix for CVE-2025-9230, CVE-2025-9231, CVE-2025-9232 until Alpine releases a fixed image.
RUN apk add --no-cache 'libcrypto3=3.5.4-r0' 'libssl3=3.5.4-r0'

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
