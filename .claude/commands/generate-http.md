Generate manual HTTP request files for a Linear issue.

Arguments:
- Issue ID or issue title

Instructions:
1. Use the Linear issue as the source of truth
2. Inspect the implemented code and routes relevant to the issue
3. Create or update one or more request files under `requests/`
4. Choose the file path based on the issue domain, for example:
   - `requests/scanner/...`
   - `requests/owner/...`
   - `requests/internal/...`
5. Each request file must include:
   - `@baseUrl` variable
   - happy path request
   - invalid input request
   - edge case request
   - short comments for expected responses
   - include request body examples
6. Keep the requests aligned with the real implemented API, not just the issue description
7. Return only valid Postman collection JSON.
