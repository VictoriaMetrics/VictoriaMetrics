FROM alpine:3
RUN apk add --no-cache rsyslog rsyslog-tls tzdata

COPY rsyslog.conf /etc/rsyslog.conf

VOLUME /var/run/rsyslog/dev
EXPOSE 514 10514

CMD ["/usr/sbin/rsyslogd", "-n"]
