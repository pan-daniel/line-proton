# Contributing

PRs are welcome. The bar is the same for everyone, and CI enforces most of it.

## Setup

All you need is Go. `go.mod` pins the exact toolchain, so any recent Go downloads the right one by itself. Then:

```bash
make ci
```

That runs the same four checks CI runs, and you can run each one on its own:

- `make format`: fails if `gofmt -l .` lists anything
- `make lint`: `go vet`, staticcheck, our style checker and the third-party licence check
- `make test`: `go test -race ./...`
- `make vulncheck`: govulncheck on our code and dependencies

`make build` makes a snapshot release build with the same GoReleaser version CI uses.

A PR only goes in once the `format`, `lint`, `test` and `vulncheck` checks are green.

## Rules

- **TDD.** Write the failing test first. No PR without tests.
- **DDD.** Decisions live in `internal/domain` as immutable values. `internal/application` only orchestrates through ports. I/O lives in `internal/infrastructure`. `cmd/line-proton` only wires things together.
- **Immutable values.** Unexported fields, validating constructors, methods return new values, no setters.
- **No side effects** outside infrastructure adapters. Nothing mutates its arguments or package state.
- **Code reads like prose.** If a function or type does more than one thing or needs reading twice, split it.
- **Names are spelled out.** No single letters or abbreviations; `ctx` and `err` are the exceptions.
- **No comments, no `else`, no `continue`, no nested control flow, no void functions.** `make lint` checks all of these.
- **Standard library first.** Add a dependency only with a concrete reason. Tests use `testing` and hand-written fakes.
- **Licences stay in sync.** After adding or upgrading a dependency, run `go run ./internal/tools/licensecheck write` and commit `THIRD_PARTY_LICENSES`. `make lint` fails if you forget.
- **Output goes through `log/slog`**, never `fmt.Print*`.
- **Current Go idioms.** `iter.Seq` and range-over-func, `os.Root`, `encoding/json/v2`, `testing/synctest`, `go tool` for dev tools.
- **Dependency injection.** Collaborators are passed in; `cmd/line-proton` is the only composition root.
- **Tests stay offline.** They never need the network, a real Proton or a display.
- **`docs/` is never committed.**
- **Conventional commits**, e.g. `fix(prefix): keep links into Proton-LINE`.

## Before a release

Run this by hand on a real machine:

1. `line-proton uninstall --purge`
2. `line-proton setup` and click through the installer
3. Log in with email and password, tick "Keep me logged in"
4. Quit LINE, start it again from the menu: no signature error, still logged in
5. `line-proton doctor` shows only ✓
