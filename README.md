canaryd
=======

`canaryd` is an aggregator for measurements taken by [`sensord`](https://github.com/canaryio/sensord).  It ingests metrics via UDP and stores a small window of them in a Redis instance. Those measurements are then made available via a simple HTTP API and Websocket.

### Build

```sh
$ go get github.com/canaryio/canaryd
$ cd $GOPATH/src/github.com/canaryio/canaryd
$ godep get
$ godep go build
```

### Configuration

`canaryd` is configured via the environment. The following values are allowed:

* `PORT` - port the HTTP and UDP servers should bind to, defaulting to '5000'
* `REDIS_URL` - URL for the redis backend, defaults to redis://localhost:6379

`canaryd` allows metrics to be recorded to Librato.  You can configure with the following environment variables:

* `LIBRATO_EMAIL` - email address of your librato account
* `LIBRATO_TOKEN` - token for your Librato account

### Usage Example

```sh
# connect to a single local sensor
$ godep go run canaryd.go
2014/05/24 14:00:41 fn=httpServer listening=true port=5000
2014/05/24 14:00:41 fn=udpServer listening=true port=5000
```

###Websocket
Measurements can be received in (near) realtime via websocket. You can create a websocket connection at `<host>:<PORT>/ws/checks/{check_id}/measurements`. This connection will stream measurements back for the check ID you've specified.
