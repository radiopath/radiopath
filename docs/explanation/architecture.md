# Architecture

How the parts of Radiopath fit together and why they are arranged this way.

## One binary

`cmd/radiopath` reads its configuration from the environment and starts, in one process, the HTTP server, the
coverage workers (`RADIOPATH_WORKERS`), an hourly janitor (expired sessions, mail tokens, API tokens expired for 30 days and rate limit entries,
accounts never confirmed), and a second listener for
Prometheus metrics. The same binary has a few subcommands: `openapi` prints the API description without a
database, `useradd`, `userdel` and `users` manage accounts. Everything the server delivers is compiled in: database
migrations, the admin templates and the built web app. A pod therefore needs only the binary, PostgreSQL with
PostGIS, optionally Valkey for map tiles, and the terrain tiles.

Web and compute can be scaled separately with the same image: pods with `RADIOPATH_WORKERS=0` only serve
requests, pods with workers compute. All state is in PostgreSQL (sessions, rate limits, the job queue), so any
replica can serve any request.

## Jobs

Everything that needs the terrain or HF data runs as a job in the table `jobs`, claimed by workers with `FOR UPDATE
SKIP LOCKED`: coverages by the coverage workers (`RADIOPATH_WORKERS`, minutes each), link analyses, HF predictions
and HF maps by a pool of short-job workers (one per CPU, seconds each), so a long coverage never delays a link. A job carries a snapshot of
its input (sites, antennas with their patterns, parameters) and the hash of it; a request that asks for the same
input gets the existing job, so a finished job is the cache of its result for an hour. Coverage jobs are followed
by the page over the event stream; for everything else the request itself queues the job and waits for it, woken by
a PostgreSQL notification (`LISTEN/NOTIFY`, polling as a fallback), at most 25 seconds, then it reports the server
as busy. The API therefore keeps its synchronous shape while the web process computes nothing.

## Request routing

`internal/web/server.go` builds one `http.ServeMux`, wrapped in security headers, metrics and Go's
`http.CrossOriginProtection` (which replaces CSRF tokens by checking `Sec-Fetch-Site` and `Origin`):

- `/api/…` is the JSON API. A small middleware puts the caller (session user or API token) and request details
  into the context, and [huma](https://huma.rocks) handles validation and the OpenAPI description.
- Raster images, PDF reports and the job event stream are plain HTTP routes next to the API, because they return
  binary data or a stream.
- `/s/…` serves share links: the page itself is the web app, its images and reports are rate limited per token.
- `/admin` is the admin panel, the only HTML the server renders itself.
- `/tiles/…` is the map tile proxy, `/healthz` and `/readyz` the probes.
- Every other path returns a file of the web app, or its `index.html` so the app can route in the browser.

## The JSON API

Operations live next to their resource in `internal/web` (`sites.go`, `antennas.go`, `links.go`,
`coverages.go`, `account.go`, `auth.go`, …). Each is registered with a scope such as `sites:write`; operations
without one are public, and a test keeps the list of public operations explicit. Requests and responses are
separate Go types with snake_case JSON keys, never the store structs, which carry internal fields such as the
owner. huma generates the OpenAPI description from these types; `make api` writes it to `web/openapi.json` and
turns it into TypeScript types for the web app, so server and client cannot drift apart unnoticed.

Validation happens on the server: one 422 lists every invalid field. The web app only checks what the browser can
know (a value that is no number) before sending.

A session may call every operation with a scope. A personal API token, which lets other programs and scripts use
the API, may call only the operations whose scope it was given, never those marked `scopeSession` (account
settings, tokens); `gate` checks this for every operation, `requireUser` for the binary routes.

## Data and ownership

`internal/store` talks to PostgreSQL with pgx; migrations are embedded and run at startup under an advisory lock.
Every object a user creates carries its owner, and every query is scoped to it. Someone else's id therefore
behaves exactly like one that does not exist (404), so there is no separate permission check that could be
forgotten. Per-user limits (sites, coverages, concurrent jobs) keep an open registration from filling the database
or blocking the workers.

## Coverage jobs

Computing a coverage takes from a second to minutes, too long for a request. "Compute" therefore only marks the
coverage as queued. Workers claim the oldest queued job with `FOR UPDATE SKIP LOCKED`, compute the link margin
along radials, and store a coloured PNG together with the margins as an 8-bit raster. A job running longer than
15 minutes is considered lost and taken over; shutdown requeues running jobs. The page of a coverage follows its
job over server-sent events and reloads the result when the job is done.

Because the margins are stored, a legend can be changed without recomputing, and several coverages can be merged
on the server into one image where the best margin wins. See
[Maps, legends and reports](maps-legends-and-reports.md).

## The web app

`web/` is a SvelteKit app (Svelte 5, TypeScript, Tailwind CSS 4, shadcn-svelte, TanStack Query, Leaflet), built as
a static single-page app and embedded into the binary. It talks to the server only through the JSON API and the
binary routes; plots of terrain profiles and antenna patterns come from the server as SVG, so the PDF reports and
the app draw them the same way. Data is loaded with TanStack Query; the generated types make a renamed field a
compile error in the app.

In development, Vite serves the app with hot reload on port 5173 and forwards everything the Go server answers to
it on port 8080, so the browser sees one origin, exactly as in production.

The admin panel deliberately does not use the app: it is a few server-rendered pages with their own CSS, so it
keeps working whatever happens to the app or the user accounts.

## Map tiles and reports

Browsers never load map tiles from OpenStreetMap directly; the proxy fetches each tile once, caches it in Valkey and
rate limits clients that walk the tile pyramid. PDF reports are composed on the server from the same tiles, so they
look identical in every browser. Details: [Operations](../reference/operations.md#map-tiles).

## Where to look in the code

| Package | Responsibility |
|---|---|
| `internal/web` | routes, API operations, auth, admin panel, shares, reports, job events |
| `internal/webui` | embeds the built web app |
| `internal/store` | PostgreSQL access and migrations |
| `internal/worker`, `internal/coverage` | coverage jobs and the radial computation |
| `internal/link`, `internal/itm` | point-to-point analysis and the ITM port |
| `internal/dem`, `internal/s3` | terrain and canopy tiles from a directory or S3 |
| `internal/antenna`, `internal/plot` | antenna patterns, SVG plots, raster PNGs, legends |
| `internal/report`, `internal/tiles` | PDF reports, map tile proxy |
| `internal/mail`, `internal/metrics`, `internal/geo` | SMTP, Prometheus collectors, geodesy |
| `web/` | the web app |
| `e2e/` | Playwright end-to-end tests |
