# echoserver

Simple `echoserver`, which dumps HTTP requests.

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
- `/metrics`: Returns the captured Prometheus metrics.

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
helm upgrade --install echoserver oci://ghcr.io/ricoberger/charts/echoserver --version 1.0.0
```

## Examples

```sh
curl -vvv "http://localhost:8080/"
curl -vvv "http://localhost:8080/panic"
curl -vvv "http://localhost:8080/status"
curl -vvv "http://localhost:8080/status?status=400"
curl -vvv "http://localhost:8080/timeout?timeout=10s"
curl -vvv "http://localhost:8080/headersize?size=100"
curl -vvv -X POST -d '{"method": "POST", "url": "http://localhost:8080/", "body": "test", "headers": {"x-test": "test"}}' http://localhost:8080/request
```
