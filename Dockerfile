FROM docker.io/library/alpine:3.12 as runtime

ENTRYPOINT ["k8up"]

RUN \
    apk add --no-cache curl bash tzdata

COPY k8up /usr/bin/
USER 65532:65532
