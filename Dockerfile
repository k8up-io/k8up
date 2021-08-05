FROM docker.io/library/alpine:3.14 as runtime

ENTRYPOINT ["k8up"]

RUN apk add --update --no-cache \
    bash \
    ca-certificates \
    curl \
    fuse \
    openssh-client \
    tzdata

COPY --from=restic/restic:0.12.1 /usr/bin/restic /usr/bin/restic

COPY k8up /usr/bin/k8up
USER 65532:65532
