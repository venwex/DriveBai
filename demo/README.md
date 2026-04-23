# DriveBai — Demo Package

Everything you need to present the DriveBai backend end-to-end.

Files in this folder:

- `DriveBai.postman_collection.json` — the whole demo as a runnable collection
- `DriveBai.postman_environment.json` — environment variables (tokens, IDs auto-populate)
- `README.md` — this file: demo scripts, feature audit, gap analysis

---

## 1. Setup checklist (do once, before defense)

1. **Backend running.** From the repo root:
   ```
   go run ./cmd/api
   ```
   Confirm: `curl http://localhost:8080/health` → `ok`.
2. **Database migrated** to version 14. Confirm: in psql,
   `SELECT version FROM schema_migrations;` → `14`.
3. **Postman:** File → Import → pick both JSON files in this folder.
4. Select environment **DriveBai Local** (top-right dropdown).
5. Leave `ownerEmail` / `driverEmail` empty — the collection generates unique demo emails on first run.
6. *(Optional)* For live Stripe payment demo, set `STRIPE_SECRET_KEY` and `STRIPE_WEBHOOK_SECRET` in the backend's `.env`. Without them, the payment-intent step returns 500 — skip it in the demo.

---

## 2. Feature audit table

| Promised feature | Code present | Route(s) | Verified by real request | Demo-ready | Notes |
|---|---|---|---|---|---|
| JWT access + refresh tokens | ✅ | `POST /auth/register`, `/login`, `/token/refresh` | ✅ | ✅ | 15 min access / 30 days refresh (configurable); rotation on refresh |
| Email verification | ⚠️ deprecated | `POST /auth/verify-email` | ✅ | Yes, but note | Handler is a stub — register already returns tokens; `verify-email` always returns 200 |
| Email OTP (passwordless) | ✅ | `POST /auth/otp/{request,verify,complete-registration}` | ✅ | ✅ | Dev mode prints code to console (no MailerSend key needed) |
| Roles: driver / car_owner / admin | ✅ | Enforced in register, active-profile | ✅ | ✅ | Admin role exists in DB but is reserved (register rejects it) |
| Multi-profile (driver↔owner on one account) | ✅ | `GET/POST /me/profiles`, `POST /me/active-profile` | ✅ | ✅ | Switching to driver mode requires both documents (409 otherwise) |
| Profile photo upload | ✅ | `POST /profile/photo` | ✅ | ✅ | 5 MB, image/jpeg or png |
| Driver document upload (DL, registration) | ✅ | `POST /documents/{type}` | ✅ | ✅ | Auto-advances `onboarding_status` when both docs present |
| Onboarding status tracking | ✅ | `GET /me`, `POST /onboarding/complete` | ✅ | ✅ | created → role_selected → photo_uploaded → documents_uploaded → complete |
| Car CRUD | ✅ | `/api/v1/cars` | ✅ | ✅ | Full CRUD + pause toggle |
| Car photos (slots) | ✅ | `/api/v1/cars/{id}/photos` | ✅ | ✅ | Slots: cover_front, right, left, back, dashboard. Uploading cover_front auto-publishes a pending car |
| Car documents | ✅ | `/api/v1/cars/{id}/documents` | ✅ | ✅ | inspection, registration, permit, insurance |
| Public listings browse (no auth) | ✅ | `GET /api/v1/listings` | ✅ | ✅ | Supports `search` and `status` query params |
| Like / unlike listings | ✅ | `POST/DELETE /listings/{id}/like`, `GET /me/likes` | ✅ | ✅ | Idempotent |
| Chat — list / create / get | ✅ | `/api/v1/chats` | ✅ | ✅ | Chat is triple (car, driver, owner); idempotent |
| Chat — messages | ✅ | `/chats/{id}/messages` | ✅ | ✅ | Text + system (lease auto-posts system message). **Fixed**: see §6 |
| Chat — read marker | ✅ | `POST /chats/{id}/read` | ✅ | ✅ | |
| Chat — attachments (file upload) | ✅ | `/chats/{id}/attachments` | ⚠️ partial | Yes, if file provided | Kind (image/document/video) auto-detected from MIME |
| Chat — archive / settings | ✅ | `/chats/{id}/archive`, `/settings` | ✅ | ✅ | |
| In-chat structured Request objects | ✅ | `/chats/{id}/requests` | Partially | Optional | Types: manual_payment, delayed_payment, mechanic_service, additional_fee, generic. Requires target & amount — demoable but not core |
| Lease request create | ✅ | `POST /listings/{id}/lease-requests` | ✅ | ✅ | Returns {chat_id, lease_request}; auto-creates chat + shares docs |
| Lease request accept / decline / cancel | ✅ | `POST /lease-requests/{id}/{accept,decline,cancel}` | ✅ | ✅ | Role guards: owner accepts/declines; creator cancels |
| Lease request — list in chat | ✅ | `GET /chats/{id}/lease-requests` | ✅ | ✅ | |
| Shared documents on lease | ✅ | `GET /chats/{id}/shared-documents` | ✅ | ✅ | Driver's onboarding docs auto-shared on create |
| WebSocket live events | ✅ | `GET /api/v1/ws?token=` | ✅ | ✅ | **Fixed**: see §6. Broadcasts lease_request_created/updated, new_message, request_created/updated |
| Payments — Stripe intent | ✅ | `POST /lease-requests/{id}/payments/intent` | ⚠️ needs keys | Depends | Returns 500 STRIPE_ERROR without `STRIPE_SECRET_KEY` |
| Payments — Stripe webhook | ✅ | `POST /stripe/webhook` | ❌ not demoable locally | No | Needs public URL + secret; show code only |
| Today tab actions | ✅ | `GET /today/actions`, `POST /today/actions/seen` | ✅ | ✅ | Filters by user's last_seen_actions_at |
| Password reset (forgot + reset) | ✅ | `POST /auth/password/{forgot,reset}` | ✅ | ✅ | Dev mode prints reset link to console |
| OpenAPI / Swagger UI | ✅ | `GET /openapi`, `GET /docs`, `GET /` | ✅ | ✅ | YAML at /openapi; Swagger UI at /docs (root `/` redirects) |
| Docker Compose local run | ⚠️ | `docker-compose.yml` | Not used in this demo | Mention only | Container was pointing at an empty DB during this session — running the binary directly against local Postgres works well |
| Database migrations 1–14 | ✅ | `migrations/*.sql` | ✅ (version=14) | ✅ | 21 tables live, all with cascades |

Legend: ✅ works · ⚠️ partial / needs config · ❌ not practical here

---

## 3. Routes actually mounted (ground truth from main.go)

- `GET /health`, `GET /`, `GET /openapi`, `GET /docs`
- `GET /api/v1/listings` (public)
- `GET /api/v1/ws?token=` (public; JWT via query)
- `POST /api/v1/stripe/webhook` (public; Stripe-signed)
- `POST /api/v1/auth/{register,login,verify-email,token/refresh,password/forgot,password/reset,logout,resend-otp}`
- `POST /api/v1/auth/otp/{request,verify,complete-registration}`
- **Protected (Bearer):**
  - `GET /me`, `PATCH /profile`
  - `GET/POST /me/profiles`, `POST /me/active-profile`
  - `POST /profile/photo`
  - `GET/POST /documents`, `POST /documents/{type}`, `DELETE /documents/{id}`
  - `POST /onboarding/complete`
  - `GET /me/actions`, `GET /today/actions`, `POST /today/actions/seen`
  - `GET /me/likes`, `POST/DELETE /listings/{id}/like`
  - `/cars` CRUD + `/pause` + `/location` + `/photos` + `/documents`
  - `/chats` list/create + `/{id}` get + `/messages` + `/read` + `/details` + `/settings` + `/archive` + `/requests` + `/attachments`
  - `GET /users/{id}/profile`
  - `POST /listings/{id}/lease-requests`
  - `GET /chats/{id}/lease-requests`
  - `GET /chats/{id}/shared-documents`
  - `POST /lease-requests/{id}/{accept,decline,cancel,payments/intent,payments/sync}`

---

## 4. Demo scenarios

### 4a. Primary — 7–10 minute "best-selling" run (recommended)

This is the ordered story: owner lists a car, driver browses and likes, they connect, chat, agree, and enter the structured request flow. Use the Postman collection in this order.

| # | Step | What to say | Proof on screen |
|---|------|-------------|-----------------|
| 1 | **01 · Health** | "The backend is a Go service — let me confirm it's up and the DB is reachable." | `200 ok` |
| 2 | **01 · Swagger UI** (browser at `http://localhost:8080/docs`) | "We ship an OpenAPI spec; here's the live Swagger UI generated from it." | Swagger UI loads |
| 3 | **02 · Register Owner** | "Two users today — Olga (owner) and Dmitry (driver). Registering issues a JWT pair immediately; no email step blocks the demo." | `201 access_token + refresh_token + user` |
| 4 | **02 · Register Driver** | (same, driver role) | `201` |
| 5 | **03 · Create Car** | "Olga lists her 2022 Camry. Price, location, deposit, insurance are all first-class fields." | `201 car` with `status: pending` |
| 6 | **03 · Upload Cover Photo** (pick any image) | "Uploading the cover photo automatically publishes the listing — this is how owners get a one-tap 'go live' experience." | `200`, then re-run `Get Car` → `status: available` |
| 7 | **05 · Browse Public Listings** (no auth) | "Now any driver — logged in or not — can see it. This is the public browse endpoint." | `listings` array contains the car |
| 8 | **06 · Like Listing** (as driver) → **Get My Likes** | "Dmitry favourites it." | `liked_listing_ids` contains the car |
| 9 | **11 · WebSocket tab** — connect as owner with `{{accessTokenOwner}}` | "Before the next step, I open a WebSocket subscription as Olga — real-time events." | `[connected]` |
| 10 | **07 · Create Lease Request** (as driver) | "Dmitry requests the car for 2 weeks. Notice: one call creates both the lease request AND the chat." | `{chat_id, lease_request}` — AND the WS tab instantly shows `lease_request_created` |
| 11 | **07 · Owner Sends Message** + **Driver Sends Message** + **List Messages** | "They chat inside the lease — every message is delivered by WebSocket in real time. The chat is tied to this lease request." | Message list with both messages + system message |
| 12 | **07 · Owner Accepts Lease Request** | "Olga accepts. Status goes requested → accepted. The driver is notified via WebSocket." | `status: accepted` — WS tab (if driver-connected) shows `lease_request_updated` |
| 13 | **08 · Today Actions** (as owner) | "The Today tab aggregates pending actions across the owner's listings — this is how the 'Today' screen in the iOS app is powered." | `actions` array |
| 14 | **Close** | "From here, Stripe Payment Intent is the last step — we have the code and schema for it, and in production with Stripe test keys it hands the driver a client_secret." | — |

### 4b. Backup — 3–4 minute "if time is short"

Cut the demo to this path. You show the most convincing parts only.

1. **Health + Swagger UI** (15 s)
2. **Register Owner + Register Driver** (30 s — rapid-fire)
3. **Create Car + Upload Cover Photo** (20 s — state change matters)
4. **Browse Public Listings** (10 s)
5. **Create Lease Request** → WebSocket shows the event live (60 s — this is the "wow")
6. **Send Chat Messages** (30 s)
7. **Owner Accepts** (15 s)
8. **Today Actions** (15 s)

Total: ~3 min 15 s. If you still have time, show the OpenAPI spec in a browser.

### 4c. Risk-managed — fallbacks for demo failures

| If this fails / looks bad | Fallback |
|---|---|
| **WebSocket doesn't connect in Postman.** | Have a terminal open with the test WS client pre-staged: `go run /tmp/ws-listen.go -token $TOKEN`. If that also fails, point to the `ws upgrade` log line in the backend and explain the fixed middleware. |
| **Stripe Payment Intent 500s.** | Expected. Say: *"Without Stripe test keys wired into local env, the Stripe client fails auth. The handler returns a structured `STRIPE_ERROR` — here's the code path."* Show `internal/handlers/lease_request.go` CreatePaymentIntent. Then skip. |
| **Email not delivered.** | Point at the server log: *"Dev mode prints the OTP / reset link in the console; in production the backend plugs into MailerSend and SendGrid — keys are empty locally on purpose so nothing leaves the laptop."* |
| **Admin role demo weak.** | Do NOT show admin. The role exists in the enum but is reserved — `register` rejects it. Mention it only as "reserved for an admin dashboard we haven't shipped yet." |
| **Upload fails in a demo network.** | All uploads are against `./uploads/` on disk. Pre-upload a file once before the demo — it'll work deterministically. |
| **Migration state inconsistent.** | Re-run once before demo: `migrate -path migrations -database "$DATABASE_URL" up`. Confirm v14. |

### 4d. Feature → demo mapping

| Promised feature | Appears at demo step | Proof artifact |
|---|---|---|
| JWT register/login/refresh | 3, 4 | Register response body with access_token/refresh_token |
| Roles (driver / car_owner) | 3, 4, 12 | `user.role` in responses; `accept` is owner-only |
| Driver onboarding documents | *(skipped in happy path — shown in 10 · Extras if asked)* | `POST /documents/drivers_license` response |
| Car CRUD + photos + auto-publish | 5, 6 | Car object status changes pending → available |
| Public listings | 7 | No-auth GET returns the just-created car |
| Favorites | 8 | `liked_listing_ids` shows the car |
| Chat + WebSocket live events | 9, 10, 11, 12 | WebSocket tab displays `lease_request_created`, `new_message`, `lease_request_updated` events |
| Lease requests accept/decline/cancel | 10, 12 | Status field transitions |
| Shared documents on lease request | 10 | `/chats/{id}/shared-documents` (note: empty unless driver pre-uploaded docs) |
| Today tab | 13 | `actions` array |
| OpenAPI / Swagger | 2 | Browser |

---

## 5. Honest gap analysis

### Fully working (demo live)
- Auth: classic register/login/refresh/logout, forgot+reset password (dev console), OTP request/verify/complete-registration
- Multi-profile account model with JWT rotation on mode switch
- User profile (/me, PATCH /profile), profile photo, driver documents, onboarding state machine
- Car CRUD, car photos (with auto-publish on cover upload), car documents, pause, location update
- Public listings browse with search & status filter
- Likes (like/unlike/list)
- Chat: create (by triple), list, messages (send/list), read marker, archive, settings, details
- Lease requests: create (auto-creates chat), list in chat, shared documents, accept, decline, cancel
- Today tab (actions + seen marker)
- WebSocket live events: lease_request_created, lease_request_updated, new_message, request_created/updated
- OpenAPI + Swagger UI served from `/openapi` and `/docs`

### Partially working / needs setup
- **Stripe payments**: handler and schema are complete; **requires real `STRIPE_SECRET_KEY`** in backend `.env` to actually call Stripe. Without keys, `POST /lease-requests/{id}/payments/intent` returns `500 STRIPE_ERROR`. The webhook is signature-verified and would need ngrok + the webhook secret.
- **Email send (SendGrid / MailerSend)**: emails fail silently when keys are absent (dev mode prints banners to the console). For the demo we rely on the console — in production we'd wire the keys.
- **Chat attachment upload**: works, but the response only populates `file_url`; the handler does not currently attach the file to a specific message (it creates an orphan attachment row). Usable for demoing upload, less so for attaching to an existing message.

### Exists in code but NOT demo-safe
- **`POST /auth/resend-otp`** and **`POST /auth/verify-email`** are deprecated stubs that always return 200. Do NOT showcase them; they will look wrong.
- **Admin role**: reserved but not exposed — registration rejects `role: "admin"`. Don't claim a working admin flow.
- **`GET /me/actions`** is legacy; use `GET /today/actions` instead.

### Promised in slides but not truly present
- **Deep push notifications** beyond WebSocket (no APNs/FCM integration in code).
- **Admin dashboard** — not in this backend (only DB enum values).
- **Mobile deep-link deep behavior** — `APP_DEEPLINK_SCHEME` only produces a URL string in the password-reset email banner; no device-side wiring is in this repo.

### Things that should NOT be shown live
- The commented-out migrate container in `docker-compose.yml` downloads the `migrate` binary at runtime — fragile for a demo.
- Stripe webhook from a local server (requires ngrok).
- Any manual DB seeding — use the happy-path flow instead.

---

## 6. Demo-critical fixes applied this session

Two bugs would have broken the demo. Both fixed, tested, and included in the current branch.

1. **`internal/handlers/chat.go` — `SendMessage`**: when clients omitted `client_message_id`, the handler inserted `uuid.Nil` into a NOT-NULL-DISTINCT unique index on `messages.client_message_id`. The *first* message per chat succeeded; every subsequent message returned `500 INTERNAL_ERROR`. Fixed by treating `uuid.Nil` as "no client ID" and inserting NULL, which the partial unique index correctly ignores.

2. **`internal/middleware/logging.go` — `responseWriter`**: our custom response-writer wrapper did not implement `http.Hijacker`, so the WebSocket upgrade at `GET /api/v1/ws` failed with `websocket: response does not implement http.Hijacker`. Added `Hijack()` and `Flush()` pass-throughs. WebSocket demos now work.

Both fixes were minimal and targeted — no behavior changes outside the bug surface.

---

## 7. Running the WebSocket demo in Postman

Postman Desktop has a dedicated "WebSocket Request" tool (File → New → WebSocket Request). Steps:

1. **URL:** `{{wsUrl}}?token={{accessTokenOwner}}` (or `{{accessTokenDriver}}`)
2. Click **Connect**. Status → "Connected".
3. Leave the tab open while you run the collection requests. Events appear in the Messages panel in real time.

Tip: open two WebSocket tabs side-by-side — one with owner token, one with driver token. Then any action from either user is visible to the other without polling.

---

## 8. Environment keys quick reference

Only these are strictly required to demo the happy path:

| Variable | Required for demo? | Notes |
|---|---|---|
| `DATABASE_URL` | ✅ | Must point to a Postgres with migrations 1–14 applied |
| `JWT_SECRET` | ✅ | Any non-empty string |
| `PORT` | (default `8080`) | — |
| `ENV` | (default `development`) | Controls logger format + dev-mode email printing |
| `UPLOAD_DIR` | (default `./uploads`) | Must be writable |
| `SENDGRID_API_KEY` | no | Leave empty → emails print to console |
| `MAILERSEND_API_KEY` | no | Leave empty → OTP prints to console |
| `STRIPE_SECRET_KEY` | only for Payments demo | Omit to skip Stripe steps |
| `STRIPE_WEBHOOK_SECRET` | only for webhook demo | — |
| `APP_DEEPLINK_SCHEME` | no | Default `drivebai` — cosmetic in reset email banner |
