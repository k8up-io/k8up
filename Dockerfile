## BUILDER

ARG GOVERSION=1.15
FROM golang:${GOVERSION} as builder

WORKDIR /app

COPY go.* ./
RUN go mod download

COPY ./. .
RUN make build

## RUNTIME

FROM docker.io/library/alpine:3.12 as runtime

ENTRYPOINT ["k8up"]

RUN apk add --no-cache \
      bash \
      curl \
      tzdata

COPY LICENSE ./
COPY --from=builder /app/k8up /usr/bin/

USER 1001:0
