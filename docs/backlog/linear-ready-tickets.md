# Tag Me — Linear Ready Tickets

## Product
Tag Me

## Scope
MVP Web App + Core API + Notification Worker

## Suggested Epic Structure
1. Scanner Messaging Flow
2. Owner Authentication & Dashboard
3. QR/Object Management
4. Notification Delivery
5. Abuse Prevention & Reliability

---

# EPIC 1 — Scanner Messaging Flow

## Issue 1
**Title:** [WEB][Scanner] QR landing page with predefined and custom message input  
**Priority:** High  
**Estimate:** M  
**Labels:** web, scanner, mvp

### Why
Scanner needs a fast, frictionless way to notify the owner without login.

### Scope
- Build QR landing page
- Show short explanatory copy
- Show predefined message options
- Allow optional custom message input
- Provide primary CTA: Notify Owner
- Validate required message selection/input rules

### Out of Scope
- Real-time chat UI
- Scanner login
- Rich media attachments

### Acceptance Criteria
1. Scanner can open a landing page from a QR link.
2. Landing page shows concise explanation of the product purpose.
3. Landing page shows predefined message choices.
4. Scanner can optionally enter custom message content.
5. Scanner can submit a valid notification request from this page.
6. Invalid or empty submission states are handled clearly in UI.
7. Page is usable on mobile viewport.

### Edge Cases
- Invalid QR token
- Expired or disabled QR token
- Empty custom input
- Long custom input
- Network failure during submit

### Unit Test Checklist
- validate selected predefined message
- validate optional custom message length and sanitization rules
- validate submission payload builder
- validate invalid QR state mapping
- validate CTA disabled/enabled state logic

### Integration Test Checklist
- landing page loads with valid QR token
- invalid QR token shows fallback/error state
- successful submit transitions to sent screen
- failed submit shows retry-friendly error state

### Definition of Done
- ACs implemented
- mobile responsive
- tests updated
- ready for one focused PR

---

## Issue 2
**Title:** [API][Scanner] Create anonymous message endpoint and conversation record  
**Priority:** High  
**Estimate:** M  
**Labels:** api, scanner, backend, mvp

### Why
Scanner notification must create a durable conversation and message event that can be tracked.

### Scope
- Implement `POST /messages`
- Accept qr_token, message_type, optional content, optional location
- Resolve QR token to owner/object context
- Create conversation record
- Create initial message record
- Return conversation identifier for status tracking
- Apply session-aware handling hooks

### Out of Scope
- Real-time delivery
- Scanner account model
- Complex conversation threading

### Acceptance Criteria
1. `POST /messages` accepts the documented payload shape.
2. Valid request creates a conversation and initial message.
3. Invalid QR token returns a safe error response.
4. Response includes enough information for scanner status tracking.
5. Request metadata needed for session/rate-limit logic is captured.
6. Sensitive owner contact data is never exposed in response.

### Edge Cases
- duplicate message attempts in same session
- missing message_type
- invalid QR token
- optional location absent
- malformed content payload

### Unit Test Checklist
- request payload validation
- QR token resolution logic
- conversation creation service
- initial status assignment logic
- response serializer excludes owner contact data

### Integration Test Checklist
- valid request persists conversation + message
- invalid request rejected with correct status
- returned response usable by status endpoint flow

### Definition of Done
- endpoint implemented
- DB persistence verified
- tests cover validation and creation flow

### New Updates
**Schema changes (2026-04-19):**
- QR token resolution (`FindByToken`) must JOIN `objects` table — `object_type` no longer on `qr_codes`
- Set `expires_at = NOW() + interval '24 hours'` on conversation creation
- Unit test: `expires_at` set correctly at creation

---

## Issue 3
**Title:** [API][Scanner] Conversation status endpoint for scanner polling  
**Priority:** High  
**Estimate:** S  
**Labels:** api, scanner, backend, mvp

### Why
Scanner needs status feedback after submitting a message.

### Scope
- Implement `GET /conversations/:id/status`
- Return scanner-safe status lifecycle
- Support states:
  - PENDING
  - DELIVERED
  - OPENED
  - ON_THE_WAY
  - RESOLVED
- Ensure anonymous scanner can only access safe conversation status view

### Out of Scope
- Full conversation thread UI
- push notifications to scanner

### Acceptance Criteria
1. Scanner can request conversation status using returned conversation ID.
2. Endpoint returns one of the allowed status states.
3. Response excludes owner private information.
4. Invalid or unknown conversation ID is handled safely.
5. Status values map correctly from owner actions.

### Edge Cases
- unknown conversation ID
- conversation exists but not yet delivered
- conversation resolved before scanner refresh
- repeated polling requests

### Unit Test Checklist
- status mapping logic
- access-safe response serializer
- invalid conversation handling
- state transition read logic

### Integration Test Checklist
- scanner sees initial pending/delivered state
- scanner sees updated state after owner action
- unknown conversation path returns safe error

### Definition of Done
- status endpoint works end-to-end
- no owner contact leakage
- tests updated

### New Updates
**Schema changes (2026-04-19):**
- `EXPIRED` added as valid status — supported by DB CHECK constraint
- Lazy expiry: check `expires_at < NOW()` on each read, UPDATE to EXPIRED in same transaction before returning
- New edge case: conversation expires between polls
- New unit tests: EXPIRED state returned when expires_at passed, lazy expiry UPDATE logic

---

## Issue 4
**Title:** [WEB][Scanner] Message sent and conversation status screens  
**Priority:** High  
**Estimate:** M  
**Labels:** web, scanner, mvp

### Why
Scanner needs clear feedback after notifying the owner.

### Scope
- Build message sent screen
- Build conversation status screen
- Show state-specific messaging:
  - Waiting
  - No response
  - Owner responded
- Poll status endpoint
- Render reminder CTA if eligible

### Out of Scope
- rich chat
- live socket updates

### Acceptance Criteria
1. After successful submit, scanner sees confirmation screen.
2. Scanner can navigate to conversation status screen.
3. Status screen reflects backend status correctly.
4. UI copy changes appropriately per state.
5. Reminder CTA only appears when allowed.
6. Loading and error states are handled.

### Edge Cases
- slow polling response
- transient backend error
- stale conversation ID
- status updated while page open

### Unit Test Checklist
- status-to-UI mapping
- polling state reducer/store
- reminder CTA visibility rule
- error state handling

### Integration Test Checklist
- successful submit leads to sent screen
- status screen polls and renders updated state
- error state shown on failed polling

### Definition of Done
- screens implemented
- status logic tested
- mobile-first UX verified

### New Updates
**Schema changes (2026-04-19):**
- `EXPIRED` is now a distinct UI state: copy "No one responded in time" → routes to fallback screen (Issue 6 / TAG-11)
- State mapping: Waiting (PENDING/DELIVERED), Owner responded (OPENED/ON_THE_WAY/RESOLVED), Expired (EXPIRED)
- Unit test must cover EXPIRED state-to-UI mapping

---

## Issue 5
**Title:** [API][Scanner] Reminder endpoint behavior with cooldown and session rules  
**Priority:** Medium  
**Estimate:** M  
**Labels:** api, scanner, anti-abuse, backend

### Why
Scanner needs a controlled way to send follow-up reminders without enabling spam.

### Scope
- Define reminder action behavior
- Enforce rate limiting and cooldown rules
- Associate reminder eligibility with session + conversation state
- Trigger follow-up notification job when allowed
- [Done] Fixed cooldown between reminder sends (e.g., 2 min minimum between reminders)

### Out of Scope
- unlimited reminders
- scanner identity system
- exponential backoff reminder cooldown (deferred → TAG-17.1; delivery retry backoff done separately in TAG-13)

### Acceptance Criteria
1. Scanner can request a reminder only for eligible conversations.
2. Reminder is blocked during cooldown window.
3. Reminder count is limited per session/conversation rules.
4. Ineligible reminder attempts return a clear response.
5. Successful reminder triggers notification workflow.
6. Reminder is blocked if conversation is EXPIRED or expires_at < NOW().

### Edge Cases
- multiple rapid reminder clicks
- reminder after resolved state
- reminder after max attempts reached
- Redis/session unavailable

### Unit Test Checklist
- cooldown logic
- max reminder count logic
- eligible status validation
- response mapping for blocked reminder

### Integration Test Checklist
- allowed reminder creates follow-up notification job
- blocked reminder returns correct response
- repeated requests enforce cooldown

### Definition of Done
- reminder logic implemented
- abuse controls verified
- tests updated

### New Updates

**Fix Cooldown Rule (2026-04-16):**
- Fixed cooldown: 2 min flat between reminder sends — ✅ IMPLEMENTED (`apps/api/internal/services/reminder_service.go`, `Cooldown: 2 * time.Minute`)
- Exponential backoff for repeated reminders: ❌ NOT implemented — cooldown is flat, not count-based. Deferred to TAG-17.1.


**Schema changes (2026-04-19):**
- Block reminder if conversation status is `EXPIRED` or `expires_at < NOW()`
- New edge case: reminder attempt on EXPIRED conversation
- New unit test: EXPIRED/expired conversation blocks reminder with correct response

---

---

## Issue 6
**Title:** [WEB][Scanner] Follow-up and fallback guidance screens  
**Priority:** Medium  
**Estimate:** S  
**Labels:** web, scanner, ux

### Why
When owner does not respond, scanner needs clear next-step guidance.

### Scope
- Build follow-up screen
- Build fallback screen — triggered by `EXPIRED` status from backend (explicit signal, not client-side threshold)
- Show reminder state
- Show fallback guidance like contacting security
- EXPIRED status is the primary trigger for fallback screen

### Out of Scope
- escalation integrations
- emergency service integrations

### Acceptance Criteria
1. Follow-up screen appears when scanner has already sent a notification.
2. Fallback screen appears when conversation status is EXPIRED.
3. Reminder CTA and fallback guidance are contextually correct.
4. Copy is concise and action-oriented.
5. Screen states are mobile friendly.

### Edge Cases
- status changes while on fallback screen
- reminder becomes available/unavailable mid-session
- invalid conversation context

### Unit Test Checklist
- fallback threshold mapping
- follow-up state mapping
- CTA visibility rules

### Integration Test Checklist
- no-response journey reaches fallback state
- follow-up path renders correct reminder state

### Definition of Done
- both screens implemented
- UX states verified
- tests updated

### New Updates
**Schema changes (2026-04-19):**
- `EXPIRED` added as explicit backend status — fallback screen now triggers on `EXPIRED` signal, not client-side timeout
- Scope updated: fallback screen is driven by `status === 'EXPIRED'` from polling response
- AC #2 updated: "Fallback screen appears when conversation status is EXPIRED"
- New unit test: EXPIRED state maps to fallback screen

---

# EPIC 2 — Owner Authentication & Dashboard

## Issue 7
**Title:** [API][Owner] OTP request and verification flow  
**Priority:** High  
**Estimate:** M  
**Labels:** api, owner, auth, backend, mvp

### Why
Owners need secure friction-light authentication.

### Scope
- Implement `POST /auth/request-otp`
- Implement `POST /auth/verify-otp`
- Create or retrieve owner record in `owners` table (contact, contact_type, dnd_enabled)
- Store OTP in Redis: key `otp:{contact}`, TTL 5 min; track attempts with `otp_attempts:{contact}`
- Return signed JWT on successful verify (stateless — no DB session table)
- Handle invalid/expired OTP safely

### Out of Scope
- password auth
- social login
- MFA beyond OTP

### Acceptance Criteria
1. Owner can request an OTP using supported contact method.
2. Owner can verify a valid OTP.
3. Invalid OTP is rejected with clear error.
4. Expired OTP is rejected safely.
5. Successful verify creates authenticated owner session.
6. OTP attempts are abuse-protected.

### Edge Cases
- repeated OTP requests
- expired OTP
- wrong OTP
- resend flow
- rate-limited auth requests

### Unit Test Checklist
- OTP generation policy
- OTP validation logic
- expiry check
- auth session creation
- attempt limit logic

### Integration Test Checklist
- request OTP then verify successfully
- invalid OTP fails
- expired OTP fails
- repeated attempts respect limit rules

### Definition of Done
- OTP flow works end-to-end
- abuse protections included
- tests updated

### New Updates
**Schema changes (2026-04-19):**
- `owners` table now defined: `id`, `contact`, `contact_type` (phone/email), `dnd_enabled`
- Scope updated: upsert owner on verify — create if not exists, retrieve if already registered
- Redis key pattern: `otp:{contact}` (TTL 5 min), attempt tracking: `otp_attempts:{contact}`
- Auth is stateless — JWT only, no DB session table needed

---

## Issue 8
**Title:** [WEB][Owner] Login screen with OTP flow  
**Priority:** High  
**Estimate:** S  
**Labels:** web, owner, auth, mvp

### Why
Owners need a simple UI to access their dashboard.

### Scope
- Build owner login screen
- Request OTP form
- Verify OTP form
- Error, loading, resend states
- Redirect to dashboard on success

### Out of Scope
- multi-step onboarding
- account profile setup

### Acceptance Criteria
1. Owner can request OTP from login screen.
2. Owner can enter and verify OTP.
3. UI handles invalid/expired OTP clearly.
4. Successful login redirects to dashboard.
5. Form states are accessible and mobile-friendly.

### Edge Cases
- resend timing
- double-submit
- network failure during auth
- expired OTP

### Unit Test Checklist
- form validation
- auth state transitions
- resend timing UI logic
- redirect-on-success logic

### Integration Test Checklist
- successful login flow to dashboard
- invalid OTP shows error
- resend flow works correctly

### Definition of Done
- owner login UI implemented
- auth UX tested
- dashboard redirect confirmed

---

## Issue 9
**Title:** [API][Owner] Conversation list and detail endpoints  
**Priority:** High  
**Estimate:** M  
**Labels:** api, owner, conversations, backend

### Why
Owner dashboard needs alert listing and detail views.

### Scope
- Implement `GET /conversations`
- Implement `GET /conversations/:id`
- Before returning list: run bulk lazy expiry UPDATE for this owner (mark `expires_at < NOW()` as EXPIRED)
- Return owner-scoped conversations only
- Support status filter: active (`NOT IN ('RESOLVED','EXPIRED') AND expires_at > NOW()`) and closed (RESOLVED or EXPIRED)
- Include in response: `expires_at`, `opened_at`, `on_the_way_at`, `resolved_at`, message content, timestamps, optional location

### Out of Scope
- advanced filtering
- analytics dashboards

### Acceptance Criteria
1. Authenticated owner can fetch their conversation list.
2. Authenticated owner can fetch conversation detail.
3. Only owner-owned conversations are returned.
4. EXPIRED conversations appear correctly with EXPIRED status in list.
5. Active and closed (RESOLVED/EXPIRED) conversation views are available.
6. Status timestamps included in detail response.
7. Bulk lazy expiry runs before list — owner always sees up-to-date EXPIRED state.

### Edge Cases
- owner with no conversations
- unauthorized access
- conversation belongs to different owner
- optional location absent

### Unit Test Checklist
- owner scoping logic
- conversation serializer
- detail lookup authorization
- active vs resolved mapping

### Integration Test Checklist
- authenticated owner sees only own conversations
- unauthorized requests rejected
- detail endpoint returns expected payload

### Definition of Done
- list/detail endpoints implemented
- authorization verified
- tests updated

### New Updates
**Schema changes (2026-04-19):**
- `EXPIRED` added as valid status in DB CHECK constraint
- New columns: `expires_at`, `opened_at`, `on_the_way_at`, `resolved_at` — all must be included in detail response
- Bulk lazy expiry pattern: `UPDATE conversations SET status='EXPIRED' WHERE owner_id=? AND expires_at < NOW() AND status NOT IN ('RESOLVED','EXPIRED')` before returning list
- Active filter: `status NOT IN ('RESOLVED','EXPIRED') AND expires_at > NOW()`
- Closed filter: `status IN ('RESOLVED','EXPIRED')`
- Partial index added: `idx_conversations_expiry_check` — query must use it

---

## Issue 10
**Title:** [WEB][Owner] Dashboard alert list and conversation detail with quick actions  
**Priority:** High  
**Estimate:** M  
**Labels:** web, owner, dashboard, mvp

### Why
Owner must be able to quickly inspect incoming alerts and respond.

### Scope
- Build dashboard with conversation list
- Build conversation detail screen
- Show status, timestamp, message, optional location
- Display EXPIRED as distinct status badge (not same as RESOLVED)
- Active tab: `status NOT IN ('RESOLVED','EXPIRED') AND expires_at > NOW()`
- Closed tab: RESOLVED + EXPIRED grouped
- Expose quick actions (hidden for EXPIRED/RESOLVED):
  - Seen
  - On the way
  - Resolved

### Out of Scope
- advanced sorting
- inbox search
- threaded owner notes

### Acceptance Criteria
1. Logged-in owner can see alert list.
2. Owner can open a conversation detail view.
3. Detail shows scanner message and timestamp.
4. Quick action buttons are visible and usable.
5. UI reflects current status correctly.
6. Active and resolved alerts are visually distinguishable.

### Edge Cases
- no alerts
- stale alert status
- failed quick-action update
- optional location missing

### Unit Test Checklist
- list rendering state mapping
- detail view model mapping
- quick-action button state logic
- empty state rendering

### Integration Test Checklist
- dashboard loads conversations from API
- detail view loads correct conversation
- quick action updates status in UI after success

### Definition of Done
- dashboard and detail flow implemented
- key owner journeys covered
- tests updated

### New Updates
**Schema changes (2026-04-19):**
- `EXPIRED` is a distinct status — render separate badge, not grouped with RESOLVED
- Active tab filter: `status NOT IN ('RESOLVED','EXPIRED') AND expires_at > NOW()`
- Closed tab: RESOLVED + EXPIRED grouped
- Quick actions (Seen, On the Way, Resolved) must be hidden for EXPIRED and RESOLVED conversations
- New unit test: EXPIRED conversations show badge, no quick actions

---

## Issue 11
**Title:** [API][Owner] Conversation status update endpoint  
**Priority:** High  
**Estimate:** S  
**Labels:** api, owner, backend, workflow

### Why
Owner actions must update conversation lifecycle and feed back to scanner.

### Scope
- Implement `PATCH /conversations/:id/status`
- Block update if conversation is EXPIRED — return 422
- Enforce valid forward-only transition map:
  - DELIVERED → OPENED (set `opened_at`)
  - OPENED → ON_THE_WAY (set `on_the_way_at`)
  - ON_THE_WAY → RESOLVED (set `resolved_at`)
- Set timestamp column on each transition
- Validate owner owns the conversation

### Out of Scope
- arbitrary custom status values
- editing scanner messages

### Acceptance Criteria
1. Authenticated owner can update status on own conversation.
2. Invalid status transitions are rejected safely.
3. Updated status is persisted.
4. Updated status is visible via scanner polling endpoint.
5. Unauthorized owner cannot update another owner’s conversation.
6. EXPIRED conversation cannot be updated — returns 422.
7. `opened_at`, `on_the_way_at`, `resolved_at` timestamps set on respective transitions.

### Edge Cases
- repeated same status update
- resolved conversation re-open attempt
- update attempt on EXPIRED conversation
- unauthorized update
- invalid status value

### Unit Test Checklist
- allowed transition logic
- invalid transition handling
- owner authorization check
- persistence and timestamp updates

### Integration Test Checklist
- owner updates status successfully
- scanner polling reflects updated status
- invalid status transition rejected

### Definition of Done
- endpoint implemented
- transition rules enforced
- tests updated

### New Updates
**Schema changes (2026-04-19):**
- Block PATCH if `status = 'EXPIRED'` — return 422
- Timestamp columns added: `opened_at`, `on_the_way_at`, `resolved_at` — set on each respective transition
- Valid forward-only transitions: DELIVERED→OPENED (set `opened_at`), OPENED→ON_THE_WAY (set `on_the_way_at`), ON_THE_WAY→RESOLVED (set `resolved_at`)
- New edge case: update attempt on EXPIRED conversation → 422
- New unit test: EXPIRED conversation blocked, timestamp columns set on each transition

---

## Issue 12
**Title:** [WEB][Owner] Settings for contact method and DND toggle  
**Priority:** Medium  
**Estimate:** S  
**Labels:** web, owner, settings

### Why
Owner needs basic notification preferences.

### Scope
- Build settings screen
- Select contact method
- Toggle DND preference
- Persist settings via backend

### Out of Scope
- advanced scheduling
- granular DND windows
- multi-channel preferences matrix

### Acceptance Criteria
1. Owner can view current notification settings.
2. Owner can update contact method.
3. Owner can toggle DND mode.
4. Settings persist after refresh.
5. UI explains the effect of DND clearly.

### Edge Cases
- invalid settings payload
- failed save
- DND active while conversation arrives

### Unit Test Checklist
- settings form validation
- DND toggle state logic
- settings persistence state mapping

### Integration Test Checklist
- settings load and save flow works
- failed save shows recoverable error state

### Definition of Done
- settings screen implemented
- persistence verified
- tests updated

---

# EPIC 3 — QR/Object Management

## Issue 13
**Title:** [API][Owner] Object CRUD endpoints  
**Priority:** High  
**Estimate:** M  
**Labels:** api, owner, objects, backend, mvp

### Why
Owner needs to register a vehicle/object and manage QR-linked assets.

### Scope
- Implement:
  - `GET /objects`
  - `POST /objects`
  - `GET /objects/:id`
  - `DELETE /objects/:id`
- Scope objects to authenticated owner
- Store metadata required for QR linkage

### Out of Scope
- editing object fields beyond MVP if not defined
- bulk import

### Acceptance Criteria
1. Owner can create an object.
2. Owner can list their objects.
3. Owner can view object detail.
4. Owner can delete an object they own.
5. Owner cannot access another owner’s object.
6. Object records can be used for QR generation flow.

### Edge Cases
- duplicate object naming rules
- deleting object with active conversations
- unauthorized access
- empty object list

### Unit Test Checklist
- object validation
- owner scoping logic
- create/delete behavior
- object lookup authorization

### Integration Test Checklist
- owner CRUD flow works end-to-end
- unauthorized access blocked
- delete behavior matches business rule

### Definition of Done
- CRUD endpoints implemented
- authorization verified
- tests updated

### New Updates
**Schema changes (2026-04-19):**
- `objects` table added: `id`, `owner_id` (FK→owners), `name`, `object_type` (car/motorcycle/bag/other), `plate` (nullable, unique)
- `object_type` moved from `qr_codes` to `objects` — `FindByToken` query must JOIN `objects`
- `DELETE /objects/:id`: blocked if any conversation with this object's QR code is not RESOLVED or EXPIRED — return 409
- App-level check before delete: query `conversations` for active status; `ON DELETE RESTRICT` on qr_codes.object_id enforces at DB layer too
- New edge case: delete with active conversations → 409 (already in spec)

---

## Issue 14
**Title:** [WEB][Owner] QR management screen with QR generate and download flow  
**Priority:** High  
**Estimate:** M  
**Labels:** web, owner, qr, mvp

### Why
Owner needs to generate and use printable QR codes.

### Scope
- Build QR management screen
- Show object list
- Generate QR for object
- Download QR asset
- Show empty and success states

### Out of Scope
- batch QR generation
- print customization
- branded templates

### Acceptance Criteria
1. Owner can view a list of registered objects.
2. Owner can generate a QR code for an object.
3. Owner can download the generated QR code.
4. UI clearly associates QR with the correct object.
5. Empty state guides owner to create an object first.

### Edge Cases
- no objects available
- QR generation failure
- download failure
- duplicate generation request

### Unit Test Checklist
- QR screen state mapping
- object-to-QR association logic
- download action state handling

### Integration Test Checklist
- object list loads into QR management screen
- QR generation works for valid object
- download action succeeds or fails gracefully

### Definition of Done
- QR management implemented
- generation/download flow tested
- owner can complete MVP QR workflow

---

# EPIC 4 — Notification Delivery (Async + Event-Ready)

---

## Issue 15
**Title:** [WORKER] BullMQ notification pipeline for conversation events  
**Priority:** High  
**Estimate:** M  
**Labels:** worker, notifications, node, bullmq, mvp

### Why
Notification delivery must be **asynchronous, reliable, and extensible** to support:
- message alerts
- reminder system
- future chat replies
- future multi-channel delivery

---

### Scope
- Implement BullMQ-based worker system 
- Repo: tag-me-worker
- Queue name: `notification`
- Handle job type:
  - `send_notification`

- Support event types:
  - `new_message`
  - `reminder`
  - (future) `owner_reply`

- Define standardized job payload:

```json
{
  "type": "new_message | reminder",
  "conversation_id": "uuid",
  "owner_contact": "string",
  "metadata": {}
}
```

- Simulate notification delivery (log-based for MVP)
- Integrate retry mechanism (BullMQ native)
- Structure worker for future provider integration (WA/email)

---

### Out of Scope
- Real provider integration (WhatsApp/email)
- Multi-provider routing
- Realtime socket delivery
- Advanced event bus (RabbitMQ)

---

### Acceptance Criteria
1. API enqueues job when:
   - new message created
   - reminder triggered

2. Worker consumes job from `notification` queue.

3. Worker processes based on `type`:
   - `new_message` → send alert
   - `reminder` → send follow-up alert

4. Worker logs structured output:

```
event=notification_sent type=new_message conversation_id=...
```

5. Worker supports retry:
   - attempts: 3
   - exponential backoff

6. Failed jobs are logged clearly (DLQ-style via failed jobs).

7. Worker is modular:
   - queue layer
   - processor layer
   - runner

---

### Edge Cases
- malformed payload
- missing contact info
- duplicate job enqueue
- temporary processing failure
- Redis connection issue

---

### Unit Test Checklist
- job payload validation
- processor branching by type
- retry trigger on failure
- failure logging structure

---

### Integration Test Checklist
- API → enqueue job → worker processes
- retry works on simulated failure
- logs produced correctly

---

### Definition of Done
- worker runs via Docker
- queue + processor working
- retry behavior verified
- logs observable
- ready for provider integration

---

### New Updates
- BullMQ pipeline fully implemented in `tag-me-worker` repo (queue, processor, handlers, DLQ, retry)
- Simulated delivery only — no real provider yet
- No dedicated PR; built as part of repo init alongside TAG-13 work
- TAG-16 status in Linear still shows Backlog — needs manual update

---

## Issue 15.1
**Title:** [API] Notification enqueue integration from conversation events  
**Priority:** High  
**Estimate:** S  
**Labels:** api, worker, integration

### Why
Worker requires proper producer integration to function.

---

### Scope

Trigger enqueue from:

#### 1. Message creation
POST /messages

#### 2. Reminder endpoint
POST /conversations/:id/reminder

- Implement helper:
enqueueNotification(payload)

---

### Acceptance Criteria
1. Creating message → enqueue `new_message`
2. Sending reminder → enqueue `reminder`
3. Payload matches worker contract
4. No sensitive data leakage

---

### Definition of Done
- API successfully triggers worker jobs
- End-to-end flow works

---

### New Updates
- Implemented in [PR #10](https://github.com/carissafarry/tag-me/pull/10)
- `enqueueNotification` helper added to both POST /messages and POST /conversations/:id/reminder
- Async goroutine with panic recovery — enqueue failure does not block API response
- Payload includes: `type`, `conversation_id`, `owner_contact`

---

## Issue 16
**Title:** [WORKER] Retry strategy and failure observability (DLQ-style)  
**Priority:** High  
**Estimate:** S  
**Labels:** worker, reliability, bullmq

### Why
Failures must be **visible, retryable, and safe**.

---

### Scope
- Configure BullMQ retry:
  - attempts: 3–5
  - exponential backoff

- Implement failure observability:

#### Logs
event=notification_failed job_id=... reason=...

#### Optional
notification_failures

---

### Out of Scope
- full DLQ infra (RabbitMQ)
- manual admin UI

---

### Acceptance Criteria
1. Failed notification attempts are retried automatically
2. Retry count and delay follow configured policy
3. Terminal failure is recorded clearly
4. Retry flow does not create duplicate success states
5. Worker remains idempotent

---

### Edge Cases
- intermittent provider recovery
- duplicate retry scheduling
- poisoned job
- retry exhaustion
- Redis downtime

---

### Unit Test Checklist
- retry behavior
- backoff timing
- failure logging
- idempotency

---

### Integration Test Checklist
- simulate failure → retry → success
- simulate failure → max retry → fail
- logs visible for both cases

---

### Definition of Done
- retry policy working
- failure path observable
- safe for production usage

---

### New Updates 
**[DONE] Fix Retry Logic (2026-04-16):**
- Implemented in [PR #1 (worker repo)](https://github.com/carissafarry/tag-me-worker/pull/1)
- BullMQ retry: 3 attempts, exponential backoff
- Structured failure logs: `event=notification_failed job_id=... reason=...`
- 30% simulated random failure used to verify retry behavior
- DLQ-style: failed jobs visible in BullMQ failed set; no separate DB table

---

# EPIC 5 — Abuse Prevention & Reliability

## Issue 17
**Title:** [API] Session tracking and rate limiting for scanner flows  
**Priority:** High  
**Estimate:** M  
**Labels:** api, anti-abuse, redis, backend

### Why
Anonymous messaging requires anti-abuse controls from day one.

### Scope
- Track scanner session
- Enforce IP/session/QR rate limits
- Enforce per-IP global limit (block after N requests across all QRs, not just per QR)
- Enforce per-conversation limit (max messages per conversation)
- Support max 2–3 messages per session rule
- Enforce delay/cooldown between sends
- Store rules in Redis-backed mechanism

### Out of Scope
- full fraud scoring
- admin moderation console

### Acceptance Criteria
1. Scanner message creation is rate-limited by session/IP/QR rules.
2. Scanner cannot exceed configured message count threshold per session.
3. Scanner IP is limited globally across all QRs (prevents multi-QR scanning abuse).
4. Conversations are limited to max messages per conversation (prevents hammering single target).
5. Cooldown prevents rapid repeated sends.
6. Rate limit failures return safe, user-friendly response.
7. Reminder flow respects separate or shared cooldown policy.

### Edge Cases
- Redis unavailable
- multiple tabs from same session
- same QR hit from different IPs
- cooldown boundary timing
- IP limit boundary with concurrent requests
- conversation limit boundary

### Unit Test Checklist
- session id handling
- message count threshold logic (per-session + per-conversation)
- global IP limit logic
- cooldown logic
- rate limit response mapping
- Redis fallback/error handling strategy

### Integration Test Checklist
- repeated message attempts hit session rate limit
- cooldown blocks rapid retry
- normal requests still succeed under threshold
- multi-QR abuse from single IP blocked by global IP limit
- per-conversation limit enforced across multiple sessions

### Definition of Done
- anti-abuse rules implemented (session, IP global, per-conversation)
- Redis integration verified
- tests updated

### New Updates
**Rate limiting enhancements (2026-04-19):**
- Added per-IP global limit: block after N requests across all QRs (not just per-QR) — prevents multi-QR scanning abuse
- Added per-conversation message limit: max messages per conversation — prevents hammering single target
- Defined session rule: max 3 messages per session
- Added explicit cooldown enforcement between sends
- AC #3, #4 added for global IP and per-conversation limits
- New edge cases: IP limit boundary under concurrent requests, conversation limit boundary
- New unit tests: global IP limit logic, per-conversation + per-session threshold logic
- New integration tests: multi-QR abuse from single IP blocked by global IP limit, per-conversation limit across enforced accross multiple sessions
- Definition of Done: anti-abuse rules implemented including session, IP global, and per-conversation (all redis-integrated)

---

## Issue 18
**Title:** [WEB/API] Fallback error handling and safe UX for QR/message failures  
**Priority:** Medium  
**Estimate:** S  
**Labels:** web, api, reliability, ux

### Why
Failure states must remain understandable and privacy-safe.

### Scope
- Standardize safe user-facing error states for:
  - invalid QR
  - expired QR
  - submit failure
  - status unavailable
  - notification uncertainty
- Ensure fallback guidance is consistent across frontend/backend

### Out of Scope
- localization
- support chat

### Acceptance Criteria
1. Invalid/expired QR shows safe fallback guidance.
2. Submit failure state provides retry-friendly messaging.
3. Status fetch failure state is understandable and non-technical.
4. Error responses never expose owner contact or internal details.
5. Fallback UI copy stays aligned with backend error cases.

### Edge Cases
- partial outage
- inconsistent backend error payload
- repeated reload after failure

### Unit Test Checklist
- error code to UI-state mapping
- safe message mapping
- invalid QR handling
- privacy-safe serializer behavior

### Integration Test Checklist
- invalid QR path shows correct fallback UI
- submit failure renders recoverable message
- status failure path renders safe UX

### Definition of Done
- fallback handling standardized
- frontend/backend error alignment verified
- tests updated

---

# FUTURE ENHANCEMENTS (Post-MVP)

## Issue 19
**Title:** [OPS] Rate limit observability and alerting  
**Priority:** Medium  
**Estimate:** M  
**Labels:** ops, observability, logging, monitoring

### Why
Rate limit rejections must be visible and actionable to detect abuse patterns early.

### Scope
- Log all rate limit rejections (IP, session, conversation level)
- Track metrics: blocks/min per source, top IPs, patterns
- Structured logging for analytics
- Alert ops if sustained abuse detected

### Out of Scope
- full fraud dashboard
- real-time analytics UI

### Acceptance Criteria
1. Every rate limit rejection is logged with source, rule violated, timestamp.
2. Logs are queryable by IP, session, conversation.
3. Metrics available for observability dashboard.
4. Alert threshold configurable for sustained abuse.

---

## Issue 20
**Title:** [DOCS] Rate limit configuration documentation and tuning CLI  
**Priority:** Medium  
**Estimate:** S  
**Labels:** docs, ops, configuration

### Why
Rate limit windows, thresholds, and rationale must be documented; ops must tune limits without redeployment.

### Scope
- Document all rate limit windows, thresholds, and rationale
- Provide admin CLI or config interface to adjust limits
- Example: `rtk api config rate-limit --key session_max_messages --value 5`

### Out of Scope
- UI-based admin console
- per-user override system

### Acceptance Criteria
1. All rate limit rules documented with rationale.
2. Admin can adjust limits without redeployment.
3. Changes take effect within 1 min.

---

## Enhancement: Exponential backoff for reminder cooldown
**Title:** [API] Exponential backoff for reminder cooldown  
**Priority:** Medium  
**Estimate:** S  
**Why:** Current cooldown is flat 2 min between every reminder send. Escalating wait time per attempt (1st → 2min, 2nd → 4min, 3rd → 8min) reduces repeat spam more effectively than a fixed window.  
**Scope:** Extend Issue 5 / TAG-10 — change flat `Cooldown: 2 * time.Minute` to count-based `cooldown * 2^(count-1)` in `reminder_service.go`. Lua script in `reminder.go` also needs to receive and apply the computed cooldown per attempt.  
**Out of scope:** Delivery retry backoff (Issue 16 / TAG-13) — already done in worker, separate concern.  
**No schema change needed.** EXPIRED conversations already block reminders entirely before cooldown logic runs.

---

## Backlog: Geographic Detection & IP Reputation
**Title:** [API] Geographic filtering and IP reputation integration  
**Priority:** Low  
**Why:** Defer until abuse patterns emerge post-launch  
**Scope:** Block by country, integrate with IP blocklist services  
**Defer:** Post-MVP, post-incident