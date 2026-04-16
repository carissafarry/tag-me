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
- linked Linear issue (the URL)
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

## PR Coverage Gate Rules
- Before opening any PR for an API issue, always run the local PR gate script:
  - `./scripts/pr_gate.sh 70`
- The PR gate must run real tests, not estimated coverage.
- If coverage is >= 70:
  - PR creation is allowed
  - the PR summary must include the real coverage number
  - the PR summary must also note that some edge cases may remain for follow-up
- If coverage is < 70:
  - do not create a PR
  - commits are allowed
  - report the real coverage value and recommend what to improve
- If tests fail:
  - do not create a PR
  - do not claim coverage passed
- Never skip the coverage gate for API issues unless explicitly instructed by the user


## Linear URL Rules
- The Linear team slug is `tag-me`
- The canonical Linear issue URL format is:
  - `https://linear.app/tag-me/issue/<ISSUE_ID>`
- Example:
  - `TAG-7` -> `https://linear.app/tag-me/issue/TAG-7`
- When an issue ID is known, always derive and include the canonical Linear issue URL
- Use the Linear issue ID and URL as the source of truth for:
  - planning
  - commit grouping
  - PR preparation
  - PR body

<!-- code-review-graph MCP tools -->
## MCP Tools: code-review-graph

**IMPORTANT: This project has a knowledge graph. ALWAYS use the
code-review-graph MCP tools BEFORE using Grep/Glob/Read to explore
the codebase.** The graph is faster, cheaper (fewer tokens), and gives
you structural context (callers, dependents, test coverage) that file
scanning cannot.

### When to use graph tools FIRST

- **Exploring code**: `semantic_search_nodes` or `query_graph` instead of Grep
- **Understanding impact**: `get_impact_radius` instead of manually tracing imports
- **Code review**: `detect_changes` + `get_review_context` instead of reading entire files
- **Finding relationships**: `query_graph` with callers_of/callees_of/imports_of/tests_for
- **Architecture questions**: `get_architecture_overview` + `list_communities`

Fall back to Grep/Glob/Read **only** when the graph doesn't cover what you need.

### Key Tools

| Tool | Use when |
|------|----------|
| `detect_changes` | Reviewing code changes — gives risk-scored analysis |
| `get_review_context` | Need source snippets for review — token-efficient |
| `get_impact_radius` | Understanding blast radius of a change |
| `get_affected_flows` | Finding which execution paths are impacted |
| `query_graph` | Tracing callers, callees, imports, tests, dependencies |
| `semantic_search_nodes` | Finding functions/classes by name or keyword |
| `get_architecture_overview` | Understanding high-level codebase structure |
| `refactor_tool` | Planning renames, finding dead code |

### Workflow

1. The graph auto-updates on file changes (via hooks).
2. Use `detect_changes` for code review.
3. Use `get_affected_flows` to understand impact.
4. Use `query_graph` pattern="tests_for" to check coverage.
