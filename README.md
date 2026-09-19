# Radiopath

> [!TIP]
> **Just want to plan a link? Use the hosted instance: [app.radiopath.org](https://app.radiopath.org)**
> — no installation, free to sign up. The rest of this README is about self-hosting.

Radio coverage and link planning for amateur radio and beyond. Web application
written in Go, inspired by Radio Mobile. Propagation is computed with the NTIA
Irregular Terrain Model (Longley-Rice), ported to Go from the public domain
reference implementation (see `internal/itm/NOTICE`).

Current scope: point-to-point link analysis (terrain profile, ITM path loss,
link budget, first Fresnel zone clearance) and area coverage (link margin raster
around a transmitter, computed along radials and stored as PNG).

## Requirements

- PostgreSQL with PostGIS
- Valkey or Redis for the map tile cache (optional, strongly recommended). Only `GET`,
  `SET` and `PING` are used, so any server speaking the Redis protocol will do
- Terrain `.hgt` tiles in the SRTM format (1 or 3 arc-second, `N47E009.hgt`),
  either in a directory or in an S3 bucket. The production bucket holds
  Copernicus GLO-30 (30 m, global) converted with `scripts/sync-dem-s3.sh`;
  for a quick local start, SRTM tiles without login work just as well:
  `https://step.esa.int/auxdata/dem/SRTMGL1/N47E009.SRTMGL1.hgt.zip`
- Optional: canopy height `.chm` tiles on the same grid, see "Tree canopy"

## Configuration

All configuration is via environment variables. For local development copy
`.env.example` to `.env` (gitignored); `make run` exports it. In Kubernetes set
the same variables on the Deployment, secrets via `secretKeyRef`.

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | required | PostgreSQL URL, e.g. `postgres://user:pw@host:5432/radiopath` |
| `RADIOPATH_DEM_DIR` | | Directory with `.hgt` tiles (read-only volume in Kubernetes). Required unless the S3 source is configured |
| `RADIOPATH_DEM_S3_ENDPOINT` | | S3-compatible endpoint, e.g. `https://s3.example.org`. Enables the S3 source together with the bucket |
| `RADIOPATH_DEM_S3_BUCKET` | | Bucket holding the `.hgt` tiles |
| `RADIOPATH_DEM_S3_PREFIX` | empty | Key prefix, e.g. `srtm1/` for keys like `srtm1/N47E009.hgt` |
| `RADIOPATH_DEM_S3_REGION` | `us-east-1` | SigV4 region; keep the default for MinIO and Ceph |
| `RADIOPATH_DEM_S3_ACCESS_KEY` | | Access key. Empty: anonymous requests (public bucket) |
| `RADIOPATH_DEM_S3_SECRET_KEY` | | Secret key, from a Kubernetes Secret |
| `RADIOPATH_DEM_CACHE_TILES` | `8` | Tiles kept in memory; a 1" tile is 26 MB (also the size of the canopy cache, 13 MB per tile) |
| `RADIOPATH_CANOPY_DIR` | | Directory with `.chm` canopy height tiles. Empty: no vegetation model |
| `RADIOPATH_CANOPY_S3_PREFIX` | | Key prefix of the `.chm` tiles in the DEM bucket, e.g. `canopy1/`. Takes precedence over the directory |
| `RADIOPATH_LISTEN` | `:8080` | Listen address |
| `RADIOPATH_METRICS_LISTEN` | `:9090` | Listen address of the Prometheus endpoint (`/metrics`). A second listener on purpose, so it is never reachable through the ingress. Empty: no metrics |
| `RADIOPATH_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error`. Logs are always JSON lines on stderr |
| `RADIOPATH_WORKERS` | `1` | Coverage workers in this process; `0` for web-only pods, `1` on compute pods |
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

Migrations are embedded and applied at startup.

## Development

```sh
make db-up                 # PostGIS, Valkey and Mailpit via docker compose
mkdir dem && cd dem && curl -sSLO https://step.esa.int/auxdata/dem/SRTMGL1/N47E009.SRTMGL1.hgt.zip && unzip *.zip
make run                   # creates .env from .env.example on first run, builds the CSS, then http://localhost:8080
make test
make css-watch             # rebuild the stylesheet while editing templates
```

`make run` and `make build` need Node for the stylesheet (see below), `make e2e`
additionally needs Docker; `go build`, `go test` and the Docker image do not.

Releases: note the changes under `## [Unreleased]` in `CHANGELOG.md`, move them
into a `## [X.Y.Z] - date` section and push the tag `vX.Y.Z`. The release
workflow runs the tests, takes that section as the release notes, attaches
Linux binaries for amd64 and arm64 and pushes the multi-arch image
`ghcr.io/radiopath/radiopath:X.Y.Z` (also tagged `latest`).

### End-to-end tests

```sh
make db-up                 # also starts Mailpit, the mail catcher (http://localhost:8025)
make e2e                   # builds the binary, starts it on :8081 against the database radiopath_e2e, runs Cypress
make e2e-open              # same, interactive runner
```

`e2e/e2e.sh` runs the server with one flat DEM tile (`tmp/e2e/dem`, generated),
registration on, Mailpit as SMTP server and a fixed admin token; `e2e/cypress.config.js`
wipes the database before each spec and creates users through the CLI. Cypress itself
runs from the pinned `cypress/included` image (override with `CYPRESS_IMAGE`), so no
Electron has to work on the host and `npm ci` stays small; the container joins the host
network and mounts the repo at its own path. The specs in
`e2e/specs` cover login, sites, antennas, links, coverage jobs, per-user
ownership, the account page, the admin panel and the mail flows (registration,
confirmation, password reset). Map tiles are stubbed in the browser, nothing reaches
the tile server. The GitHub Actions workflow (`.github/workflows/ci.yml`) runs
gofmt, vet and the unit tests, checks that the committed stylesheet and the Helm
chart are in order, and runs the same e2e script with PostGIS and Mailpit as
services.

## DEM from S3

With `RADIOPATH_DEM_S3_ENDPOINT` and `RADIOPATH_DEM_S3_BUCKET` set, tiles are
fetched on demand with `GET {endpoint}/{bucket}/{prefix}{band}/{name}.hgt`
(path style, AWS Signature V4 when keys are given) and kept in memory like
directory tiles. Keys are grouped by latitude band, `srtm1/N47/N47E009.hgt`,
so a bucket covering the world stays browsable; directories are flat.
Only GetObject is used, so a read-only policy on the bucket is enough.
`scripts/sync-dem-s3.sh` fills the bucket from Copernicus GLO-30 (public AWS
bucket `copernicus-dem-30m`, one ~40 MB COG per degree) using the S3 settings
in `.env`: each tile is warped onto the 1" grid with gdalwarp together with its
east, south and south-east neighbours, because the source has 3600 pixels per
degree and the 3601st row and column belong to the next tile. Needs curl and
gdalwarp. Resumable, skips sea tiles, `-f` overwrites existing tiles:

```sh
scripts/sync-dem-s3.sh 34 59 -11 44         # Europe: lat 34..59, lon -11..44, about 1200 tiles / 31 GB
scripts/sync-dem-s3.sh -j 4 -- -90 89 -180 179 # the world: about 26 000 tiles / 680 GB, 8 h with 4 workers
scripts/sync-dem-s3.sh -f N47E009 N47E008   # single tiles, replaced
```

The key prefix `srtm1/` is historical: the tiles are Copernicus data in the
SRTM file format.

Fetching a 26 MB tile takes a moment on first use; size `RADIOPATH_DEM_CACHE_TILES`
so the working area stays in memory. The readiness probe does not check S3.

## Tree canopy

ITM has no vegetation. With a canopy height layer configured, a link reads the
tree top height along the path and adds, per end, the loss of ITU-R P.833-10
§2.1 for a terminal in woodland, `A = A_m (1 - exp(-d γ / A_m))`, where `d` is
the length of the direct ray inside the canopy from the antenna until it rises
above the trees. `γ` and `A_m` are read from the recommendation's Fig. 2 and
eq. (2) for mixed forest (about 1.1 dB/m and 52 dB at 5.7 GHz, 0.2 dB/m and
24 dB at 950 MHz). P.833 defines this loss as excess over free space and
diffraction, so ITM keeps running on the bare terrain; the tree tops are drawn
in the profile and enter the Fresnel clearance figure. Woods in the middle of
a path are drawn but add no loss, and the manual extra path loss field stays
for what the layer does not know. Without a layer nothing changes.

Two caveats. GLO-30, like SRTM, is a surface model: over dense forest the
elevation already contains part of the canopy, so trees on top are an upper bound until a real
terrain model is used. And the layer is a satellite estimate; the measured
level on a built link remains the check.

Tiles are `.chm` files on the SRTM 1" grid: 3601 x 3601 nodes, one byte per
node with the canopy top height in metres, 0 for no trees, row 0 at the north
edge, `N47E009.chm`. A missing tile means no trees, so only tiles with forest
need to exist. Heights come from the ETH Global Canopy Height 2020 map (10 m,
CC BY 4.0; Lang, Jetz, Schindler, Wegner: *A high-resolution canopy height
model of the Earth*, Nature Ecology & Evolution 7, 2023). That model reports
heights on meadows and rock as well (11 to 19 m on the Säntis summit), so the
converter keeps them only where ESA WorldCover 2021 (10 m, CC BY 4.0, same
tiles and grid) has tree cover — majority class per 30 m cell, tallest tree
per cell. `scripts/sync-canopy-s3.sh` does both and needs curl, python3 and
GDAL — or a running podman machine, then it uses the `ghcr.io/osgeo/gdal`
image:

```sh
scripts/sync-canopy-s3.sh 34 59 -11 44   # same box as the DEM, about 16 GB
scripts/sync-canopy-s3.sh -j 6 -- -60 80 -180 179   # the world, about 170 GB; -j runs source tiles in parallel
scripts/sync-canopy-s3.sh N45E009        # one 3x3 degree source tile
```

## Login

All pages require a login; public are `/healthz`, `/readyz`, `/static/`,
`/login`, `/register`, the tile proxy `/tiles/` and the share links under `/s/`
(see below). Users
can register themselves unless `RADIOPATH_REGISTRATION=false`; names are 3 to
32 characters (letters, digits, `._-`) and unique ignoring case, passwords need
10 characters (at most 72). Registering asks for an email address and mails a
confirmation link (24 hours); the account cannot log in before it is opened
(and is deleted after 7 days if it never is),
and logging in with the right password meanwhile offers a button for a fresh
link (3 per account per hour). Under
the user name in the header, `/account` changes the address (again by
confirmation link, the old one stays until then) and the password (the old
one is required, every other session is logged out). `/forgot` mails a reset
link (one hour) and answers the same whether or not the address is known.
Links are random tokens stored hashed, like sessions, and used once.
Sites, links, coverages and antenna patterns belong to the user
who created them; nobody else sees them except through a share link the owner
switched on, and deleting a user deletes their data with them. One account can hold 200 sites and 50 coverages and have 2 coverage
jobs queued or running at a time (constants in `internal/store/store.go`), so
an open registration cannot fill the database or block the workers. Accounts
can also be managed with the binary against the same database; those have no
address and count as confirmed:

```sh
radiopath useradd hb9hil      # asks for the password twice; non-interactive: echo 'secret' | radiopath useradd hb9hil
radiopath users
radiopath userdel hb9hil      # also deletes their sites, links, coverages and antennas
# Kubernetes: kubectl exec deploy/radiopath -- /radiopath useradd hb9hil <<< 'secret'
```

Passwords are stored as bcrypt hashes. Sessions live in PostgreSQL (cookie holds
a random token, the table its SHA-256), so any replica can serve any session and
logout revokes it. A session expires seven days after it was last used and
thirty days after the login that created it, whichever comes first: the row's
expiry is pushed back out on use, but never past that cap, so an abandoned
login and a stolen cookie both die within a week
(`sessionIdle` and `sessionTTL` in `internal/web/auth.go`). Refreshing writes at
most once per session per hour, not once per request. The cookie is `HttpOnly`, `SameSite=Lax` and
`Secure` when the request came over TLS or with `X-Forwarded-Proto: https`; make
sure the ingress sets that header. CSRF protection uses Go's
`http.CrossOriginProtection` (Sec-Fetch-Site / Origin checks on every POST),
so forms carry no tokens. Every response carries `X-Content-Type-Options:
nosniff`, `X-Frame-Options: DENY` and `Referrer-Policy: same-origin`; TLS and
HSTS are the ingress's job.

Rate limits are counted in PostgreSQL, so they hold across replicas: 20 failed
logins per IP and 5 per user name in 15 minutes, 5 registration attempts per IP
per hour, 5 reset requests per IP and 3 per address per hour, 3 confirmation
mails per account per hour, 120 requests per shared object per hour; further
requests get HTTP 429. Behind an ingress set
`RADIOPATH_TRUST_PROXY=true` so the client IP is read from `X-Forwarded-For`
(last hop); without it every request would count against the ingress address.

## Share links

A link or a coverage can be published read-only: "Share" on its page opens a
dialog that hands out a URL like `https://radiopath.example.org/s/l/<token>` that
shows the same page — profile, map, tables and the PDF — to anyone, without an
account. A shared coverage comes with the map as the owner arranged it: the
raster opacity, the range circles and the other coverages merged into the map are
stored on the coverage and published with it, so a multi-site picture stays a multi-site picture. It is valid 30 days, "Extend by 30 days" pushes that out while keeping
the URL, "Revoke" kills it immediately. The visitor sees no navigation, no
account name and no other object of the owner; nothing on the page can change
or recompute anything. The token is what grants access, so treat the URL as the
secret: shared pages are sent `X-Robots-Tag: noindex` and `Cache-Control:
no-store`, but a URL pasted somewhere public is public.

## Admin panel

With `RADIOPATH_ADMIN_TOKEN` set, `/admin` lists the users with their address
and confirmation state and can create them (without an address, confirmed),
set passwords, revoke sessions and delete accounts (together with the user's
data). It is not an account: there
is no admin user, no role column, and therefore nothing to lock yourself out of
— deleting every user leaves the panel reachable. Without the variable the
routes are not registered at all and `/admin` behaves like any other unknown
path.

The token is checked once against a form, after which a cookie signed with that
token carries the session for eight hours: no server-side state, so every
replica accepts it. Rotating the token invalidates every admin cookie — in
Kubernetes that means editing the Secret **and** restarting the pods, since the
value is injected at pod start. Ten wrong attempts per client address in 15
minutes give HTTP 429, counted in the same table as the login limits. The cookie
is `HttpOnly`, `SameSite=Strict` and scoped to `/admin`, and admin responses are
never cached.

Two switches sit above the user list: **Revoke all sessions** logs everyone out
at once, and **Maintenance mode** blocks new logins and shows a notice on the
login page instead of the form (HTTP 503; registration is blocked with it).
Sessions that are already open are deliberately left alone — use both buttons
together to empty the app. The admin panel itself is unaffected, it does not
depend on a user session.

Password suggestions are generated in the browser with `crypto.getRandomValues`
(20 characters from a 64 character alphabet, always containing a lower case
letter, an upper case letter, a digit and a symbol), so no server response ever
carries a plaintext password. Without JavaScript the field stays empty and you
type your own. Passwords are only ever stored as bcrypt hashes and cannot be
read back.

## Antenna patterns

Antennas are optional. Without one, gain is the same in every direction; with
one, the azimuth pattern is subtracted from the gain in the direction that
matters — towards the other site for a link, per radial for a coverage. Import
is MSI Planet (`.msi`, `.pln`), the format antenna manufacturers ship: only the
horizontal block is read, values are attenuation in dB below the main lobe, and
a `GAIN` without a unit is taken as dBd. Files with 720 half-degree values are
resampled to one value per degree. Picking an antenna on a link or coverage
fills in its gain and locks the field; the stored gain is always the main lobe
gain, and the pattern only subtracts from it.

Without a file, an antenna can be described instead: name, gain, 3 dB beamwidth
and front-to-back ratio give the usual sector shape,
12 (angle / beamwidth)^2 dB capped at the front-to-back ratio; 360 degrees of
beamwidth is an omni. Enough for planning, not a replacement for the
manufacturer's pattern.

A link aims both antennas at the other site unless an azimuth is given; a
coverage always takes an explicit azimuth, 0 degrees is north. Elevation
patterns and mechanical downtilt are not modelled.

## User interface

Server-rendered `html/template`, no JS framework; Leaflet is the only script.
Styling is Tailwind CSS v4: the source is `internal/web/ui/app.css`, the built
stylesheet `internal/web/static/app.css` is committed and embedded, so nothing
but Go is needed to build or deploy the app. After changing a template or the
source CSS run `make css` (or `make css-watch`) and commit the result; the CI
job `lint-css` rebuilds it and fails if the committed file is stale.

A computed coverage page can show the other computed rasters together with its
own, with one opacity slider for all of them. Ticked rasters are merged by the
server into one image (`/coverages/{id}/composite.png?with=3,5`): where they
overlap, the best link margin wins, so a green cell is never hidden by a yellow
one from another coverage. The merged raster is coloured with the legend of
the coverage whose page it is on. For that the worker stores, next to the
coloured PNG, the margins themselves as an 8-bit grey PNG (`result_margin`,
whole dB); rasters computed before that column existed are merged at the
accuracy of their colour classes.

The legend is per coverage: "Edit legend" on the coverage page opens a dialog
with up to 8 rows of "margin at least N dB" and a colour, as Radio Mobile does
it. The default is 30 / 20 / 10 / 0 dB in green to orange; a cell below every
threshold or without data stays transparent. The legend is applied when a
raster is served (`image.png`, the merged view, the PDF), so changing it needs
no recomputation; it is stored in `legend_db` / `legend_colors` (NULL = default)
and a duplicate keeps it. Results computed before margins were stored keep the
colours they were rendered with until recomputed.

"Duplicate" on a site, link or coverage opens the "new" form with the values of the original (name suffixed " (copy)"); nothing is stored until you save it.

"Print report" on a link or coverage page returns `/links/{id}/report.pdf` or
`/coverages/{id}/report.pdf`: one A4 page with the sites, the radio parameters,
the result table, the terrain profile and a map of the path (or the coverage
raster over the map). The PDF is written by the server with `go-pdf/fpdf`
(core fonts, no font files), so it looks the same in every browser and can be
saved or printed from the browser's PDF viewer. The map is composed from the
same tiles the proxy serves (`internal/tiles.Static`); when the tile server is
unreachable the report still comes out, with a note in place of the map.

Light and dark theme are picked from the system setting and can be switched in
the header; the choice is stored in `localStorage` and applied before first
paint. Colours are CSS variables (`--cp-*`) redefined under `[data-theme=dark]`,
which also covers Leaflet's controls, the OSM tiles (inverted in dark mode,
overlays and markers untouched) and the terrain profile SVG. Interface text is
IBM Plex Sans, JetBrains Mono is reserved for numbers and results.

## Map tiles

Browsers never load tiles from OpenStreetMap directly. Leaflet (vendored under
`internal/web/static/leaflet`, BSD-2) requests `/tiles/{z}/{x}/{y}.png` from
Radiopath, which serves them from Redis or fetches them once from the upstream
server with its own User-Agent and stores them for `RADIOPATH_TILE_TTL`. Tiles
and static assets carry ETags, so a browser reload revalidates with 304 instead
of downloading them again. Size the
Redis `maxmemory` with an LRU policy; a few hundred MB is plenty. If you deploy
this publicly, read the OpenStreetMap tile usage policy and consider your own
tile server or a commercial provider via `RADIOPATH_TILE_URL`.

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
  carries ids, tile coordinates and the tokens of `/reset/{token}`. An unmatched
  request is `other`. `/coverages/{id}/events` is an SSE stream of minutes and
  always lands in the `+Inf` bucket.
- `radiopath_coverage_jobs_total{result}` (done, failed, requeued) and
  `radiopath_coverage_job_duration_seconds`. The histogram covers successful
  jobs; a computation that hits the 10 minute timeout is `result="failed"`.
- `radiopath_link_analyses_total{result}`, `radiopath_link_analysis_duration_seconds`.
- `radiopath_dem_tile_cache_total{kind,result}` — raise
  `RADIOPATH_DEM_CACHE_TILES` when the miss share is high. A tile that does not
  exist (sea) counts as a miss.
- `radiopath_s3_requests_total{result}`, `radiopath_map_tiles_total{result}`,
  `radiopath_logins_total{result}`.
- Gauges read from the database on every scrape: `radiopath_users`,
  `radiopath_sites`, `radiopath_links`, `radiopath_sessions_active` (sessions
  that have not expired, so in practice those used in the last seven days) and
  `radiopath_coverages{job_state}`, the last one being the job queue depth. A
  database that is down leaves them out and logs a warning; the scrape still
  succeeds and everything else is still there.

**Query the gauges with `max`, not `sum`.** Every replica queries the same
database and reports the same numbers, so `sum` multiplies them by the replica
count. The counters and histograms are per pod and are summed as usual.

```promql
max(radiopath_coverages{job_state="queued"})            # jobs waiting
sum(rate(radiopath_http_requests_total{code=~"5.."}[5m]))
histogram_quantile(0.9, sum by (le) (rate(radiopath_coverage_job_duration_seconds_bucket[1h])))
```

In Kubernetes the chart puts `containerPort: 9090` on web and worker pods and,
with `metrics.podMonitor.enabled`, adds a PodMonitor that selects both
components directly. There is no Service for the metrics port on purpose: a
Service selector is equality only and could not say "web or worker", and the
existing one skips worker pods in production anyway (`worker.serveWeb: false`).
The PodMonitor needs the Prometheus operator CRD; if your Prometheus filters
PodMonitors by label, set `metrics.podMonitor.labels`.

## Kubernetes

The Helm chart in [radiopath/helmchart](https://github.com/radiopath/helmchart)
renders web pods (`RADIOPATH_WORKERS=0`,
behind a Service and a Traefik Ingress), worker pods (`RADIOPATH_WORKERS=1`, a
computation uses all CPUs up to the pod limit) and a Valkey for the tile cache.
Database and DEM storage are external. The chart's `values-example.yaml` shows the
values a deployment needs; secrets (`DATABASE_URL`, the S3 keys, the SMTP
password) come from an existing Secret named by `envFromSecret`:

```sh
kubectl -n radiopath create secret generic radiopath-env --from-literal=DATABASE_URL=... --from-literal=RADIOPATH_DEM_S3_ACCESS_KEY=... --from-literal=RADIOPATH_DEM_S3_SECRET_KEY=...
git clone https://github.com/radiopath/helmchart && cd helmchart
helm upgrade --install radiopath . -n radiopath -f values-example.yaml --set image.tag=<tag>
kubectl -n radiopath exec -it deploy/radiopath -- /radiopath useradd <name>
```

Workers can be autoscaled on CPU (HPA, needs metrics-server), one job each,
spread over nodes; `worker.serveWeb` also routes web traffic to worker pods,
which is fine for a small setup and slow while a coverage is computing.

`ingress.hosts` lists every name a release answers on. Setting
`ingress.canonicalHost` to one of them makes that host the only one serving the
app: it gets its own Ingress, and all the others share a second Ingress with a
Traefik `Middleware` that answers 301, one URL per site, which is what search
engines want. Two Ingresses because Traefik applies the middleware annotation to
every router of an Ingress, so the canonical host has to stand apart.

Certificates come from cert-manager. `ingress.tls.clusterIssuer` defaults to
`letsencrypt`; the chart writes the `cert-manager.io/cluster-issuer` annotation
and derives the Secret names (`<release>-tls`, and `<release>-redirect-tls` for
the redirect Ingress, since one shared name would leave two Certificates fighting
over one Secret with different SAN lists). Issuance is http-01, so every host in
`ingress.hosts` needs DNS pointing at the cluster before its certificate can be
issued. On a cluster without cert-manager set `ingress.tls.clusterIssuer: ""`,
otherwise Traefik answers with its self-signed default certificate.

- `GET /healthz` liveness, `GET /readyz` readiness (database ping, DEM source
  reachable, Redis ping when configured).
- `GET /metrics` on port 9090, a separate listener that is in no Service and no
  Ingress; scraped by the PodMonitor from `metrics.podMonitor.enabled`, see
  "Metrics" above.
- Memory: about `RADIOPATH_DEM_CACHE_TILES` x 26 MB (x 39 MB with a canopy
  layer) plus 100 MB per pod.
- Shutdown on SIGTERM requeues running jobs and drains connections for up to 10 s.
- Migrations run at startup under a Postgres advisory lock, so any number of
  replicas may start at once.
- Registration is off in the chart; create the first user with `useradd` as above.

## Model notes

- ITM is valid for 20 MHz to 20 GHz, antenna heights 0.5 to 3000 m and paths
  up to 2000 km. Paths below 1 km produce a warning.
- Copernicus GLO-30 (and SRTM) is a surface model (includes trees and
  buildings) and ITM has no clutter model, so predictions in forests and cities are optimistic unless
  the canopy layer is configured (see "Tree canopy").
- The ITM "propagation mode" is derived from smooth-earth horizons. An
  obstructed path inside the smooth-earth line-of-sight range still reports
  "line of sight"; the path loss and the Fresnel clearance show the obstruction.
- Fixed environment: continental temperate climate, N0 = 301, epsilon = 15,
  sigma = 0.005 S/m, 50 % time/location/situation.
- Terrain is sampled at the resolution the elevation model reports (30 m for
  1" tiles, 90 m for 3"), bilinearly interpolated, capped at 4000 samples per
  path.
- Links and coverage use different ITM variability modes. Both ends of a link
  are at known positions, so location variability is switched off and the result
  is given twice: the median and the level available 99 % of the time. Coverage
  uses the mobile mode at the median.
- Vegetation at the ends of a link comes from the canopy layer and ITU-R
  P.833; buildings and anything else stay manual in the extra path loss field.
  Rule of thumb without the layer: about 1 dB per metre of foliage at 5 to
  6 GHz, saturating near 30 to 40 dB behind woodland, a few dB at VHF.
- A link can store the level measured on the built link; the detail page shows
  measured minus predicted, which is the only honest way to learn how far the
  model is off in your terrain.
- Area coverage runs ITM point-to-point on every radial prefix (as Radio Mobile
  and SPLAT! do). Radials are spaced one pixel apart at the outer edge, between
  360 and 3600 of them, one sample per resolution step; pixels take the value of
  the nearest radial sample. Nearby small obstacles therefore cast thin radial
  shadows, which is inherent to the method. The raster is north-up
  equirectangular; its bounds are stored for a later map overlay. Compute runs
  synchronously in the request: 30 km at 100 m takes about 1 s, 60 km about 5 s
  on 10 cores; time grows with roughly (range / resolution)^3 and scales with CPU count
  (GOMAXPROCS, i.e. the container CPU limit in Kubernetes).

## License

AGPL-3.0-or-later, see `LICENSE`: use, modify, self-host and redistribute
freely, as long as the source of your version is published under the same
license, also when it is only offered over a network. If you want to run a
modified Radiopath as a hosted service without publishing your changes, a
commercial license is available, see `COMMERCIAL-LICENSE.md`.

The ITM port in `internal/itm` derives from NTIA public domain software, see
`internal/itm/NOTICE`. Vendored: Leaflet 1.9.4 (BSD-2,
`internal/web/static/leaflet/LICENSE`), JetBrains Mono 2.304 and IBM Plex Sans
(both SIL OFL 1.1, `internal/web/static/fonts/OFL-*.txt`) and four Lucide icons
(ISC) inlined in `internal/web/templates/icons.html`.

Data: terrain from the Copernicus DEM GLO-30 (© DLR e.V. 2010-2014 and
© Airbus Defence and Space GmbH 2014-2018, provided under COPERNICUS by the
European Union and ESA, all rights reserved; free use with this attribution),
canopy from ETH Global Canopy Height 2020 and ESA WorldCover 2021 (both
CC BY 4.0, see "Tree canopy"), map tiles © OpenStreetMap contributors (ODbL).
