FROM docker.io/library/alpine:3.14 as k8up

ENTRYPOINT ["k8up"]

RUN mkdir /.cache && chmod -R g=u /.cache

RUN apk add --update --no-cache \
    bash \
    ca-certificates \
    curl \
    fuse \
    openssh-client \
    tzdata

ENV RESTIC_BINARY=/usr/local/bin/restic

COPY wrestic/wrestic.sh /app/wrestic
COPY --from=restic/restic:0.12.1 /usr/bin/restic $RESTIC_BINARY
COPY k8up /usr/local/bin/

RUN chmod a+x /usr/local/bin/k8up /usr/local/bin/restic /app/wrestic

USER 65532

## wrestic compatibility shim
FROM k8up as wrestic

WORKDIR /app
ENTRYPOINT ["./wrestic"]

## wrestic root compatibility shim
FROM wrestic as wrestic-root

USER 0

## Default build shall not contain the wrestic compatibility stuff
FROM k8up as default
