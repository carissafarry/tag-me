Create a minimal implementation plan for a Linear issue.

Arguments:
- Issue ID or issue title

Instructions:
1. Use the Linear MCP to retrieve the issue
2. Do not restate the full issue description
3. Derive the canonical Linear issue URL using the project rule:
   - `https://linear.app/tag-me/issue/<ISSUE_ID>`
4. Use CLAUDE.md as the source of truth for stack rules
5. Inspect only the minimal relevant files or folders
6. Return only:
   - relevant files/modules
   - 3 to 5 implementation steps
   - minimal test targets
   - blockers or missing prerequisites
7. Keep the answer concise
8. Do not implement yet

Output format:
- Relevant files
- Implementation steps
- Minimal test targets
- Blockers