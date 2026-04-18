# Snipe-IT API & Sync Flow

---

## Snipe-IT API

### Envelope behavior

- **POST / PATCH** responses are wrapped: `{ "status": "success", "messages": {}, "payload": { ... } }`
- **GET** responses are the object directly (no envelope).
- Always check `env.Status == "success"` after POST/PATCH — a 200 response can
  still carry `"status": "error"` with validation messages. This applies to
  `CreateLicense`, `CreateManufacturer`, `CheckoutSeat`, and any other mutating
  call. Decoding without this check will silently accept failed operations.

### Rate limiting

- Rate-limit all Snipe-IT calls via the `rateLimitMs` parameter passed to `snipeit.NewClient`.
  Default is 500ms (2 req/s); configurable via `sync.rate_limit_ms` in settings or
  `SNIPE_RATE_LIMIT_MS` env var. Reduce for local instances; increase if you encounter 429s.
- Apply the limiter in the shared HTTP helpers (`get`, `post`, `patch`, the delete
  helper), not at the method level, so it is always enforced regardless of which
  method is called.

### Checkout / checkin

- `CheckoutSeat` uses **PATCH** on `/api/v1/licenses/{id}/seats/{seatID}` — POST
  returns 405. Always use PATCH for seat assignment. The PATCH body must use
  `"assigned_to": <userID>` (integer) to assign the user.
- `CheckinSeat` uses **PATCH** on `/api/v1/licenses/{id}/seats/{seatID}` with
  body `{"assigned_to": null, "asset_id": null}`. Snipe-IT does **not** support
  DELETE on license seats — DELETE returns an error. Clearing the assignment via
  PATCH is the correct and only supported way to check a seat back in.
- `ListLicenseSeats` returns up to 500 rows (`?limit=500`). If a license could
  have more than 500 assigned users, pagination is required.

### Seat listing response — `assigned_user` vs `assigned_to`

The `GET /api/v1/licenses/{id}/seats` response uses **`assigned_user`** (not
`assigned_to`) as the JSON field name for the user object. The field is a nested
object (`{"id": N, "name": "...", "email": "..."}`) or `null` when the seat is
free. The Go struct tag must be `json:"assigned_user"`.

The PATCH checkout/checkin request body uses `"assigned_to"` (integer for assign,
null for release). These two field names are intentionally different — `assigned_user`
is the read field on GET, `assigned_to` is the write field on PATCH.

Do not confuse them — using `"assigned_to"` as the JSON tag on the struct will
silently decode all seats as unassigned (nil) on every sync, causing full
re-checkout of all seats on every run.

### POST response returns free_seats_count: 0

Snipe-IT's `POST /api/v1/licenses` (create) response returns `free_seats_count: 0`
regardless of the seat count. Do **not** use `lic.FreeSeatsCount` from a create
response to drive ghost detection — ghost cleanup will compute `ghostCount = seats - 0 = N`
and drain the entire free seat pool before the checkout loop runs.

Always refresh the license with `FindLicenseByID` (a GET) after create or expand
before reading `FreeSeatsCount`. See step 7.5 in the standard sync flow below.

### Ghost checkout cleanup

"Ghost" checkouts are seats Snipe-IT internally counts as used (tracked via
`lic.Seats - lic.FreeSeatsCount`) but whose `assigned_user` is null in the seat
listing. This can happen when seats were checked out via the Snipe-IT UI and the
user was later removed through a mechanism that didn't properly check the seat in.

Detect and clean up ghosts after loading seat state:

```go
snipeCheckedOut := lic.Seats - lic.FreeSeatsCount
ghostCount := snipeCheckedOut - len(checkedOutByEmail)
if ghostCount > 0 {
    // clean up ghostCount seats from freeSeats using CheckinSeat
}
```

The PATCH checkin (`{"assigned_to": null, "asset_id": null}`) correctly resolves
the ghost state by clearing Snipe-IT's internal assignment record.

### FindOrCreate pattern

- `FindOrCreate*` methods first search by exact name, then create if not found.
- Search endpoints may return fuzzy/partial matches — always verify with
  `strings.EqualFold` before treating a row as the match.
- `license_category_id` is required by Snipe-IT when creating a license. Validate
  it is non-zero before any API calls and fail with a clear error message.

### User creation (`--create-users`)

When a source user has no matching Snipe-IT account, the `--create-users` flag
triggers `CreateUser` instead of warning and skipping. Created users are populated
from source system data and configured to be inert within Snipe-IT:

- `activated: false` — user cannot log into Snipe-IT
- `send_welcome: false` — no welcome email is sent
- No group membership — avoids any auto-assign license groups
- `start_date` — set to the account creation date in the source system (when
  available), formatted as `YYYY-MM-DD`
- `notes` — documents the auto-creation source (e.g. "Auto-created from
  `<source system>` via `<integration name>`")
- `password` — cryptographically random, unusable in practice since `activated`
  is false; required by the Snipe-IT API

`CreateUser` POSTs to `/api/v1/users`. The response is wrapped in the standard
envelope (`status`, `messages`, `payload`). Always check `env.Status == "success"`
before unmarshaling the payload.

The `Result.UsersCreated` counter tracks how many accounts were created. In
dry-run, creation is simulated (logged as `[dry-run] would create Snipe-IT user`)
and the counter is still incremented so output is meaningful.

Unmatched users whose creation fails are counted as `Warnings` (not added to
`UnmatchedEmails`) — the Slack notification path for unmatched emails is bypassed
since the failure is already logged.

Source system data needed for `CreateUser` (display name, account creation date)
typically requires an additional API scope or endpoint beyond what the basic sync
needs. Gate the extra API call behind the `--create-users` flag rather than
fetching it unconditionally — see the integration's `CONTEXT.md` for specifics.

### Seat state partitioning

Partition the slice from `ListLicenseSeats` into two structures:
- `checkedOutByEmail map[string]*LicenseSeat` — key is `strings.ToLower(seat.AssignedTo.Email)`
- `freeSeats []*LicenseSeat` — seats with nil or empty `AssignedTo`

After partitioning, run the ghost cleanup pass (see above) before the checkout loop.

If a checkout call fails, return the seat to `freeSeats` before continuing.
Seats are never shrunk automatically — only ever expanded.

---

## Standard sync flow

The sync runs in two passes. The pattern below is vendor-agnostic; vendor-specific
steps (role fetching, manufacturer resolution, etc.) are described in `CONTEXT.md`.

**Checkout/update pass** (runs for all users, or one user with `--email`):

1. Fetch active users from the source system (paginated).
2. Build an active email set for use in the checkin pass.
3. Apply `--email` filter if set.
4. Fetch per-user metadata from the source system (roles, groups, etc.).
5. Resolve any Snipe-IT entities needed before license creation (manufacturer, etc.).
6. Resolve target seat count: vendor API → `snipe_it.license_seats` config → active member count (floor).
   Find or create the Snipe-IT license (dry-run: find only; synthesize if absent).
7. Expand license seat count if `targetSeats > license.Seats`.
7.5. **Refresh the license with `FindLicenseByID`** — the POST response has `free_seats_count: 0`;
   the GET response has the real value. Required before ghost detection.
8. Load current seat assignments; partition into `checkedOutByEmail` and `freeSeats`.
   Run ghost cleanup before the checkout loop.
9. For each active user:
   - Look up the Snipe-IT user by email.
   - If not found and `--create-users` is set: call `CreateUser` (see User creation
     section above); increment `UsersCreated`; continue to checkout. In dry-run,
     log the would-be creation and count `CheckedOut`; skip the real checkout.
   - If not found and `--create-users` is not set: warn + skip + append to
     `result.UnmatchedEmails`.
   - If already checked out: compare notes; update if changed or `--force`.
   - If not checked out: pop a free seat and call `CheckoutSeat`.

**Checkin pass** (skipped when `--email` filter is active):

10. For each seat whose assigned email is not in the active set: call `CheckinSeat`.

### Result struct

```go
type Result struct {
    CheckedOut      int      // seats newly assigned
    NotesUpdated    int      // seats whose notes were updated
    CheckedIn       int      // seats returned for inactive users
    Skipped         int      // users already up to date
    Warnings        int      // users with no matching Snipe-IT account, or API errors
    UsersCreated    int      // new Snipe-IT users created (--create-users)
    UnmatchedEmails []string // source users with no corresponding Snipe-IT account
}
```

### User matching

Match source system users to Snipe-IT users by **lowercased email address**.
Prefer the primary email field; fall back to the login/username field. All
comparisons must use `strings.ToLower` — never compare email addresses
case-sensitively.

### Notes field

Write per-user metadata from the source system into the seat's `notes` field.
Sort multi-value fields (e.g. role labels) alphabetically. Use an empty string
if there is nothing to record. Compare notes before patching — skip unchanged
seats (unless `--force`).

---

## Slack notifications (internal/slack/client.go)

- Minimal stdlib-only webhook client (`net/http`). No external dependencies.
- `Send(ctx, text)` is a no-op if `webhook_url` is empty — the binary works
  without any Slack configuration.
- Slack errors are logged as warnings via `slog.Warn`; they never fail the
  sync command.
- **Keep the sync package free of Slack dependencies.** Collect unmatched user
  emails in `Result.UnmatchedEmails` during the sync; send notifications from
  `cmd/sync.go` after `syncer.Run()` returns.
- Three events trigger notifications, all suppressed in dry-run:
  1. **Sync failure** — send after logging the error, before returning it.
  2. **Unmatched users** — one message per email in `result.UnmatchedEmails`.
  3. **Sync success** — send after printing the result summary line.
- Configuration: `slack.webhook_url` in `settings.yaml`, overridable via
  `SLACK_WEBHOOK` env var (bind in `cmd/root.go` `initConfig()`).
