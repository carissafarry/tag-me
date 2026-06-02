.PHONY: api-dev

api-dev:
	cd apps/api && air

api-test:
	cd apps/api && go test ./... -coverpkg=./...

api-coverage:
	cd apps/api && go test ./... -coverpkg=./... -coverprofile=coverage.out
	cd apps/api && go tool cover -func=coverage.out

fe-dev:
	cd apps/web && next dev --webpack

pr-gate:
	./scripts/pr_gate.sh 70