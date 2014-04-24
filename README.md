canaryd
=======

`canaryd` is a simple telemetry service charged with both receiving measurements made against various URLs and serving them up in a fast and unobstrusive manner via a simple HTTP API.


### Concepts

`canaryd` is intended to be a simple, unassuming http server.  It accepts a JSON array of measurement data via `POST /measurements` in the format of:

```js
[
  {
    "check": {
      "id": "947235e3-b9bc-451e-90fb-f5744298df5f",
      "url": "https://github.com"
    },
    "id": "64cd257b-d1e2-4832-4813-c51d6e197a28",
    "location": "do-ny2",
    "t": 1398360795,
    "exit_status": 0,
    "http_status": 200,
    "local_ip": "107.170.68.66",
    "primary_ip": "192.30.252.131",
    "namelookup_time": 0.02074,
    "connect_time": 0.027214,
    "starttransfer_time": 0.061265,
    "total_time": 0.067903
  },
  ...
]
```

Measurements are stored within a Redis database, with a sorted set being allocated for each check id.

### Configuration

`canaryd` is configured via environment variables.

#### PORT

What HTTP port `canaryd` should listen on.  This defaults to port 5000.

#### REDIS_URL

`canaryd` stores incoming measurements in redis sorted sets.  It uses one set per URL being monitored, and evicts measurements older than 60 seconds.
