# Accounts, sharing and the admin panel

How logins, sessions, rate limits, share links and the admin panel work, and why they are built that way.

## Login and sessions

Everything except the login, registration and password reset pages, the tile proxy `/tiles/`, `/healthz`, `/readyz` and the
share links under `/s/` (see below) requires a login: the API answers 401
without a session, and the app sends the browser to `/login`, back to the page
afterwards. The login takes the user name or the confirmed address of the
account; an address counts against the same rate limits as the name it stands
for, and an unknown address fails exactly like an unknown name. Users
can register themselves unless `RADIOPATH_REGISTRATION=false`; names are 3 to
32 characters (letters, digits, `._-`) and unique ignoring case, passwords need
10 characters (at most 72). Registering asks for an email address and mails a
confirmation link (24 hours); the account cannot log in before it is opened
(and is deleted after 7 days if it never is). Until then the address waits
outside the unique index, so registering somebody else's address blocks
nothing. Registration answers the same whether the name or the address is
taken: the mail to that address says what happened (a confirmation link, "you
already have an account", or "the name is taken"), so only its owner learns it.
Logging in with the right password before confirming offers a button for a
fresh link (3 per account per hour). Under
the user name in the header, **Account** changes the address on its Profile tab
(the password is required, so a stolen session cannot redirect the reset mails;
again by confirmation link, the old address stays until then and gets a notice
afterwards) and on its Security tab the password (the old one is required, every
other session is logged out) or just logs out every other device; the API
tokens and the API reference have their own tabs. `/forgot` mails a reset link (one hour) and answers the same
whether or not the address is known. Mails go out in the background, so the
response time does not tell either, and an unknown name costs the same bcrypt
work at login as a known one. Links are random tokens stored hashed, like
sessions, and used once; they sit in the URL fragment (`/reset#<token>`), which
browsers never send, so they reach no access log, and the page removes them
from the address bar.
Sites, links, coverages and antenna patterns belong to the user
who created them; nobody else sees them except through a share link the owner
switched on, and deleting a user deletes their data with them. One account can hold 200 sites and 50 coverages and have 2 coverage
jobs queued or running at a time (constants in `internal/store/store.go`), so
an open registration cannot fill the database or block the workers. Accounts can also be
managed with the binary, see [Operations](../reference/operations.md#user-accounts-from-the-command-line).

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
so requests carry no tokens. Every response carries `X-Content-Type-Options:
nosniff`, `X-Frame-Options: DENY` and `Referrer-Policy: same-origin`. The app
and the admin panel add a `Content-Security-Policy` that allows scripts only
from the app itself plus their fixed inline scripts by hash (computed at start
from the build and the rendered admin page), no plugins, no framing and no
connections elsewhere; the admin panel allows no script files at all. TLS and
HSTS are the ingress's job.

Rate limits are counted in PostgreSQL, so they hold across replicas: 20 failed
logins per IP, 5 per user name from one IP and 100 per user name from all IPs
in 15 minutes (so a stranger cannot lock the owner out with a few wrong
passwords), 5 registration attempts per IP and 3 registration mails per address
per hour (so nobody can flood a mailbox from many IPs), 5 reset requests per IP
and 3 per address per hour, 3 confirmation mails per account per hour, 120
requests per shared object per hour, 60 path analyses, PDF reports and
raster merges per user per minute (every view of a link recomputes it, so one
account could otherwise keep the CPUs busy), and 600 requests per user per
minute over every API operation that needs a login, sessions and tokens
together (so a runaway script cannot hammer the database); further requests get HTTP 429. A made-up share token answers 404 without being
counted, so guessing leaves no rows behind. Behind an ingress set
`RADIOPATH_TRUST_PROXY=true` so the client IP is read from `X-Forwarded-For`
(last hop); without it every request would count against the ingress address.

## What stays internal

Public answers say as little about the installation as possible:

- The version only for logged-in users (footer, `/api/me`), besides the log and
  the metrics: not in `/api/config`, not in the API description.
- The API description (`/api/openapi.json`, `/api/schemas/`) needs a login or
  a token. The reference page in the app (Scalar) is bundled into the binary, with
  its telemetry, AI agent, proxy and web fonts switched off; the
  Content-Security-Policy would block any call to its services anyway.
- A failed link or coverage calculation shows a fixed reason; the error itself
  can name tile files, hosts or the storage bucket and goes to the log only.
  Coverage jobs do not report which worker (pod) computes them.
- `/readyz` answers `not ready` without saying which part failed.
- One user can hold at most five job event streams per process.

## API tokens

Programs such as a logbook use the API with a personal token instead of a
password. The design follows from one question: what can a stolen token do?

- **Only what it was given.** A token carries scopes per resource (read, or
  read and write) and reaches nothing else; account settings, passwords and the
  tokens themselves need a session, so a token can neither take over the account
  nor mint more tokens. A request with an `Authorization` header is judged by the
  token alone: a wrong token answers 401 and never falls back to a cookie.
- **Not forever, unless chosen.** Every token has an expiration picked when it is
  created; "never" is allowed but marked as such. Expired tokens stay listed for
  30 days, so the owner sees why a program stopped working.
- **Hard to get.** A token is 32 random bytes; the database keeps only its
  SHA-256, like sessions, and the secret is shown once. Creating one needs the
  account password, so a stolen session cannot leave a lasting token behind, and
  the confirmed address gets a mail about every new token.
- **Easy to notice and stop.** The token page shows when each token was last
  used (updated at most once a minute) and revokes it at once. Changing the
  password does not revoke tokens: programs keep working, and the list shows what
  is still active. Revoking is manual on purpose; the admin panel leaves tokens
  alone, and deleting a user deletes them.
- **Limited like a login.** 20 bad tokens per IP in 15 minutes give 429 before
  the database is asked, a valid token makes at most 300 requests per minute, and
  maintenance mode stops tokens like it stops logins (503).

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
secret: shared pages are sent `X-Robots-Tag: noindex`, their rasters and
reports also `Cache-Control: no-store`, but a URL pasted somewhere public is
public. The page and the API call behind it (`/api/share/links/<token>`,
`/api/share/coverages/<token>`) count against the same 120 requests per hour.

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
