Execute one issue in a disciplined, test-oriented way with minimal scope and minimal token usage.

Arguments:
- Issue ID or issue title

Instructions:
1. Use the Linear issue and CLAUDE.md as source of truth
2. Before coding, inspect only the necessary files or folders
3. If no plan exists in the current session, create a minimal implementation plan first
4. Implement only the current issue scope
5. Add or update only the tests for the acceptance criteria
6. Summarize:
   - files changed
   - acceptance criteria covered
   - tests added/updated
   - blockers or follow-ups
7. Do not broaden scope beyond the issue.
8. Do not create a PR yet unless explicitly asked
9. Do not redesign the stack.