FROM golang:1.24.3 AS build
WORKDIR /echoserver
COPY go.mod  ./
RUN go mod download
COPY . .
RUN export CGO_ENABLED=0 && make build

FROM alpine:3.22.0
RUN apk update && apk add --no-cache ca-certificates
RUN mkdir /echoserver
COPY --from=build /echoserver/bin/echoserver /echoserver
WORKDIR /echoserver
USER nobody
ENTRYPOINT  [ "/echoserver/echoserver" ]
