FROM golang:1.10-alpine

RUN apk add --no-cache tzdata

WORKDIR /go/src/git.vshn.net/vshn/baas
COPY . .

RUN go install -v ./...

ENTRYPOINT [ "operator" ]
