# Redis Keys Inventory

Complete catalog of all Redis keys used in Tag Me codebase.

## Summary Table

| Key Pattern | Prefix | Type | Content | Default TTL | Source |
|---|---|---|---|---|---|
| `msg:<sessionID>:<qrID>` | `msg` | Hash | `count`, `last_sent_at` | 6h | message_state.go |
| `cooldown:<sessionID>:<qrID>:<action>` | `cooldown` | String | Unix timestamp | Dynamic | cooldown.go |
| `reminder:<sessionID>:<qrID>` | `reminder` | Hash | `count`, `last_sent_at` | 6h | reminder.go |
| `cooldown:<sessionID>:<qrID>:reminder` | `cooldown` | String | Unix timestamp | 2m | reminder.go |
| `conversation_creation_guard:<sessionID>:<ipAddress>:<qrID>` | `conversation_creation_guard` | String | `"1"` flag | 60s | conversation_creation_guard.go |
| `ip:<ipAddress>:<qrID>` | `ip` | String | Request count (int) | 10m | ip_rate_limit.go |

## Repository Breakdown

### MessageStateRepository
**File**: `apps/api/internal/repository/message_state.go`

Tracks message send count per session/QR combination.

**Key Pattern**: `msg:<sessionID>:<qrID>`
- Type: Hash
- Fields:
  - `count` (integer): Number of messages sent
  - `last_sent_at` (unix timestamp): When last message was sent
- Default TTL: 6 hours
- Operations:
  - `TrackMessage()`: Atomic increment with Lua script
  - `GetState()`: Retrieve current state

### CooldownRepository
**File**: `apps/api/internal/repository/cooldown.go`

Generic cooldown tracker for rate-limiting actions.

**Key Pattern**: `cooldown:<sessionID>:<qrID>:<action>`
- Type: String
- Value: Unix timestamp (when action is next allowed)
- TTL: Varies by action type
- Operations:
  - `GetNextAllowedAt()`: Get cooldown expiry time
  - `SetNextAllowedAt()`: Set cooldown with custom TTL

### ReminderRepository
**File**: `apps/api/internal/repository/reminder.go`

Tracks reminder send count and enforces cooldowns per session/QR.

**Key Patterns**:
1. `reminder:<sessionID>:<qrID>`
   - Type: Hash
   - Fields:
     - `count` (integer): Number of reminders sent
     - `last_sent_at` (unix timestamp): When last reminder was sent
   - Default TTL: 6 hours

2. `cooldown:<sessionID>:<qrID>:reminder` (internal)
   - Type: String
   - Value: Unix timestamp for next allowed reminder
   - TTL: 2 minutes (default, configurable via `REMINDER_COOLDOWN_SECONDS`)

**Operations**:
- `ReserveReminder()`: Atomic check-and-set with Lua script (enforces max count and cooldown)
- `GetState()`: Retrieve current reminder state

### ConversationCreationGuardRepository
**File**: `apps/api/internal/repository/conversation_creation_guard.go`

Rate-limit guard: prevents multiple conversation creations for same session/IP/QR.

**Key Pattern**: `conversation_creation_guard:<sessionID>:<ipAddress>:<qrID>`
- Type: String
- Value: `"1"` (simple flag marker)
- TTL: 60 seconds (default, configurable via `CONVERSATION_CREATION_COOLDOWN_SECONDS`)
- Operations:
  - `ReserveConversationCreation()`: SetNX (set if not exists) to enforce one-per-window

### IPRateLimiter
**File**: `apps/api/internal/repository/ip_rate_limit.go`

Request rate limiting by IP address and QR code.

**Key Pattern**: `ip:<ipAddress>:<qrID>`
- Type: String
- Value: Request count (integer)
- TTL: 10 minutes (default, configurable via `IPRateLimitTTL`)
- Operations:
  - `IncrementAndCheck()`: Atomic increment with Lua script (enforces max requests per window)

## Configuration

All TTLs are configurable via environment variables or defaults in `apps/api/internal/config/app.go`:

| Setting | Env Var | Default | Used By |
|---|---|---|---|
| `MessageStateTTL` | (hardcoded) | 6 hours | MessageStateRepository |
| `ReminderStateTTL` | (hardcoded) | 6 hours | ReminderRepository |
| `IPRateLimitTTL` | (hardcoded) | 10 minutes | IPRateLimiter |
| Conversation Creation Cooldown | `CONVERSATION_CREATION_COOLDOWN_SECONDS` | 60 seconds | ConversationCreationGuardRepository |
| Reminder Cooldown | `REMINDER_COOLDOWN_SECONDS` | 2 minutes | ReminderRepository (internal) |

## Data Type Patterns

### Strings
Used for simple values (counters, timestamps):
- `ip:<ipAddress>:<qrID>` - request count
- `cooldown:*` - unix timestamp
- `conversation_creation_guard:*` - flag marker

### Hashes
Used for structured state with multiple fields:
- `msg:<sessionID>:<qrID>` - message state
- `reminder:<sessionID>:<qrID>` - reminder state

## Lua Scripts

Two atomic scripts are used to handle complex operations:

1. **Message Tracking** (`trackMessageScript` in message_state.go)
   - Increments count
   - Updates last_sent_at
   - Sets expiry

2. **Reminder Reservation** (`reserveReminderScript` in reminder.go)
   - Checks max reminder limit
   - Checks cooldown status
   - Atomically increments and updates cooldown if allowed

3. **IP Rate Limit** (`incrementIPRateLimitScript` in ip_rate_limit.go)
   - Increments request count
   - Sets initial expiry on first increment
   - Enforces max requests per window

## Key Naming Convention

All keys follow the pattern: `<prefix>:<param1>:<param2>[:<param3>]`

- Uses colon (`:`) as delimiter
- Components are lowercase prefix + variable parameters
- No special characters in values
- Scoped by session/IP for isolation
