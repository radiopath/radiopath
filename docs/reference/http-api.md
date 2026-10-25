# HTTP API

The app uses the same JSON API that other programs can use, under `/api`. It is
described by OpenAPI 3.1 at `/api/openapi.json` (for logged-in users and tokens only),
shown as an interactive reference at `/account/api-reference` in the app, generated from the Go types with
[huma](https://huma.rocks) (`internal/web/api.go`, operations next to their
resource in `sites.go`, `links.go`, `coverages.go`, ...). Errors are RFC 7807
problem details; invalid input answers 422 with one entry per field in
`errors[].location` (`body.lat`). Authentication is the session cookie of
`POST /api/session` or a personal API token (below). Rasters, reports and the job event stream of a coverage
(`/coverages/{id}/image.png`, `composite.png`, `report.pdf`, `events`) are plain
HTTP routes next to the API, as are their shared variants under `/s/`.

## Authentication

The web app uses the session cookie. Other programs send a personal API token, created under **Account → API tokens**
in the app ([how-to](../how-to/use-the-api-with-a-token.md)), as `Authorization: Bearer rp_<43 characters>`.

- A request with an `Authorization` header is judged by the token alone, never by a cookie it also carries.
- Every operation names the scope a token needs (`security` in the description, "API token scope" in its text):

  | Scope | Reaches |
  |---|---|
  | `sites:read`, `sites:write` | `/api/sites` |
  | `antennas:read`, `antennas:write` | `/api/antennas` |
  | `links:read`, `links:write` | `/api/links`, `links:read` also `/links/{id}/report.pdf` |
  | `coverages:read`, `coverages:write` | `/api/coverages`, `coverages:read` also the rasters, report and events |
  | `hf:read` | `POST /api/hf/predictions`, `POST /api/hf/maps`, `GET /api/hf/sunspots`; there is no write scope, nothing is stored |
  | `account:read` | `GET /api/me` |

  A write scope is stored together with the read scope of the same resource. Account settings
  (`/api/me/email`, `/api/me/password`, `DELETE /api/me/sessions`, which logs out every other session and
  answers `{"count": n}`) and token management (`/api/me/tokens`) need a session.
- A token has an optional expiry; without one it lives until it is revoked. Expired tokens stay listed for 30 days.
- Answers: 401 with `WWW-Authenticate: Bearer` for a malformed, unknown, expired or revoked token; 403 when the
  scope is missing or the operation needs a session; 429 after 20 bad tokens per IP in 15 minutes, 300 requests
  per token and minute or 600 per user and minute over all sessions and tokens; 503 in maintenance mode.
- `GET /api/me/tokens` lists the tokens of the session's user, `POST /api/me/tokens` creates one (name, scopes,
  optional `expires_at`, the account password) and returns the token once, `DELETE /api/me/tokens/{id}` revokes it.
  At most 20 tokens per user.

## Routes outside the JSON API

| Route | Auth | Answer |
|---|---|---|
| `GET /links/{id}/report.pdf` | session or `links:read` | PDF report of a link |
| `GET /coverages/{id}/image.png` | session or `coverages:read` | coverage raster, coloured with its legend |
| `GET /coverages/{id}/composite.png?with=1,2` | session or `coverages:read` | this raster merged with other computed coverages; bounds in `X-Raster-Bounds` (S,W,N,E) |
| `GET /coverages/{id}/report.pdf` | session or `coverages:read` | PDF report of a coverage |
| `GET /coverages/{id}/events` | session or `coverages:read` | server-sent events with the job state (`state`, `since` in RFC 3339, `error`) until it is final; at most 5 open streams per user, 429 beyond |
| `GET /s/l/{token}/report.pdf`, `GET /s/c/{token}/{image.png,composite.png,report.pdf}` | share token | the same for shared links and coverages; overlays only as released by the owner |
| `GET /tiles/{z}/{x}/{y}.png` | none | map tiles through the proxy |
| `GET /healthz`, `GET /readyz` | none | liveness and readiness; `not ready` without details |

Without a session or token these routes answer 401 (403 for a token without the scope); the web app sends the browser to the login.

## HF prediction

`POST /api/hf/predictions` (scope `hf:read`, session) predicts an HF circuit between two of the user's sites with
ITU-R P.533-14 and P.372 (`internal/hf`) and stores nothing. It counts towards the 60 calculations per minute of a
user (429 beyond that). The web app offers it on the page `/hf`. It answers 503 until the HF data set is loaded (the
`hf1/` folder of the dataset root, see `RADIOPATH_DATASET_HF`), and 404 for a site of another user. The body names the sites and the conditions:

```json
{
  "site_a_id": 1, "site_b_id": 2,
  "year": 2026, "month": 4, "hours_utc": [6, 12, 18], "ssn": 60,
  "frequencies_mhz": [7.1, 14.2],
  "tx_power_dbw": 0, "bandwidth_hz": 3000, "required_snr_db": 15, "snr_percentile": 90,
  "noise_environment": "rural"
}
```

At most 24 hours and 30 frequencies (1 to 30 MHz). Antennas are isotropic with `tx_gain_dbi` and `rx_gain_dbi`, or
the user's antennas (`tx_antenna_id`, `rx_antenna_id`, aimed with `tx_azimuth_deg`, `rx_azimuth_deg`, by default at
the other end): the gain at a direction is the antenna's gain less the attenuation of its horizontal pattern (azimuth
offset) and vertical pattern (elevation above the horizon), as for the links, so an antenna measured at VHF is only a
rough model at HF. `modulation` is `analog` or `digital` (with optional `digital` parameters), `long_path` takes the
long way round the
globe. The answer has one entry in `points` per hour and frequency: MUFs, field strength, receiver power, noise,
SNR, circuit reliability and the dominant mode with its delay, elevation and loss. The value -307 means that the
quantity does not apply to the path. Beyond 7000 km the model reports the field strength of the whole path and no
single modes, so `dominant_mode` is `NONE` there. `overall_reliability_pct` is only calculated for digital
modulation up to 9000 km and is 0 otherwise; use `basic_reliability_pct` for analog modulation.

`POST /api/hf/maps` (scope `hf:read`, session) answers the same question for the whole globe: the circuit from one
site to the centre of every 10 degree cell, for one frequency and the requested hours, with isotropic antennas
(`tx_gain_dbi`, `rx_gain_dbi`). The body is that of a prediction with `site_id` instead of the two sites, one
`frequency_mhz` instead of a list and no antennas or `long_path`. The answer has `step_deg`, `rows`, `cols` and a frame
in `frames` per hour, each with `reliability_pct`: one integer per cell, rows from north to south and cells from west
to east, the same reliability as in the prediction (overall for digital modulation up to 9000 km, basic otherwise).
The value -307 marks a cell the model could not calculate. A map is up to 15000 runs of the model, so it counts
against its own limit of 6 maps per user and minute (429 beyond that); a server computes one map at a time, further ones
wait for their turn up to 30 seconds and answer 429 after that. Otherwise the errors are those of a prediction.

`GET /api/hf/sunspots?year=&month=` (scope `hf:read`) returns the 12-month smoothed sunspot number (R12) of a month,
the value for `ssn`: `ssn` (1 to 311), `source` (`observed` for the smoothed measurement, `predicted` for the forecast
of the months NOAA has not smoothed yet) and `reference`. The server loads the values from NOAA SWPC (see
`RADIOPATH_SUNSPOT_URL`); it answers 404 for a month without a value and while nothing is loaded. The P.533 model
saturates at a sunspot number of 160, higher values give the same prediction.

## Errors

Every error of the JSON API is an RFC 7807 problem (`application/problem+json`) with `title`, `status` and
`detail`. Invalid input answers 422 with one entry per field in `errors[]`, for example
`{"location": "body.lat", "message": "Latitude: must be between -90 and 90"}`; checks across fields use the
location `body`. 401 means no session or a bad token, 403 a token without the scope, 404 also covers objects of other users, 409 a conflict (a site still in
use, a taken name), 429 a rate limit (600 requests per user per minute over every operation that needs a login;
`GET /api/links/{id}`, the reports and `composite.png` share 60 per user per minute), 503 maintenance mode. The public account operations never tell whether a
name or an address exists: registration answers 204 either way and says what happened only in the mail to that
address. A failed calculation reports a fixed reason (`No terrain data for this area.`, `Calculation failed.`);
the underlying error is only in the log.

## Conventions

- JSON keys are snake_case; optional values are omitted rather than `null` where possible; empty lists are `[]`.
- The API is not versioned. It only changes in backwards-compatible ways: new operations and fields are added,
  existing ones are not renamed or removed.
