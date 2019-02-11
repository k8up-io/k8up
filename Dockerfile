FROM golang:1.11-alpine

RUN apk add --no-cache tzdata

WORKDIR /go/src/github.com/vshn/k8up
COPY . .
RUN go install -v ./...

ENTRYPOINT [ "operator" ]
