# echoserver

Simple `echoserver`, which dumps HTTP requests.

- `/`: Dump the HTTP request.
- `/health`: Returns a 200 status code.
- `/panic`: Panics within the http handler and returns a status code 500.
- `/status`: Returns a random status code, when the `status` parameter is empty or `random`. Return the status code specified in the `status` parameter, e.g. `?status=200`.
- `/timeout`: Wait the given amount of time (`?timeout=1m`) before returning a 200 status code.
- `/headersize`: Returns a 200 status code with a header `X-Header-Size` of the size defined via `?size=1024`.
- `/metrics`: Returns the captured Prometheus metrics.

## Build

The `echoserver` can be built and run with the following commands:

```sh
make build
./bin/echoserver
```

When you are using Docker, you can use the following commands:

```sh
docker build -f ./cmd/echoserver/Dockerfile -t ghcr.io/ricoberger/echoserver:main .
docker run -it --rm --name echotest -p 8080:8080 ghcr.io/ricoberger/echoserver:main
```

## Deploy

To deploy the `echoserver` at Kubernetes run the following commands:

```sh
kubectl apply -n test -f https://raw.githubusercontent.com/ricoberger/echoserver/main/deploy/ns.yaml
kubectl apply -n test -f https://raw.githubusercontent.com/ricoberger/echoserver/main/deploy/deploy.yaml
kubectl apply -n test -f https://raw.githubusercontent.com/ricoberger/echoserver/main/deploy/svc.yaml
kubectl apply -n test -f https://raw.githubusercontent.com/ricoberger/echoserver/main/deploy/vs.yaml
```

## Exmples

```sh
curl -vvv "http://localhost:8080/"
curl -vvv "http://localhost:8080/panic"
curl -vvv "http://localhost:8080/status"
curl -vvv "http://localhost:8080/status?status=400"
curl -vvv "http://localhost:8080/timeout?timeout=10s"
curl -vvv "http://localhost:8080/headersize?size=100"
```
