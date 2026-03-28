# Tag Me — Claude Project Guide

## Project Goal
Tag Me is a portfolio-grade product with web and mobile apps.
The goal is to build high-quality, production-minded features with small, testable pull requests.

## Source of Truth
- Linear is the source of truth for backlog, sprint scope, and issue requirements.
- GitHub is the source of truth for code review, pull requests, and merge history.
- The repository codebase is the source of truth for implementation details.

## Working Style
- Always start from a Linear issue or a clearly scoped task.
- Before coding, inspect the relevant code paths first.
- Keep each PR focused on one issue only.
- Prefer small, reviewable changes over large rewrites.
- Do not mix unrelated refactors into issue work unless explicitly requested.

## Default Workflow
For every issue:
1. Read the Linear issue
2. Summarize the goal, scope, and acceptance criteria
3. Inspect the relevant files in the repository
4. Propose a minimal implementation plan
5. Implement one step at a time
6. Add or update tests for the acceptance criteria
7. Run relevant test commands
8. Commit using the Linear issue ID
9. Push branch
10. Create a PR using the repository template

## Planning Rules
- Break work into small, verifiable steps
- Each step should be completable in one focused iteration
- Mention affected files/modules before coding
- Explicitly map implementation work to acceptance criteria
- Do not implement before a plan exists unless the task is trivial

## Testing Rules
- Every functional change should include or update tests where appropriate
- Unit tests should cover business logic and validation
- Integration tests should cover important user flows
- Do not claim completion unless tests pass or gaps are explicitly stated

## Git Rules
- Branch names must include the Linear issue ID
- Commit messages must start with the Linear issue ID
- PR titles must start with the Linear issue ID

## Branch Naming Convention
Use:
- `tag-123-short-description`
- `mobile-45-fix-login-state`
- `web-88-add-tag-form`

If the exact issue code is available, always use that format.

## Commit Message Convention
Format:
- `TAG-123 feat: add tag creation form`
- `TAG-124 fix: prevent duplicate tag names`
- `TAG-125 test: add validation coverage`

## Pull Request Rules
PR description must include:
- summary
- linked Linear issue
- acceptance criteria covered
- tests added/updated
- risks or follow-ups

## Coding Rules
- Prefer readability over cleverness
- Follow existing patterns before introducing new abstractions
- Keep modules cohesive
- Avoid premature abstractions
- Avoid touching unrelated files

## Security Rules
- Never hardcode secrets
- Never commit tokens or credentials
- Validate all user input
- Use environment variables for secrets and configuration
- Be careful with logs that may expose sensitive data

## Output Format
When finishing a task, always summarize:
- issue worked on
- files changed
- acceptance criteria completed
- tests added or updated
- commands run
- suggested commit message
- suggested PR title
- remaining follow-ups

## Cost Discipline
To reduce token usage:
- inspect only relevant files
- summarize before implementing
- avoid reading the whole repository when unnecessary
- work in small issue-sized units
- prefer incremental edits over large rewrites

## Useful MCP Usage
When available:
- use Linear MCP for issue retrieval and task context
- use GitHub MCP for PR/issue/repo context
- use Serena for repository inspection and targeted code navigation

## Tool Selection Rules
- Use Serena first for any code implementation, debugging, refactor, or test-related task.
- Do not use Serena for documentation-only or Linear issue creation tasks.
- Use superpowers:brainstorming only when the solution approach is unclear.
- Use superpowers:writing-plans after issue scope is clear and Serena inspection is done.
- Use superpowers:executing-plans only after a written plan exists or has been approved.
- Prefer separate phases:
  1. inspect with Serena
  2. write plan
  3. execute plan
  4. ship

## Serena Usage Rules
- For any implementation task, use Serena first to inspect the relevant codebase areas.
- Do not rely only on generic file reads when Serena is available.
- Before planning or coding, identify:
  - relevant files
  - relevant symbols/functions
  - existing patterns to follow
- Mention explicitly which files/modules were identified through Serena.
- Skip Serena only for non-code tasks such as:
  - Linear backlog creation
  - PRD rewriting
  - issue writing
  - documentation-only tasks

## Fixed Stack Rules
- apps/web uses Next.js
- apps/api uses Golang
- apps/worker uses Node.js
- PostgreSQL is the primary database
- Redis is used for cache, queue, session, and rate-limit support
- Do not propose alternative stacks unless explicitly asked
- Do not redesign the stack during issue execution