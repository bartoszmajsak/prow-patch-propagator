FROM alpine:3.15.0

RUN apk --update --no-cache add ca-certificates \
    && adduser -D patch-propagator

USER patch-propagator

COPY  ./patch-propagatorr /usr/local/bin/patch-propagator

ENTRYPOINT ["/usr/local/bin/patch-propagator"]
