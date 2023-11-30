# See https://medium.com/on-docker/use-multi-stage-builds-to-inject-ca-certs-ad1e8f01de1b
ARG certs_image
ARG root_image
FROM $certs_image as certs

RUN apk update && apk upgrade && apk --update --no-cache add ca-certificates

FROM $root_image

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# fixes https://security.alpinelinux.org/vuln/CVE-2023-5363
# can remove after alpine releases fixed container image
RUN apk update && apk upgrade --no-cache libcrypto3 libssl3
#