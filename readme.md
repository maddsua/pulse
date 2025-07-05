# Intro

Pulse is similar to cloudprober, but minus the cloud part. It's a standalone service
that you can just run everywhere (minus the icmp part lol, that requires root)
with minimal config and still get those uptime and latency metrics.

## Probe types

Pulse has a few ways to check if your service is up and running

### HTTP

Umm, so it's an http probe, yeah. All it does is makes a request and waits for a 2xx response. Literally.

Configuration:
```yml
interval: 1m	# probe invocation interval in time.Duration format
timeout: 1m		# probe run timeout in time.Duration format
url: http://www.google.com/gen_204	# target url
method: HEAD	# set http request method (defaults to GET)
headers:		# optional headers that you may want to add to the request
  user-agent: my-custom-ua			# any headers here, really
proxy_url: socks://user:pass@host:port	# a proxy to use with the request
retries: 4		# number of retries if a request failed
```

The `proxy_url` can be used to enable a proxy, duh, in cases when you want to bypass firewalls or sumthng.


### ICMP

This was supposed to be the cool one, but due to the way the things are, you can't use it unless you run pulse on a VPS or your PC with root priviledges. Not great, but hey, you got the option.

Config:
```yml
interval: 1m		# probe invocation interval in time.Duration format
timeout: 1m			# probe run timeout in time.Duration format
host: example.com	# target host as an ip or a domain name
retries: 2			# number of retries if a request failed
```

## Writers

Unlike v1, pulse v2 has a completely modular storage model.

By default, a simple stdout output is used. It will log probe results directly to the console.

These are the supported backends:

### timescaledb/postgres

The same primary db as in v1, except for the migrations being completely changed. Now, instead of using full schema migration, pulse will create a new series table for each version. This hepls prevent broken read queries from third party tools, but also gets rid of a whole bunch of package dependencies.

Setting the `TIMESCALE_URL` environment variable will enable this storage backend.

Make sure the database user has the permission to create tables in the schema 'public'.

#### Querying metrics

Get metrics for the last 6 hours using just plain SQL:
```sql
select
  time,
  label,
  coalesce(latency, -1) as latency
from pulse_uptime_v2
where time >= now() - '6h'::interval
group by
  time,
  label,
  latency
order by time
```

Basic postgres/timescale query with grafana postgres data source:
```sql
select
  time,
  label,
  coalesce(latency, -1) as latency
from pulse_uptime_v2
where $__timeFilter(time)
group by
  time,
  label,
  latency
order by time
```

With interval grouping (should sample data, removing points that are too close):
```sql
select
  $__timeGroupAlias(time, $__interval),
  label,
  avg(latency) as latency
from pulse_uptime_v2
where $__timeFilter(time)
group by
  time,
  label,
  latency
order by time
```

### Prometheus PushGateway

Pulse v2 doesn't have exporters api that can be used by scrapers, instead it can be configured to send metrics to PushGateway, that can be scraped instead.

Set `PUSHGATEWAY_URL` env variable to enable this storage; format: `{http|https}://{host:?port}`.

Please note that this driver cannot store string values as metrics and they're converted to labels instead. Boolean values are converted to integers as well.

It's expected that you'll use something like Grafana to query the data, have fun 👍

### InfluxDB

Enabled by `INFLUXDB_URL` env variable, format: `{http|https}://:{token}@{host:?port}/{bucket}`

This one is a bit fucky. Since the basic auth (aka username:pass) doesn't seem to work whatsoever with the influx v2, we have to stick with the v1 API.

However, even the v1 API doesn't want to accept the credentials for some reason, which means that we have to resort to using tokens. And in my totally not biased opinion it makes sence to still pass the token in the url in the password position, while leaving the username empty or setting it to something silly. Don't worry, golang can parse that, I tried. The bucket name is passed as the sole path segment, similar to `psql` URLs.

## Deploying

Using a dockerfile:
```Dockerfile
from ghcr.io/maddsua/pulse:latest
copy ./your-config.yml /pulse.yml
cmd ["-config=/pulse.yml"]
```
