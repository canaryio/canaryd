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
* `REDIS_PUBLISH` - set to `1` to publish measurements via Redis; `PSUBSCRIBE` to `measurements:*` to get measurements for all check IDs
* `RETENTION` - how long to store measurements, in seconds; defaults to 60

`canaryd` allows internal performance metrics to be captured.  To send them to
Librato, set the following environment variables:

* `LIBRATO_EMAIL` - email address of your librato account
* `LIBRATO_TOKEN` - token for your Librato account
* `LIBRATO_SOURCE` - defaults to the server's hostname

To send them to InfluxDB:

* `INFLUXDB_HOST` - the InfluxDB hostname
* `INFLUXDB_DATABASE` - database to store the metrics in
* `INFLUXDB_USER`, `INFLUXDB_PASSWORD` - authentication credentials

You can also dump them to `stderr`:

* `LOGSTDERR` - set to `1` to enable

### Usage Example

```sh
# connect to a single local sensor
$ godep go run canaryd.go
2014/05/24 14:00:41 fn=httpServer listening=true port=5000
2014/05/24 14:00:41 fn=udpServer listening=true port=5000
```

###Websocket
Measurements can be received in (near) realtime via websocket. You can create a websocket connection at `<host>:<PORT>/ws/checks/{check_id}/measurements`. This connection will stream measurements back for the check ID you've specified.
