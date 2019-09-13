FROM golang:1.11-alpine as build

RUN apk add --no-cache tzdata

WORKDIR /go/src/github.com/vshn/k8up
COPY . .
RUN go install -v ./...

# runtime image
FROM docker.io/alpine:3
WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

COPY --from=build /go/bin/operator /app/

USER 1001

ENTRYPOINT [ "./operator" ]
