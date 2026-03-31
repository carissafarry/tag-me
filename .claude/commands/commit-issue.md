Create logical commits for a Linear issue, but do not create a PR yet.

Arguments:
- Issue ID or issue title

Instructions:
1. Use the Linear issue as the source of truth
2. Derive the canonical Linear issue URL:
   - `https://linear.app/tag-me/issue/<ISSUE_ID>
3. Inspect the current git diff
4. Exclude local-only files from commits, including:
   - .env files
   - tmp/ or build artifact folders
   - local logs
5. Group the remaining changes into logical commits
6. Before committing, show:
   - commit groups
   - files in each group
   - proposed commit messages
7. Wait for confirmation before running git commit
8. Commit messages must:
   - start with the Linear issue ID
   - use clear conventional style when possible
9. Do not push
10. Do not create a PR

Output format:
- Commit plan
- Files per commit
- Commit messages