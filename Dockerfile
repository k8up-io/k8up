FROM docker.io/library/alpine:3.19 as restic

RUN apk add --update --no-cache \
    bash \
    ca-certificates \
    curl

COPY go.mod fetch_restic.sh ./
RUN ./fetch_restic.sh /usr/local/bin/restic \
    && /usr/local/bin/restic version

FROM docker.io/library/alpine:3.19 as k8up

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

COPY --from=restic /usr/local/bin/restic $RESTIC_BINARY
COPY k8up /usr/local/bin/

RUN chmod a+x /usr/local/bin/k8up
RUN $RESTIC_BINARY version

USER 65532
