# Local Dev

## API hot reload

Install Air:

```bash
go install github.com/air-verse/air@latest
```

Run the API with hot reload from the repo root:

```bash
make api-dev
```

This uses `apps/api/.air.toml`, watches `.go` files, and ignores `tmp`, `vendor`, `node_modules`, `coverage`, and `test-results`.
