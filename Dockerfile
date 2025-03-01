FROM golang:1.24.0 as build
WORKDIR /echoserver
COPY go.mod  ./
RUN go mod download
COPY . .
RUN export CGO_ENABLED=0 && go build

FROM alpine:3.21.3
RUN apk update && apk add --no-cache ca-certificates
RUN mkdir /echoserver
COPY --from=build /echoserver/echoserver /echoserver
WORKDIR /echoserver
USER nobody
ENTRYPOINT  [ "/echoserver/echoserver" ]
