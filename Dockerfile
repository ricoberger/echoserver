FROM golang:1.23.3 as build
WORKDIR /echoserver
COPY go.mod  ./
RUN go mod download
COPY . .
RUN export CGO_ENABLED=0 && go build

FROM alpine:3.20.3
RUN apk update && apk add --no-cache ca-certificates
RUN mkdir /echoserver
COPY --from=build /echoserver/echoserver /echoserver
WORKDIR /echoserver
USER nobody
ENTRYPOINT  [ "/echoserver/echoserver" ]
