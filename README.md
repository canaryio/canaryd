canaryd
=======

`canaryd` is an aggregator for measurements taken by [`sensord`](https://github.com/canaryio/sensord).  It ingests metrics via UDP and stores a small window of them in a Redis instance. Those measurements are then made available via a simple HTTP API.

### Build

```sh
$ go get github.com/canaryio/canaryd
$ cd $GOPATH/src/github.com/canaryio/canaryd
$ godep get
$ godep go build
```

### Configuration

`canaryd` is configured via flags.

```sh
$ ./canaryd -h
Usage of ./canaryd:
  -port="5000": port the HTTP and UDP servers should bind to
  -redis_url="redis://localhost:6379": redis url
```

`canaryd` allows metrics to be recorded to Librato.  You can configure with the following environment variables:

```
export LIBRATO_EMAIL=me@mydomain.com
export LIBRATO_TOKEN=asdf
export LIBRATO_SOURCE=my_hostname
```

### Usage Example

```sh
# connect to a single local sensor
$ godep go run canaryd.go
2014/05/24 14:00:41 fn=httpServer listening=true port=5000
2014/05/24 14:00:41 fn=udpServer listening=true port=5000
```
