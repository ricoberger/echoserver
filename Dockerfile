FROM golang:1.15.6-alpine3.12 as build
WORKDIR /echoserver
COPY go.mod  ./
RUN go mod download
COPY . .
RUN go build

FROM alpine:3.12.0
RUN apk update && apk add --no-cache ca-certificates
RUN mkdir /echoserver
COPY --from=build /echoserver/echoserver /echoserver
WORKDIR /echoserver
USER nobody
ENTRYPOINT  [ "/echoserver/echoserver" ]
