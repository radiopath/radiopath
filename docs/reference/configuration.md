# Configuration

Reference of every setting. Radiopath is configured only through environment variables; for local development
`make dev` copies `.env.example` to `.env` (gitignored) and loads it, in Kubernetes the same variables are set on
the Deployment, secrets via `secretKeyRef`.

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | required | PostgreSQL URL, e.g. `postgres://user:pw@host:5432/radiopath` |
| `RADIOPATH_MODE` | `all` | What this process does. `web`: serves the app and the API, computes nothing and needs no datasets, S3 or `/tmp`; `worker`: computes the jobs with the datasets and serves only `/healthz` and `/readyz`; `all`: both in one process (development, single-node) |
| `RADIOPATH_DATASET_DIR` | | Directory holding one folder per dataset, e.g. `/data` with `/data/srtm1/N47E009.hgt` (read-only volume in Kubernetes). Required unless the S3 bucket is configured; not read in mode `web` |
| `RADIOPATH_DATASET_S3_ENDPOINT` | | S3-compatible endpoint, e.g. `https://s3.example.org`. Required with the bucket |
| `RADIOPATH_DATASET_S3_BUCKET` | | Bucket holding one key prefix per dataset. Takes precedence over the directory |
| `RADIOPATH_DATASET_S3_REGION` | `us-east-1` | SigV4 region; keep the default for MinIO and Ceph |
| `RADIOPATH_DATASET_S3_ACCESS_KEY` | | Access key. Empty: anonymous requests (public bucket) |
| `RADIOPATH_DATASET_S3_SECRET_KEY` | | Secret key, from a Kubernetes Secret |
| `RADIOPATH_DATASET_DEM` | `srtm1` | Folder or key prefix (without slash) of the `.hgt` terrain tiles |
| `RADIOPATH_DATASET_CANOPY` | `canopy1` | Folder or key prefix (without slash) of the `.chm` canopy height tiles. Set it empty, or leave the folder out, for no vegetation model |
| `RADIOPATH_DATASET_HF` | `hf1` | Folder or key prefix (without slash) of the ITU-R P.533/P.372 data of the HF model (`ionosNN.bin`, `COEFFNNW.txt`, `P1239-3 Decile Factors.txt`), created by `scripts/fetch-hf-data.sh`. Set it empty, or leave the folder out, for no HF model: the HF operations answer 503. With the S3 bucket the files are downloaded into a temporary directory at startup (about 131 MB) and checked against their SHA-256. An incomplete folder, files missing from the bucket, a file that does not match or a failed download turn the HF model off and are logged; the rest of the app runs |
| `RADIOPATH_DEM_CACHE_TILES` | `8` | Tiles kept in memory; a 1" tile is 26 MB (also the size of the canopy cache, 13 MB per tile) |
| `RADIOPATH_LISTEN` | `:8080` | Listen address |
| `RADIOPATH_SUNSPOT_URL` | `https://services.swpc.noaa.gov/json/solar-cycle` | Directory of the NOAA SWPC solar cycle files (`observed-solar-cycle-indices.json`, `predicted-solar-cycle.json`) the server downloads every 12 hours (15 minutes after a failure) to fill in the sunspot number of the HF page; with `RADIOPATH_REDIS_URL` the files are shared through Valkey, so all replicas together download them once per 12 hours. Empty: no download, `GET /api/hf/sunspots` answers 404 and the page keeps its default |
| `RADIOPATH_METRICS_LISTEN` | `:9090` | Listen address of the Prometheus endpoint (`/metrics`). A second listener on purpose, so it is never reachable through the ingress. Empty: no metrics |
| `RADIOPATH_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error`. Logs are always JSON lines on stderr |
| `RADIOPATH_WORKERS` | `1` | Coverage workers in this process (modes `worker` and `all`); next to them one short-job worker per CPU computes link analyses and HF. `0` computes nothing at all |
| `RADIOPATH_TRUST_PROXY` | `false` | Take the client IP for rate limiting from the last `X-Forwarded-For` hop. Set `true` behind your ingress only |
| `RADIOPATH_REGISTRATION` | `true` | Self-service account creation at `/register`; `false` hides it and only the CLI and the admin panel create users. Needs `RADIOPATH_SMTP_HOST`, new accounts confirm their address by mail |
| `RADIOPATH_SMTP_HOST` | empty | Submission server for confirmation and reset mails (STARTTLS required, PLAIN auth when a user is set). Empty: no registration and no "Forgot password" |
| `RADIOPATH_SMTP_PORT` | `587` | Port of that server |
| `RADIOPATH_SMTP_STARTTLS` | `true` | `false` sends in the clear: only for a relay on localhost or the e2e mail catcher; Go refuses PLAIN auth without TLS anyway, except to localhost |
| `RADIOPATH_SMTP_USER` | empty | Login for the submission server |
| `RADIOPATH_SMTP_PASSWORD` | empty | Its password, from a Kubernetes Secret |
| `RADIOPATH_MAIL_FROM` | | Sender address, required with the host |
| `RADIOPATH_BASE_URL` | | Public address of the app, e.g. `https://radiopath.example.org`; the links in mails and the share links start with it. Required with the SMTP host; without it the share panel shows a path instead of a full URL |
| `RADIOPATH_ADMIN_TOKEN` | empty | Token for the admin panel at `/admin`, at least 32 characters. Empty: no admin panel |
| `RADIOPATH_REDIS_URL` | empty | Redis URL for the map tile cache, e.g. `redis://redis:6379/0`. Empty: tiles are proxied but not cached |
| `RADIOPATH_TILE_URL` | `https://tile.openstreetmap.org/{z}/{x}/{y}.png` | Upstream tile template (`{z}`, `{x}`, `{y}`, optional `{s}`) |
| `RADIOPATH_TILE_TTL` | `720h` | Cache lifetime of a tile, in Redis and in the browser |
| `RADIOPATH_TILE_MISSES_PER_IP` | `300` | Tiles one client IP may fetch upstream per minute; `0` disables the limit |

## Replaced variables

Releases before the dataset root had separate variables for terrain and canopy. The server refuses to start while
one of them is set and names its replacement:

| Old | New |
|---|---|
| `RADIOPATH_DEM_DIR=/data/dem` | `RADIOPATH_DATASET_DIR=/data` and `RADIOPATH_DATASET_DEM=dem` (or rename the folder to `srtm1`) |
| `RADIOPATH_DEM_S3_ENDPOINT`, `_BUCKET`, `_REGION`, `_ACCESS_KEY`, `_SECRET_KEY` | `RADIOPATH_DATASET_S3_*` with the same suffix |
| `RADIOPATH_DEM_S3_PREFIX=srtm1/` | `RADIOPATH_DATASET_DEM=srtm1` |
| `RADIOPATH_CANOPY_DIR`, `RADIOPATH_CANOPY_S3_PREFIX` | `RADIOPATH_DATASET_CANOPY`, a folder next to the terrain |
| `RADIOPATH_HF_DIR` | `RADIOPATH_DATASET_HF`, a folder next to the terrain (default `hf1`) |

Migrations are embedded and applied at startup.
