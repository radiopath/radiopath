# Operations

What the binary offers an orchestrator, and how the map tile proxy behaves.

## Requirements

- PostgreSQL with PostGIS
- Valkey or Redis for the map tile cache (optional, strongly recommended). Only `GET`,
  `SET` and `PING` are used, so any server speaking the Redis protocol will do
- Terrain `.hgt` tiles in the SRTM format (1 or 3 arc-second, `N47E009.hgt`),
  either in a directory or in an S3 bucket. The production bucket holds
  Copernicus GLO-30 (30 m, global) converted with `scripts/sync-dem-s3.sh`;
  for a quick local start, SRTM tiles without login work just as well:
  `https://step.esa.int/auxdata/dem/SRTMGL1/N47E009.SRTMGL1.hgt.zip`
- Optional: canopy height `.chm` tiles on the same grid, see [Build the tree canopy layer](../how-to/build-the-canopy-layer.md)

## Kubernetes

The Helm chart, its values and the ingress/certificate setup are documented in
[radiopath/helmchart](https://github.com/radiopath/helmchart). What the app
itself offers an orchestrator:

- `GET /healthz` liveness, `GET /readyz` readiness (database ping, DEM source
  reachable, HF data files present when the HF data set is loaded, Redis ping
  when configured). A failing check answers 503 `not ready`;
  which part failed and why is only in the log (`not ready`, `part`, `err`).
- `GET /metrics` on port 9090, a separate listener that is in no Service and no
  Ingress, see [Metrics and logging](metrics-and-logging.md).
- Memory: about `RADIOPATH_DEM_CACHE_TILES` x 26 MB (x 39 MB with a canopy
  layer) plus 100 MB per pod.
- Shutdown on SIGTERM requeues running jobs and drains connections for up to 10 s.
- Migrations run at startup under a Postgres advisory lock, so any number of
  replicas may start at once.

## Map tiles

Browsers never load tiles from OpenStreetMap directly. Leaflet (bundled into
the app, BSD-2) requests `/tiles/{z}/{x}/{y}.png` from
Radiopath, which serves them from Redis or fetches them once from the upstream
server with its own User-Agent and stores them for `RADIOPATH_TILE_TTL`. Tiles
carry ETags, so a browser reload revalidates with 304 instead of downloading
them again; the app's hashed files under `/_app/immutable/` are cached for a
year. Size the
Redis `maxmemory` with an LRU policy; a few hundred MB is plenty. Only tiles that
miss the cache count against `RADIOPATH_TILE_MISSES_PER_IP`, so normal map use
never hits the limit and a crawler walking the tile pyramid gets 429s; raise it
if many users share one address behind NAT. If you deploy
this publicly, read the OpenStreetMap tile usage policy and consider your own
tile server or a commercial provider via `RADIOPATH_TILE_URL`.

## User accounts from the command line

Accounts can also be managed with the binary against the same database; those have no address and count as
confirmed:

```sh
radiopath useradd hb9hil      # asks for the password twice; non-interactive: echo 'secret' | radiopath useradd hb9hil
radiopath users
radiopath userdel hb9hil      # also deletes their sites, links, coverages and antennas
# Kubernetes: kubectl exec deploy/radiopath -- /radiopath useradd hb9hil <<< 'secret'
```
