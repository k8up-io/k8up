FROM docker.io/library/alpine:3.16 as restic

RUN apk add --update --no-cache \
    bash \
    ca-certificates \
    curl

COPY go.mod fetch_restic.sh ./
RUN ./fetch_restic.sh /usr/local/bin/restic \
 && /usr/local/bin/restic version

FROM registry.devops.rivtower.com/cita-cloud/cloud-op:latest as cloudop

#FROM docker.io/library/alpine:3.16 as k8up
FROM debian:bullseye-slim

ENTRYPOINT ["k8up"]

RUN mkdir /.cache && chmod -R g=u /.cache

RUN apt-get update && apt-get install -y \
    ca-certificates \
    curl \
    fuse \
    openssh-client \
 && rm -rf /var/lib/apt/lists/*

ENV RESTIC_BINARY=/usr/local/bin/restic

COPY --from=cloudop /usr/bin/cloud-op /usr/local/bin
COPY --from=restic /usr/local/bin/restic $RESTIC_BINARY
COPY k8up /usr/local/bin/

RUN chmod a+x /usr/local/bin/k8up
RUN $RESTIC_BINARY version
RUN mkdir -p /state_data

#USER 65532
