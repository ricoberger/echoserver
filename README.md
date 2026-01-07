# echoserver

The `echoserver` is a HTTP / gRPC server written in Go, which was mainly written
to dump HTTP requests. Nowdays it can also be used to test / showcase the
instrumentation of a Go application with metrics, logs, traces and profiles. It
can also be used to test the timeout / header size configuration of a reverse
proxy.

## Building and Running

To build and run the `echoserver` the following commands can be used:

```sh
make build
./bin/echoserver
```

Via Docker the following commands can be used to build the image and run the
`echoserver`:

```sh
docker build -f ./cmd/echoserver/Dockerfile -t ghcr.io/ricoberger/echoserver:latest .
docker run -it --rm --name echoserver -p 8080:8080 ghcr.io/ricoberger/echoserver:latest
```

The `echoserver` can also be deployed on Kubernetes via Helm:

```sh
helm upgrade --install echoserver oci://ghcr.io/ricoberger/charts/echoserver --version <VERSION>
```

## Configuration

```plantext
Usage: echoserver [flags]

Flags:
  -h, --help                           Show context-sensitive help.
      --http-server.address=":8080"    The address where the server should listen on ($HTTP_SERVER_ADDRESS).
```

```sh
# Configure an OTLP/gRPC or OTLP/HTTP endpoint for traces, metrics, and logs.
# To configure different endpoints for traces, metrics, and logs, use the
# OTEL_EXPORTER_OTLP_TRACES_ENDPOINT, OTEL_EXPORTER_OTLP_METRICS_ENDPOINT,
# and OTEL_EXPORTER_OTLP_LOGS_ENDPOINT environment variables.
export OTEL_EXPORTER_OTLP_ENDPOINT="localhost:4317"
# Set the logs exporter which should be used. Valid values are "console",
# "otlp" and "none". The default value is "none".
export OTEL_LOGS_EXPORTER="console"
# Set the metrics exporter which should be used. Valid values are "console",
# "otlp", "prometheus" and "none". The default value is "none".
export OTEL_METRICS_EXPORTER="console"
# Set the traces exporter which should be used. Valid values are "console",
# "otlp" and "none". The default value is "none".
export OTEL_TRACES_EXPORTER="console"
# Key-value pairs to be used as resource attributes. This can be used to
# overwrite the default service name / version and to set additional attributes,
# like the Kubernetes Pod name, etc.
export OTEL_RESOURCE_ATTRIBUTES=service.name=my-service,service.version=1.0.0
# Enable resource detectors. Valid values are "container", "host", "os",
# "process", and "sdk".
export OTEL_RESOURCE_DETECTORS="container,host,os,process,sdk"

# When "OTEL_LOGS_EXPORTER" is set to "console", the following environment
# variables can be used to configure the logging output:
# - LOG_FORMAT: Set the output format of the logs. Must be "console" or "json".
# - LOG_LEVEL: Set the log level. Must be "DEBUG", "INFO", "WARN" or "ERROR".
# - LOG_RESOURCE_ATTRIBUTES: Define if the resource attributes should be logged.
#   Must be "true" or "false".
export LOG_FORMAT="console"
export LOG_LEVEL="INFO"
export LOG_RESOURCE_ATTRIBUTES="true"
```

## Usage

### HTTP Endpoints

- `/`: Dump the HTTP request.
- `/health`: Returns a 200 status code.
- `/panic`: Panics within the http handler and returns a status code 500.
- `/status`: Returns a random status code, when the `status` parameter is empty
  or `random`. Return the status code specified in the `status` parameter, e.g.
  `?status=200`.
- `/timeout`: Wait the given amount of time (`?timeout=1m`) before returning a
  200 status code.
- `/headersize`: Returns a 200 status code with a header `X-Header-Size` of the
  size defined via `?size=1024`.
- `/request`: Returns the response of the requested server. The request body
  should have the following structure:
  `{"method": "POST", "url": "http://localhost:8080/", "body": "test", "headers": {"x-test": "test"}}`
- `/fibonacci`: Returns the Fibonacci number for the given `n` parameter, e.g.
  `?n=100`. The intention behind this endpoint is to simulate a CPU-intensive
  task.
- `/websocket`: Can be used to test WebSocket connections. It returns the
  message sent over the WebSocket connection.
- `/metrics`: Returns the captured Prometheus metrics.

### gRPC Endpoints

- `Echoserver.Echo`: Echoes the message sent in the request.
- `Echoserver.Status`: Returns a gRPC error with the status specified in the
  `status` field of the request or a random gRPC error when the value of the
  `status` field is `random`.
- `Echoserver.Request`: Forwards the request to the specified gRPC endpoint and
  returns the response. The request message should have the following structure:
  `{"uri": "localhost:8081", "method": "Echoserver.Echo", "message": "{ \"message\": \"Hello\" }"}`

### Examples

```sh
curl -vvv "http://localhost:8080/"
curl -vvv "http://localhost:8080/panic"
curl -vvv "http://localhost:8080/status"
curl -vvv "http://localhost:8080/status?status=400"
curl -vvv "http://localhost:8080/timeout?timeout=10s&flush=2s"
curl -vvv "http://localhost:8080/headersize?size=100"
curl -vvv -X POST -d '{"method": "POST", "url": "http://localhost:8080/", "body": "test", "headers": {"x-test": "test"}}' http://localhost:8080/request
curl -vvv "http://localhost:8080/fibonacci?n=100"
```

```sh
grpcurl -format-error -plaintext -d '{ "message": "Hello" }' 'localhost:8081' Echoserver.Echo
grpcurl -format-error -plaintext -d '{ "status": "random" }' 'localhost:8081' Echoserver.Status
grpcurl -format-error -plaintext -d '{ "uri": "localhost:8081", "method": "Echoserver.Status", "message": "{ \"status\": \"random\" }" }' 'localhost:8081' Echoserver.Request
grpcurl -format-error -plaintext -d '{ "uri": "localhost:8081", "method": "Echoserver.Echo", "message": "{ \"message\": \"Hello\" }" }' 'localhost:8081' Echoserver.Request

# List endpoints, list methods for a specific endpoint and describe a method
grpcurl -plaintext 'localhost:8081' list
grpcurl -plaintext 'localhost:8081' list Echoserver
grpcurl -plaintext 'localhost:8081' describe Echoserver.Echo
```
