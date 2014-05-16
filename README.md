canaryd
=======

`canaryd` is an aggregator for measurements taken by [`sensord`](https://github.com/canaryio/sensord).  It connects to each sensor and streams measurements into a Redis instance.  Those measurements are then made available via a simple HTTP API.

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
  -port="5000": port the HTTP server should bind to
  -redis_url="redis://localhost:6379": redis url
  -sensord_url=[]: List of sensors
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
$ godep go run canaryd.go --sensord_url http://admin:admin@localhost:5001/measurements

2014/05/14 21:15:42 fn=main listening=true port=5000
2014/05/14 21:15:42 fn=ingestor url=http://admin:admin@localhost:5001/measurements connect=success
2014/05/14 21:15:42 fn=recorder check_id=http-nbviewer.ipython.org measurement_id=c256ddaf-bbc5-402f-79fb-641c60f512a0
2014/05/14 21:15:43 fn=recorder check_id=https-www.hostedgraphite.com measurement_id=c72f8c4a-66e6-4c59-6f9e-497953d7dd6c
2014/05/14 21:15:43 fn=recorder check_id=http-www.indeed.com measurement_id=be8619db-f72c-4bd7-46f5-2c56782f5a13
2014/05/14 21:15:43 fn=recorder check_id=http-github.com measurement_id=816d14d6-8191-4fed-5c6b-8837588adedb
2014/05/14 21:15:43 fn=recorder check_id=https-github.com measurement_id=dc45a49f-63c8-476a-7ae2-131757676485
2014/05/14 21:15:43 fn=recorder check_id=http-git.io measurement_id=60c67299-3cb0-42c2-430a-b17cc83a0e2b
2014/05/14 21:15:43 fn=recorder check_id=https-gist.github.com measurement_id=fc2c0b31-f63a-4e31-477c-ebfd5647a91a
^C
```
