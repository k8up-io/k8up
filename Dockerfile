FROM golang:1.11-alpine

RUN apk add --no-cache tzdata curl

WORKDIR /go/src/github.com/vshn/k8up
COPY . .
RUN go install -v ./...

ENTRYPOINT [ "operator" ]

HEALTHCHECK --interval=5m --timeout=3s \
  CMD curl -f http://localhost:8080/metrics || exit 1