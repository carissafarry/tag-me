Prepare issue work for shipping through git and GitHub.

Arguments:
- Issue ID

Instructions:
1. Verify the issue scope is complete
2. Verify relevant tests were added or updated
3. Suggest:
   - branch name
   - commit message
   - PR title
   - PR body summary
4. If git is clean and changes are ready:
   - create commit using the issue ID
   - push the current branch
   - create a PR using the repository template
5. Return:
   - commit message used
   - PR title
   - PR summary
   - PR URL if created