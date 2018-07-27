FROM golang:1.10

WORKDIR /go/src/git.vshn.net/vshn/baas
COPY . .

RUN go install -v ./...

ENTRYPOINT [ "operator" ]
