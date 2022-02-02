FROM alpine:3.15

RUN apk --update --no-cache add ca-certificates && adduser -D prow-patcher

USER prow-patcher

COPY  ./prow-patcher /usr/local/bin/prow-patcher

ENTRYPOINT ["/usr/local/bin/prow-patcher"]
