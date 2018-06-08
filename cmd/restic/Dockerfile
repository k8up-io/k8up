FROM golang:1.10

ENV RESTIC_VERSION=0.9.1

RUN apt-get update && apt-get install wget bzip2 -y && cd / && \
    wget https://github.com/restic/restic/releases/download/v${RESTIC_VERSION}/restic_${RESTIC_VERSION}_linux_amd64.bz2 && \
    bzip2 -d restic_${RESTIC_VERSION}_linux_amd64.bz2 && \
    mkdir -p /usr/local/bin/ && \
    mv /restic_${RESTIC_VERSION}_linux_amd64 /usr/local/bin/restic && \
    chmod +x /usr/local/bin/restic && \
    mkdir /build

COPY main.go /build
RUN cd /build && go build -o restic-wrapper && mv restic-wrapper /usr/local/bin/

ENTRYPOINT [ "/usr/local/bin/restic-wrapper" ]
