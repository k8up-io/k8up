FROM golang:1.13-alpine as build

ENV CGO_ENABLED=0

WORKDIR /go/src/github.com/vshn/k8up
COPY . .
RUN go test -v ./...
RUN go install -v ./...

# runtime image
FROM docker.io/alpine:3
WORKDIR /app

RUN apk --no-cache add tzdata

COPY --from=build /go/bin/operator /app/

USER 1001

ENTRYPOINT [ "./operator" ]
