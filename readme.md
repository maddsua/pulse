# Intro

Pulse is similar to cloudprober, but minus the cloud part. It's a standalone service that you can just run everywhere with minimal config and still get those uptime and latency metrics.

## Connecting to a database

By default, pulse uses an embedded storage backed up by sqlite3. That's cool for testing/debugging,
but for production-ish use you'd want to connect it to a proper database service such as TimescaleDB or PostgreSQL.

Use the `DATABASE_URL` environment variable to provide the, well, database url to use.

Make sure the database user has the permission to create tables in the schema 'public'.

## Adding probes

Adding a probe is as simple as adding a config file entry:
```yml
probes:
  cloudlfare-dns:
    http:
      method: GET
      url: https://1.1.1.1/
      interval: 60
      timeout: 10
```

## Deploying

Using a dockerfile:
```Dockerfile
from ghcr.io/maddsua/pulse:latest
copy ./pulse.config.yml /pulse.config.yml
cmd ["-config=/pulse.config.yml"]
```

## Querying the metrics

Get metrics for the last 6 hours using just plain SQL:
```sql
select
  time,
  latency,
  label
from series
where time >= now() - '6h'::interval
```

Basic postgres/timescale query with grafana postgres data source:
```sql
select
  time,
  latency,
  label
from series
where $__timeFilter(time)
```

With interval grouping (should sample data, removing points that are too close):
```sql
select
  $__timeGroupAlias(time, $__interval),
  avg(latency),
  label
from series
where $__timeFilter(time)
group by
  time,
  latency,
  label
order by time
```

There's also an exporter api, that can be enabled with the following config lines:
```yml
exporters:
  series: true
```

This will enable a local http server with a path `/exporters/series` that can be used to query metrics in json format.

See the [openapi.yml](./openapi.yml) for more details.

## Full config reference

`pulse.config.yml`:
```yml
# defines the list of probe tasks
probes:
  # each probe has to have a unique tag/key/name, whatever you want to call it
  # this key is what you will see in the metrics
  cloudlfare-dns:
    # probe config specifically for the http variant
    http:
      # the http method to use (defaults to HEAD if not set)
      method: GET
      # http request url, required and that's non-negotiable
      url: https://10.10.10.10:443/
      # optional request headers to send;
      # this is particularly useful to status check load balancers and proxies
      headers:
        # here we override the '10.10.10.10:443' request host to a site name that the proxy must serve us
        host: example.com
      # probe interval in seconds, so one minute in this case
      interval: 60
      # probe operation timeout in seconds, if the target doesn't respond in that time - it will be considered to be down
      timeout: 10

# data export API options;
# if no exporters are enabled, pulse won't even start the http server, as it's not needed for anything
exporters:
  # this one enables the series endpoint
  series: true
```
