# See https://medium.com/on-docker/use-multi-stage-builds-to-inject-ca-certs-ad1e8f01de1b
FROM alpine:3.10 as base

RUN apk --update --no-cache add ca-certificates

RUN mkdir /future-tmp

FROM scratch

COPY --chown=0:0 ./passwd ./group /etc/
USER 1000

COPY --from=base --chown=1000:1000 /future-tmp /tmp
COPY --from=base --chown=1000:1000 /future-tmp /vmstorage-data

COPY --from=base --chown=1000:1000 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
