GORELEASER := go run github.com/goreleaser/goreleaser/v2@v2.20.2

.PHONY: ci format lint test vulncheck build

ci: format lint test vulncheck

format:
	@unformatted="$$(gofmt -l .)"; test -z "$$unformatted" || { echo "gofmt needed on:"; echo "$$unformatted"; exit 1; }

lint:
	go vet ./...
	go tool staticcheck ./...
	go run ./internal/tools/stylecheck .
	go run ./internal/tools/licensecheck check

test:
	go test -race ./...

vulncheck:
	go tool govulncheck ./...

build:
	$(GORELEASER) build --snapshot --clean --single-target
