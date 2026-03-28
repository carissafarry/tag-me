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

### Out of Scope
- unlimited reminders
- scanner identity system

### Acceptance Criteria
1. Scanner can request a reminder only for eligible conversations.
2. Reminder is blocked during cooldown window.
3. Reminder count is limited per session/conversation rules.
4. Ineligible reminder attempts return a clear response.
5. Successful reminder triggers notification workflow.

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
- Build fallback screen
- Show reminder state
- Show fallback guidance like contacting security
- Use backend-derived status thresholds

### Out of Scope
- escalation integrations
- emergency service integrations

### Acceptance Criteria
1. Follow-up screen appears when scanner has already sent a notification.
2. Fallback screen appears for long no-response cases.
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
- Create or retrieve owner identity
- Return authenticated session/token
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
- Return owner-scoped conversations
- Include message content, timestamps, status, optional location
- Support active/resolved views

### Out of Scope
- advanced filtering
- analytics dashboards

### Acceptance Criteria
1. Authenticated owner can fetch their conversation list.
2. Authenticated owner can fetch conversation detail.
3. Only owner-owned conversations are returned.
4. Response includes fields needed by dashboard/detail UI.
5. Active and resolved conversation states are available.

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
- Expose quick actions:
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
- Allow owner actions:
  - OPENED / Seen
  - ON_THE_WAY
  - RESOLVED
- Validate allowed transitions
- Persist audit timestamps if needed

### Out of Scope
- arbitrary custom status values
- editing scanner messages

### Acceptance Criteria
1. Authenticated owner can update status on own conversation.
2. Invalid status transitions are rejected safely.
3. Updated status is persisted.
4. Updated status is visible via scanner polling endpoint.
5. Unauthorized owner cannot update another owner’s conversation.

### Edge Cases
- repeated same status update
- resolved conversation re-open attempt
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

# EPIC 4 — Notification Delivery

## Issue 15
**Title:** [WORKER] Notification queue consumer for owner alerts  
**Priority:** High  
**Estimate:** M  
**Labels:** worker, notifications, node, mvp

### Why
Message delivery must be asynchronous and reliable.

### Scope
- Implement queue consumer for `send_notification`
- Support owner notification dispatch
- Structure payload for WhatsApp/email providers
- Handle success/failure outcomes
- Update delivery state back to system

### Out of Scope
- advanced provider routing
- realtime websockets
- multi-provider failover matrix

### Acceptance Criteria
1. New scanner message enqueues a notification job.
2. Worker consumes notification jobs successfully.
3. Successful delivery updates delivery state.
4. Failed delivery is marked for retry handling.
5. Worker logs enough structured info for debugging without leaking sensitive data.

### Edge Cases
- provider timeout
- malformed job payload
- duplicate job delivery
- temporary provider outage

### Unit Test Checklist
- job payload validation
- delivery adapter mapping
- success/failure result handling
- retry eligibility logic

### Integration Test Checklist
- API creates job and worker consumes it
- successful send updates delivery state
- failed send enters retry path

### Definition of Done
- queue consumer implemented
- happy path and failure path tested
- structured logging present

---

## Issue 16
**Title:** [WORKER] Notification retry strategy and failure handling  
**Priority:** High  
**Estimate:** S  
**Labels:** worker, notifications, reliability

### Why
Notification delivery failures must be retried to meet reliability goals.

### Scope
- Implement retry policy
- Configure retry count/backoff
- Mark terminal failures
- Expose internal retry trigger endpoint if needed for debug
- Ensure repeated failures do not spam owner

### Out of Scope
- provider switching
- manual ops dashboard

### Acceptance Criteria
1. Failed notification attempts are retried automatically.
2. Retry count and delay follow configured policy.
3. Terminal failure is recorded clearly.
4. Retry flow does not create duplicate final success states.
5. Optional internal retry endpoint is protected and non-public.

### Edge Cases
- intermittent provider recovery
- duplicate retry scheduling
- poisoned job
- retry exhaustion

### Unit Test Checklist
- retry scheduling logic
- terminal failure logic
- duplicate success prevention
- internal retry authorization guard

### Integration Test Checklist
- failed delivery is retried according to policy
- eventual success updates status correctly
- exhausted retries marked as terminal failure

### Definition of Done
- retry policy working
- failure path observable
- tests updated

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
- Support max 2–3 messages per session rule
- Enforce delay/cooldown between sends
- Store rules in Redis-backed mechanism

### Out of Scope
- full fraud scoring
- admin moderation console

### Acceptance Criteria
1. Scanner message creation is rate-limited by session/IP/QR rules.
2. Scanner cannot exceed configured message count threshold.
3. Cooldown prevents rapid repeated sends.
4. Rate limit failures return safe, user-friendly response.
5. Reminder flow respects separate or shared cooldown policy.

### Edge Cases
- Redis unavailable
- multiple tabs from same session
- same QR hit from different IPs
- cooldown boundary timing

### Unit Test Checklist
- session id handling
- message count threshold logic
- cooldown logic
- rate limit response mapping
- Redis fallback/error handling strategy

### Integration Test Checklist
- repeated message attempts hit rate limit
- cooldown blocks rapid retry
- normal requests still succeed under threshold

### Definition of Done
- anti-abuse rules implemented
- Redis integration verified
- tests updated

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