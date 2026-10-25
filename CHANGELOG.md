# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Log in with the user name or the confirmed address of the account (by @HB9HIL)
- Personal API tokens for other programs and scripts, under Account → API tokens: access per resource (none, read, read and write), an expiration date or none, shown once, revocable at any time; creating one needs the password and is mailed to the confirmed address (by @HB9HIL)
- Log out every other device of the account under Account → Security (by @HB9HIL)
- An interactive API reference in the app (Account → API reference), with the scope of every operation, code examples and test requests; bundled, nothing is loaded from outside (by @HB9HIL)
- JSON API under `/api`, described by OpenAPI at `/api/openapi.json` for logged-in users; the web app is built on it, other programs can use it too (by @HB9HIL)
- The app and the admin panel send a Content-Security-Policy: scripts only from the app itself and its own inline scripts by hash, no plugins, no framing, no connections elsewhere (by @HB9HIL)

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
