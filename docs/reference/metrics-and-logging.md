# Metrics and logging

## Logging

`log/slog` as JSON on stderr, nothing else. Radiopath logs events, not requests:
logins and failed logins, admin actions, coverage jobs, and errors that produce
a 5xx. There is deliberately no access log — a request that succeeds leaves no
trace, and neither does a 404.

**If you want per-request logs, the reverse proxy in front of Radiopath has to
produce them.** That is the only place they exist. The application will not grow
an access log: it would duplicate what the proxy already writes, and the tile
endpoint alone would bury every other line.

Note that `kubectl logs deploy/radiopath` follows a single pod out of several.
Use the label and prefix the pod name:

```sh
kubectl -n radiopath-production logs -l app.kubernetes.io/component=web -f --prefix
```

## Metrics

Prometheus on a **second listener** (`RADIOPATH_METRICS_LISTEN`, default
`:9090`), never on the public port: `/metrics` is in no Service and no Ingress,
so there is nothing to authenticate. Locally: `curl localhost:9090/metrics`.

Next to the `go_*` and `process_*` runtime metrics:

- `radiopath_http_requests_total{route,method,code}` and
  `radiopath_http_request_duration_seconds{route}`. `route` is always a
  registered ServeMux pattern (`/coverages/{id}`), never the path — that one
  carries ids, tile coordinates and share tokens. An unmatched
  request is `other`. `/coverages/{id}/events` is an SSE stream of minutes and
  always lands in the `+Inf` bucket.
- `radiopath_jobs_total{kind,result}` (done, failed, requeued) and
  `radiopath_job_duration_seconds{kind}`. The histogram covers successful
  jobs; a computation that hits its timeout is `result="failed"`.
- `radiopath_link_analyses_total{result}`, `radiopath_link_analysis_duration_seconds`.
- `radiopath_dem_tile_cache_total{kind,result}` — raise
  `RADIOPATH_DEM_CACHE_TILES` when the miss share is high. A tile that does not
  exist (sea) counts as a miss.
- `radiopath_s3_requests_total{result}`, `radiopath_map_tiles_total{result}`,
  `radiopath_logins_total{result}`.
- Gauges read from the database on every scrape: `radiopath_users`,
  `radiopath_sites`, `radiopath_links`, `radiopath_sessions_active` (sessions
  that have not expired, so in practice those used in the last seven days) and
  `radiopath_coverages{job_state}` and `radiopath_jobs{kind,state}`, the
  queued ones being the queue depth. A database that is down leaves them out
  and logs a warning; the scrape still succeeds and everything else is still
  there.

**Query the gauges with `max`, not `sum`.** Every replica queries the same
database and reports the same numbers, so `sum` multiplies them by the replica
count. The counters and histograms are per pod and are summed as usual.

```promql
max by (kind) (radiopath_jobs{state="queued"})           # jobs waiting
sum(rate(radiopath_http_requests_total{code=~"5.."}[5m]))
histogram_quantile(0.9, sum by (le) (rate(radiopath_job_duration_seconds_bucket{kind="coverage"}[1h])))
```
