# See https://medium.com/on-docker/use-multi-stage-builds-to-inject-ca-certs-ad1e8f01de1b
FROM alpine:3.10 as base

RUN apk --update --no-cache add ca-certificates

FROM scratch

COPY --from=base /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
