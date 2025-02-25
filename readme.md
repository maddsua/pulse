# Into

Pulse is similar to cloudprober, but minus the cloud part. It's a standalone service that you can just run everywhere with minimal config and still get those uptime and latency metrics.

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

Basic postgres/timescale query:
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
