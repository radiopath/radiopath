# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- The configuration reference explains `pool_max_conns` in `DATABASE_URL` and how to size it against the server's `max_connections` (by @HB9HIL)
- `make clean` removes the build and test output (by @HB9HIL)

## [2.3.0] - 2026-10-25

### Added
- Every API operation that needs a login counts against 600 requests per user and minute, sessions and tokens together, and answers 429 beyond that; the per-token limit of 300 stays (by @HB9HIL)

### Fixed
- The API reference groups the HF operations under their own tag "HF" instead of "Account" (by @HB9HIL)

## [2.2.0] - 2026-10-25

### Added
- The MUF and reliability charts of the HF page show a crosshair with the values of the hour under the pointer, shared by both charts and the reliability grid, which marks the column of that hour (by @HB9HIL)
- The HF map draws the great circle from A to B, dashed for the long path (by @HB9HIL)
- The HF map shades the night side of the earth for the hour shown (the 15th of the month, declination by Cooper's formula) and marks the sun (by @HB9HIL)
- The HF page shows a prediction as one grid per frequency with a row per month and a column per hour: the cell is the reliability, coloured like the map, with SNR, field strength, MUF and propagation mode in its tooltip; clicking a month of a range shows its map and charts (by @HB9HIL)
- The HF page sums a prediction up in four tiles above the map: path length, the most reliable hour and frequency, the span of the median MUF over the day and the sunspot number used (by @HB9HIL)
- The HF page scrolls to the results when a prediction is done, counts the months of a range on the button while they run, and says so above the results when the form changed since they were predicted (by @HB9HIL)
- The HF page remembers its form in the browser (nothing is stored on the server), shows the TX power in watts next to the dBm field, and fills the sunspot number from NOAA again when the typed one is cleared (by @HB9HIL)

### Changed
- The HF form lays its four blocks out in two columns on wide screens, circuit and stations above time and signal, and the Predict button says what it will ask for, like "3 months, 24 hours, 2 frequencies" (by @HB9HIL)
- The HF reliability map is a flat world map drawn as SVG (Natural Earth coastlines, a rectangle per cell with its value as tooltip, markers A and B) instead of a Leaflet map on OpenStreetMap tiles: every cell has the same size, nothing is loaded from the tile server (by @HB9HIL)
- The HF page asks for the time one way: a first and a last month (the same for one month) and the hours as from, to and every, which used to be the "several months" mode only; the year, month and hour list fields are gone and a prediction covers every hour of the day by default (by @HB9HIL)

### Removed
- The HF table by hour and frequency; the grid holds its figures (by @HB9HIL)

## [2.1.0] - 2026-10-25

> With the Helm chart, update to 2.2.0 together with this image: it runs web pods in `RADIOPATH_MODE=web` (no datasets, less memory) and worker pods in `RADIOPATH_MODE=worker` with the HF data set, which the workers compute from now on. An older chart leaves the workers without HF data, and HF answers 503 until both are updated.

### Added
- `RADIOPATH_MODE` splits the process: `web` serves the app and the API without datasets, S3 or `/tmp` and computes nothing; `worker` computes coverages, link analyses and HF with the datasets and serves only `/healthz` and `/readyz`; `all` (the default) does both as before (by @HB9HIL)

### Changed
- The HF model builds the 24 hour control point table of a long path (7000 km and more) once per path instead of once per hour and frequency: a prediction of 24 hours and 30 frequencies takes 14 ms instead of 650 ms, a reliability map 1.5 s instead of 9 s of CPU time, with results identical to the bit (by @HB9HIL)
- An isotropic HF antenna shares one elevation row between its 360 azimuths instead of allocating a 360 x 91 pattern: a reliability map allocates 110 MB instead of 470 MB (by @HB9HIL)
- The magnetic field series of the HF model evaluates its sines, cosines and powers once per term instead of three times: a prediction takes 8 ms instead of 15 ms, a map 0.7 s instead of 1.5 s of CPU time. The results are identical on amd64; on arm64 the compiler fuses the multiply-adds differently, which can change the last bit (by @HB9HIL)
- Workers and the job event stream are woken by PostgreSQL notifications (`LISTEN/NOTIFY`) when a job is queued or finished, instead of polling every second or two; polling stays as a fallback every 10 s (by @HB9HIL)
- Link analyses are computed by the worker processes as jobs instead of in the web server: `GET /api/links/{id}`, the shared link view and the link report queue the analysis, wait for the result (woken by a notification, at most 25 s, then "The server is busy") and answer as before. A finished analysis is kept for an hour and served again while the link, its sites and antennas stay the same; the janitor removes old ones. Limits: 20 such jobs queued or running per user, 500 queued in all (by @HB9HIL)
- HF predictions and maps are computed by the workers as jobs too, with the same waiting, caching and limits; the one-map-at-a-time slot of the web server is gone, the short-job pool bounds the maps instead. A server whose workers have no HF data set answers 503 as before. The sunspot numbers are fetched whenever `RADIOPATH_SUNSPOT_URL` is set, no longer only with the HF data set (by @HB9HIL)
- The NOAA sunspot files are cached in Valkey for 12 hours when `RADIOPATH_REDIS_URL` is set, so all web replicas together download them once instead of each on its own (by @HB9HIL)
- Metrics: `radiopath_jobs_total{kind,result}` and `radiopath_job_duration_seconds{kind}` replace `radiopath_coverage_jobs_total` and `radiopath_coverage_job_duration_seconds`; the gauge `radiopath_jobs{kind,state}` counts the jobs in the queue table (by @HB9HIL)

### Refactored
- Coverage jobs live in a table `jobs` of their own (migration 0023 moves them, the `job_*` columns of a coverage are gone) that is built to hold every kind of job: owner, kind, input snapshot and its hash, state, attempts, result. A coverage shows its newest job; a job claimed three times without finishing fails instead of being taken over again (by @HB9HIL)

### Chore
- The e2e stack turns the sunspot download off and the "HF not configured" spec mocks the answer like the other HF specs, so `make e2e` behaves the same whether `tmp/testfiles/hf1` exists on the host or not (by @HB9HIL)
- Benchmarks of an HF prediction and of a reliability map, and a golden test that hashes every reference case bit for bit (`testdata/golden_<arch>.txt`), so performance work on the HF model can prove it left the results alone (by @HB9HIL)

## [2.0.0] - 2026-10-25

> ⚠️ **Breaking change:** terrain, canopy and HF data now come from one dataset root with a folder per dataset.
> Before updating, replace `RADIOPATH_DEM_*` and `RADIOPATH_CANOPY_*` with the
> `RADIOPATH_DATASET_*` variables (table in docs/reference/configuration.md) and put the tiles under their dataset
> folder (`srtm1/`, `canopy1/`, in the image `/data/srtm1` instead of `/data/dem`). A server that still has one of the old
> variables set refuses to start. With the Helm chart, update to 2.1.0 or later: it uses the new names and gives
> the pods the `/tmp` volume and memory the HF model needs.

### Added
- Log in with the user name or the confirmed address of the account (by @HB9HIL)
- Personal API tokens for other programs and scripts, under Account → API tokens: access per resource (none, read, read and write), an expiration date or none, shown once, revocable at any time; creating one needs the password and is mailed to the confirmed address (by @HB9HIL)
- Log out every other device of the account under Account → Security (by @HB9HIL)
- An interactive API reference in the app (Account → API reference), with the scope of every operation, code examples and test requests; bundled, nothing is loaded from outside (by @HB9HIL)
- JSON API under `/api`, described by OpenAPI at `/api/openapi.json` for logged-in users; the web app is built on it, other programs can use it too (by @HB9HIL)
- The app and the admin panel send a Content-Security-Policy: scripts only from the app itself and its own inline scripts by hash, no plugins, no framing, no connections elsewhere (by @HB9HIL)
- HF propagation library `internal/hf`: Go port of ITU-R P.533-14 (MUF, field strength, SNR, circuit reliability) and P.372 radio noise, validated against the ITU reference implementation; reads the ITU data set from the dataset `RADIOPATH_DATASET_HF` (default `hf1`): a folder of the dataset root, or with S3 downloaded and checked by SHA-256 at startup (fetch it with `scripts/fetch-hf-data.sh`), and is served by `POST /api/hf/predictions`, with isotropic antennas or the user's antenna patterns; a data set that cannot be used only turns the HF model off, the rest of the app runs (by @iu2frl)
- HF prediction page in the app (HF in the header and the tab bar): choose two sites (or take them, the antennas and the aiming from a link), the month, hours, frequencies, antennas with their azimuth, the TX power in dBm, a mode (AM, FM, SSB, PSK31, RTTY, FT8, FT4 or custom bandwidth, required SNR and digital parameters) and the noise environment, the short or the long path, and read MUF, field strength, SNR and reliability per hour and frequency, with charts of the MUF and the reliability over the day, and a world map of the reliability from site A with a slider for the hour (`POST /api/hf/maps`, one map at a time per server, the others wait their turn); nothing is stored (by @iu2frl)
- `/readyz` also checks that the HF data files are still present when the HF data set is loaded (by @iu2frl)
- API tokens can be given the `hf:read` scope for the HF predictions, maps and sunspot numbers (by @iu2frl)
- The HF page takes one month with its hours, or several months with the hours from, to and a step; a range is one prediction per month, with a month selector for the charts and the map and the month in the table. The model has no day resolution, so no day is asked for (by @iu2frl)
- Info buttons in the corner of the HF page cards (MUF, circuit reliability, map, table) explain what the figures mean (by @iu2frl)
- The sunspot number of the HF page is filled in from NOAA SWPC: the server loads the smoothed observed and predicted values every 12 hours (`GET /api/hf/sunspots`, `RADIOPATH_SUNSPOT_URL`, empty turns it off) and the page offers the value of the chosen month until you type your own (by @iu2frl)

### Changed
- **Breaking:** terrain and canopy tiles live under one dataset root, a directory (`RADIOPATH_DATASET_DIR`) or an S3 bucket (`RADIOPATH_DATASET_S3_*`), with one folder per dataset: `RADIOPATH_DATASET_DEM` (default `srtm1`) and `RADIOPATH_DATASET_CANOPY` (default `canopy1`, empty for none). `RADIOPATH_DEM_DIR`, `RADIOPATH_DEM_S3_*`, `RADIOPATH_CANOPY_DIR` and `RADIOPATH_CANOPY_S3_PREFIX` are no longer read and stop the server at startup; the images look in `/data/srtm1` instead of `/data/dem`. See the table in docs/reference/configuration.md; for development run `git -C tmp/testfiles pull` (by @HB9HIL)
- The account page is split into the tabs Profile, Security, API tokens and API reference, each section with its explanation next to the form (by @HB9HIL)
- Login, registration and the password pages share one centred layout that fits phones; apart from the login page they are not indexed by search engines (by @HB9HIL)
- The web interface is a SvelteKit app embedded in the binary instead of server-rendered pages: no reload after saving, a coverage page picks up its finished job by itself. Page, share and mail link URLs stay the same; only the admin panel is still rendered by the server (by @HB9HIL)
- Rasters, reports and job events answer 401 without a session instead of redirecting to the login (by @HB9HIL)
- The header carries the account, the theme, the API description and logout in one menu; on phones the sections sit in a tab bar at the bottom (by @HB9HIL)
- The end-to-end tests run on a phone as well: tab bar, row menus and no page wider than the screen (by @HB9HIL)
- Link and coverage pages open with their key figures as tiles; on phones the map fits the screen and the less used actions sit in a menu (by @HB9HIL)
- Lists keep their row actions in one menu, show fewer columns on phones, an empty state with a way forward and a placeholder while loading (by @HB9HIL)
- Messages and errors after an action appear as toasts, deleting asks in a dialog of the app instead of the browser's own (by @HB9HIL)
- `make dev` serves the app on http://localhost:5173, from the Vite dev server in Docker that proxies to the Go server (by @HB9HIL)
- Changing the address of an account needs its password, and the old address gets a notice once the new one is confirmed (by @HB9HIL)
- Confirmation and reset links carry their token in the URL fragment (`/verify#…`, `/reset#…`), which never reaches a server log; links mailed before the update no longer work, request a new one (by @HB9HIL)
- Failed logins count per user name and IP (5) and per user name from all IPs (100) instead of per name alone, so a stranger can no longer lock an account out with five wrong passwords (by @HB9HIL)
- One user can hold at most five job event streams at a time (by @HB9HIL)
- One user can run at most 60 link analyses, PDF reports and raster merges per minute, so a single account cannot keep the server busy (by @HB9HIL)
- Registration mails at most three times per hour to the same address, so nobody can flood a mailbox from many IPs (by @HB9HIL)
- The server drops requests whose headers and body take longer than 30 seconds to arrive and idle connections after two minutes (by @HB9HIL)

### Fixed
- Changing the address on a server without mail configured crashed the whole server; it now answers that mail is not configured, and a mail without a mailer is only logged (by @HB9HIL)
- Registration no longer tells whether a user name or an address is taken: it answers the same in every case, and only the mail to that address says what happened. An unconfirmed registration no longer blocks its address for others (by @HB9HIL)
- Mails of the public account pages go out in the background, so response times no longer tell whether an address exists (by @HB9HIL)
- The login no longer follows a `next` parameter that the browser reads as another site, such as `/<tab>/evil.example` (by @HB9HIL)
- Failed link and coverage calculations show a fixed reason instead of the internal error, which could name tile files, hosts or the storage bucket, also on shared pages; stored job errors are replaced by the migration (by @HB9HIL)
- `/readyz` no longer names the failing part and its error, the log does (by @HB9HIL)
- The version is shown to logged-in users only, in the footer and `/api/me`; `/api/config` and the API description no longer carry it (by @HB9HIL)
- Share URLs with a made-up token answer 404 without leaving a rate limit row behind (by @HB9HIL)
- The cookies that logout and admin logout clear are marked `Secure` (behind HTTPS) and `SameSite` like the ones they replace (by @HB9HIL)
- A tab opened before a deploy loads the new version instead of failing on script chunks the new build no longer has (by @HB9HIL)
- Legend colours are parsed byte by byte, so a colour can never be truncated on its way into a raster or report (by @HB9HIL)
- `scripts/sync-canopy-s3.sh` works south of the equator; a negative latitude was taken for an option (by @HB9HIL)

### Removed
- The API docs page `/api/docs`, which loaded its scripts from a CDN; `/api/openapi.json` and `/api/schemas/` need a login (by @HB9HIL)
- The `worker` field of coverage jobs and their event stream, which named the pod (by @HB9HIL)
- `GET /api/password/reset/{token}`, replaced by `POST /api/password/reset/check` with the token in the body (by @HB9HIL)
- The Tailwind build of the server-rendered pages (`make css`, `internal/web/ui`, the CI job `lint-css`) (by @HB9HIL)

### Chore
- CI downloads the pinned, checksummed ITU data set and runs the HF model against 26 reference cases made with the original ITU C program (by @iu2frl)
- CI job `hf-reference` rebuilds the ITU C program from its pinned sources and checks that it still writes the committed HF reference reports (by @iu2frl)
- Prettier formats TypeScript and Svelte (`make format`), checked by `make check` and in CI (by @HB9HIL)
- The Docker build context holds only the sources (`.dockerignore` as an allowlist), so a local `.env` or terrain data no longer reach it (by @HB9HIL)
- `CONTRIBUTING.md`: development setup, code layout, conventions, the checks before a pull request and how the changelog is kept (by @HB9HIL)
- The `cookie` package of SvelteKit is pinned to 0.7.2 (advisory for versions below 0.7.0); SvelteKit 3 is held back in Renovate until the UI components support it (by @HB9HIL)
- CodeQL code scanning of the Go code, the web app and the workflows on every push to main, pull request and weekly (by @HB9HIL)
- The documentation moved from the README into `docs/`, organised as tutorials, how-to guides, reference and explanation (Diátaxis), with a new development tutorial, an architecture overview and guides for API operations, end-to-end tests and releases (by @HB9HIL)
- The report tests use public summits as sample sites (by @HB9HIL)

## [1.2.8] - 2026-10-25

### Fixed
- Release images showed the version of the preceding main build (1.2.7 showed `v1.2.5-19-g1ca860e`): every CI run now packs its own binaries instead of reusing the commit's image (by @HB9HIL)

### Chore
- Renovate keeps the node image on version 26 (by @HB9HIL)

## [1.2.7] - 2026-10-25

### Changed
- `make dev` starts the local stack (PostgreSQL, Valkey, Mailpit, stylesheet watcher) in Docker and the server in the foreground, with a dev user and terrain from radiopath/testfiles; `make run`, `make db-up` and `make db-down` are gone. Node only runs in Docker, the host needs Go and Docker (by @HB9HIL)
- One CI workflow instead of two: the binaries are built once and reused for e2e, the image and the release; new checks staticcheck, govulncheck and a doc comment on every function. Images are also tagged with the commit (`:<sha>`) (by @HB9HIL)

### Fixed
- Data race on the map tile cache key prefix under concurrent tile requests; the per-IP tile limit now always uses the same key (by @HB9HIL)

### Refactored
- Using Playwright for e2e testing instead of Cypress, every service in its own container, on real terrain and canopy (by @HB9HIL)

### Chore
- Update to Go 1.27 (by @HB9HIL)
- Helm chart documentation moved to radiopath/helmchart (by @HB9HIL)
- Doc comments for all functions (by @HB9HIL)

## [1.2.6] - 2026-10-14

### Chore
- Updated renovate rules to pick up all necessary versions (by @HB9HIL)
- Dependency Updates (by @renovate)

## [1.2.5] - 2026-10-14

### Chore
- Dependency Updates (by @HB9HIL)

## [1.2.4] - 2026-10-10

### Chore
- Dependency Updates (by @HB9HIL)

## [1.2.3] - 2026-10-02

### Fixed
- Fixed some fuzzy log messages (by @HB9HIL)

## [1.2.2] - 2026-09-28

### Chore
- Dependency Updates and Pipeline fixes (by @HB9HIL)

## [1.2.1] - 2026-09-28

### Chore
- Dependency Updates (by @HB9HIL)

## [1.2.0] - 2026-09-20

### Added
- Added a rate limiter to the tile proxy (by @HB9HIL)

### Changed
- Dev stack now runs on 9.1-alpine aswell, just as the helmchart in production. (by @HB9HIL)

## [1.1.0] - 2026-09-19

### Added
- Every commit to main branch now builds a :staging image for the staging environment (by @HB9HIL)

### Changed
- Sessions now expire seven days after they were last used instead of thirty days after login, with thirty days from login as a hard cap. Existing sessions are pulled into the shorter window the next time they are used. (by @HB9HIL)

### Fixed
- The ci pipline was still on push to master, fixed to main (by @HB9HIL)

### Chore
- Moved stuff in the repo around to make it a bit more organized (by @HB9HIL)

## [1.0.0] - 2026-09-19

### Added
- Initial public release: point-to-point link analysis and area coverage with the NTIA Irregular Terrain Model, antenna patterns, tree canopy loss, share links, PDF reports, admin panel, Prometheus metrics and a Helm chart. (by @HB9HIL)
