# Tag Me

QR-code-based vehicle communication platform. Portfolio-grade product with web and mobile apps.

[API Docs](https://documenter.getpostman.com/view/13444669/2sBXwpMWSf)

## Overview

Tag Me enables vehicle owners to share a QR code on their vehicle. When others scan the code, they can start or resume conversations with the owner, send messages, and request reminders. Built with production-quality standards, comprehensive testing, and small, reviewable PRs.

## Tech Stack

- **Web**: Next.js 16, React 19, TypeScript, Tailwind CSS
- **API**: Go (Gin router), pgx (PostgreSQL driver)
- **Database**: PostgreSQL
- **Cache/Queue/Sessions**: Redis
- **Testing**: Go test suite (70% coverage gate), TypeScript unit tests
- **Version Control**: GitHub with Linear issue tracking

## Project Structure

```
tag-me/
├── apps/
│   ├── api/          # Go backend (Gin, pgx, PostgreSQL)
│   ├── web/          # Next.js 16 frontend (React 19, TypeScript, Tailwind)
│   └── worker/       # Node.js background job queue (optional)
├── scripts/          # Build, deployment, and testing scripts
├── docs/             # Development guides and setup
├── requests/         # API request examples (Postman snippets)
└── Makefile          # Common commands (api-dev, fe-dev, api-test, pr-gate)
```

## Core Features

**Conversation System**
- Scan QR code → start or resume conversation with vehicle owner
- Session-based tracking (one active conversation per user per QR code)
- Message threads with creation timestamps
- Conversation status tracking (ACTIVE/RESOLVED)

**Vehicle Identification**
- QR codes store vehicle plate numbers (e.g., "ABC-1234")
- Plate displayed in conversation context
- Owner-managed QR code creation

**Reminders**
- Users request reminders for follow-up
- Cooldown between reminders per user
- Daily limit enforcement per user
- Toast notifications (success/error/cooldown states)
- Already-sent detection

**Account Management**
- OTP-based authentication
- Account locking after failed OTP attempts
- Account enable/disable via is_active flag
- Session management

## Getting Started

### Prerequisites

- Go 1.20+
- Node.js 18+
- PostgreSQL 14+
- Redis 6+
- Make

### Environment Setup

1. **Clone repository**
   ```bash
   git clone <repo-url>
   cd tag-me
   ```

2. **Install dependencies**
   ```bash
   # API dependencies (Go modules)
   cd apps/api
   go mod download

   # Web app dependencies
   cd ../web
   npm install

   # Worker dependencies
   cd ../worker
   npm install
   ```

3. **Configure environment**
   - Copy `.env.example` to `.env` (if provided)
   - Set up local database and Redis connections
   - Configure API base URLs for web app

### Local Development

**Start API server** (with hot reload):
```bash
make api-dev
```
Requires Air: `go install github.com/air-verse/air@latest`

**Start web dev server**:
```bash
make fe-dev
```
Runs on `http://localhost:3000`

**Start worker**:
```bash
cd apps/worker
npm run dev
```

## Testing

### API Tests
```bash
make api-test              # Run all Go tests
make api-coverage          # Generate coverage report with breakdown
```

**Coverage gate**: 70% minimum before PR. Run `./scripts/pr_gate.sh 70` locally to verify.

### Web Tests
```bash
cd apps/web
npm test                   # TypeScript unit tests
npm run lint               # ESLint validation
```

**Testing strategy**: Unit tests for business logic, integration tests for user flows.

## Data Model

**Conversations**: Tied to QR code + owner, tracks ACTIVE/RESOLVED status
**Messages**: Linked to conversation, includes session_id for per-user tracking
**QR Codes**: Store vehicle plate, owner reference
**Owners**: Account info, is_active flag for disabling

## Development Workflow

1. **Start**: Pick Linear issue (source of truth for requirements)
2. **Explore**: Inspect relevant Go/React files and understand current patterns
3. **Plan**: Brief implementation outline, identify affected files
4. **Code**: Implement with tests; one issue per PR
5. **Test**: Run `make api-test`, `make api-coverage` to verify coverage ≥ 70%
6. **PR**: Reference Linear issue, include acceptance criteria and coverage number
7. **Review**: GitHub code review before merge

## API & Examples

API requests documented in `requests/` directory (Postman snippets for local testing).

---

## AI Integration

This project leverages **Claude AI** for development assistance.
**Tools Used**:
- Claude API for semantic code search and understanding
- Memory system for project context and patterns
- Code review assistance with risk analysis
- Test-driven development (TDD) guidance

---

## How It Works (User Flow)

1. Vehicle owner signs up and creates a QR code with their vehicle plate
2. Owner shares/prints QR code and attaches to vehicle
3. Scanner scans the QR code via web/mobile app
4. App detects if scanner has active conversation (session-based)
   - If yes: resume existing conversation
   - If no: create new conversation
5. Scanner sends messages and can request reminders
6. Owner receives messages and responds
7. Conversation marked RESOLVED when complete

## Current Development

**TAG-20**: OTP authentication flow
- Request OTP endpoint (rate-limited, cooldown)
- Verify OTP with account locking on failed attempts
- HTTP status codes with Retry-After headers
- Account disabling via is_active flag
