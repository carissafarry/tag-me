Prepare a completed Linear issue for shipping to GitHub.

Arguments:
- Issue ID or issue title

Instructions:
1. Use the Linear issue as the source of truth
2. Derive the canonical Linear issue URL:
   - `https://linear.app/tag-me/issue/<ISSUE_ID>`
3. Inspect current git status and recent commits on the current branch
4. Run the real coverage gate before any PR action:
   - `./scripts/pr_gate.sh 70`
5. If tests fail:
   - do not create a PR
   - report the failure clearly
6. If coverage is below 70:
   - do not create a PR
   - commits are allowed, but shipping stops here
   - report the real coverage value and what should be improved
7. If coverage is 70 or above:
   - show:
     - current branch name
     - PR title
     - PR summary
     - real coverage number
     - acceptance criteria covered
     - tests added
   - wait for confirmation before creating the PR
8. After confirmation:
   - push the current branch if needed
   - create a PR with `gh pr create`
   - include a short coverage note in the PR body:
     - `Coverage gate passed: XX.X%`
     - `Note: additional edge cases may still be handled in follow-up work`
9. Return:
   - pushed branch name
   - commit message used
   - PR title
   - PR summary
   - PR URL

Do not create new commits unless explicitly asked.
PR Title, branch name, and commit message must start with Issue ID
PR body must follow the repository PR template