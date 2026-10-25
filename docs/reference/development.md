# Development commands and CI

## Development environment

The host needs Go (the version in `go.mod`) and Docker. PostgreSQL/PostGIS, Valkey, Mailpit, Node and Playwright
run in containers from `docker/compose.yaml`; Node only in the compose services `node` and `vite`, so
`web/node_modules` is built for Linux and must not be installed from the host.

`make dev` creates `.env` from `.env.example` on first start, clones the terrain and canopy tiles of
[radiopath/testfiles](https://github.com/radiopath/testfiles) (N47E009 only) into `tmp/testfiles`, starts PostGIS,
Valkey and Mailpit (http://localhost:8025), runs the server in the background with `RADIOPATH_BASE_URL` pointing
at the Vite port, creates the user `dev` / `devpassword` and runs the Vite dev server of `web/` in the foreground
on http://localhost:5173. Vite proxies everything the Go server answers (`/api`, `/admin`, tiles, images,
reports, job events) to port 8080; the list is the regular expression in `web/vite.config.ts`. Ctrl-C stops
everything, including the database containers.

The HF model needs the ITU data set (131 MB, pinned and checked by SHA-256), which is not part of the testfiles:
run `scripts/fetch-hf-data.sh` once, after the clone, and it fills `tmp/testfiles/hf1` (the dataset root from
`.env`); without it the HF page answers that the model is not configured. The HF tests read it from there as
well, or from `hf1` below an absolute `RADIOPATH_DATASET_DIR`, and skip without it.

## Make targets

| Command | What it does |
|---|---|
| `make dev` | the local stack, see above |
| `make test` | `go test ./...`; store tests that need SQL skip without `DATABASE_URL` |
| `make vet` | `go vet ./...` |
| `make check` | `svelte-check` and the Prettier check of the web app |
| `make format` | formats TypeScript and Svelte with Prettier (`web/.prettierrc`) |
| `make api` | writes `web/openapi.json` (`radiopath openapi`) and `web/src/lib/api.gen.ts`; commit both |
| `make web` | builds the web app into `internal/webui/dist`, which the binary embeds |
| `make build` | web app and a local binary in `bin/radiopath` |
| `make e2e` | web app, Linux binary, e2e image and all Playwright specs; see [Run and write end-to-end tests](../how-to/run-and-write-e2e-tests.md) |
| `make e2e-run` | the specs against the binary already in `bin/`, as CI does |
| `make docker` | the production image from `docker/Dockerfile` |

Other checks CI runs:

| Command | What it checks |
|---|---|
| `gofmt -l cmd internal` | Go formatting |
| `go tool staticcheck ./...` | static analysis, including unused code (pinned as a tool in `go.mod`) |
| `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` | known vulnerabilities in called code |
| `python3 scripts/funcdoc.py` | a doc comment on every Go, TypeScript and Svelte function (except generated files and `web/src/lib/components/ui`) |

`go vet`, `go test` and `go build` need `internal/webui/dist` to exist. `make test` and `make vet` create an empty
placeholder when the web app has not been built; CI does the same.

## Generated files

| File | Source | Regenerate with |
|---|---|---|
| `web/openapi.json` | the huma operations and their Go types | `make api` |
| `web/src/lib/api.gen.ts` | `web/openapi.json` | `make api` |
| `web/src/lib/components/ui/` | the shadcn-svelte CLI | its `add` command in the node container |
| `internal/webui/dist/` | the web app | `make web` (not committed) |

## CI (`.github/workflows/ci.yml`)

Runs on every push to main, on tags `v*` and on pull requests; a newer push cancels the running workflow of the
same branch, tag runs always finish.

| Job | Does |
|---|---|
| `go` | gofmt, vet, staticcheck, OpenAPI freshness, `go test -race ./...` |
| `govulncheck` | govulncheck (not required for images, so a new report does not block releases) |
| `funcdoc` | the doc comment check |
| `web-check` | freshness of `api.gen.ts`, svelte-check, Prettier |
| `web` | builds the web app, passes it on as an artifact |
| `binary` | static Linux binaries for amd64 and arm64 with that web app |
| `e2e` | the Playwright specs against the amd64 binary |
| `docker-image` | on pushes: packs both binaries with `docker/ci.Dockerfile` into a multi-arch image `ghcr.io/radiopath/radiopath` tagged `:<sha>`, plus `:staging` on main or `:X.Y.Z` and `:latest` on a tag |
| `release` | on tags: the GitHub release with the changelog section as notes and the binaries attached |

A second workflow, `.github/workflows/codeql.yml`, runs GitHub CodeQL on pushes to main, on pull requests and every
Monday: taint analysis of the Go code (built with the `dist` placeholder, since the autobuilder cannot handle the
embedded web app), the TypeScript of the web app (generated files excluded) and the workflows themselves. Findings
appear in the repository's Security tab and as annotations on pull requests; they do not block CI or releases. The
"default setup" of code scanning must stay off, it cannot run next to this workflow.

Every run packs the binaries it built itself: the version comes from `git describe` at build time, so an image is
never retagged from another run. Each job has its own Go build cache, keyed by job and architecture.

Renovate keeps dependencies current and pins Go to 1.27, PostGIS to 17, Valkey to 9.1 and Node to 26
(`renovate.json`).
