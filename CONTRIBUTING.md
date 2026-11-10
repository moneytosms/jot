# Contributing to jot

## Prerequisites

- Go 1.22 or newer
- No third-party dependencies. jot uses the standard library only, so do not add to `go.mod`.

## Workflow

1. For large changes, open an issue first to agree on the approach.
2. Make the change on a branch.
3. Format and check:

   ```sh
   make fmt     # gofmt -w .
   make check   # go vet ./... and go test ./...
   ```

4. Open a pull request.

## Standards

- Run `gofmt` on all Go files. `make fmt` does this.
- Every behavior change needs a test in the same change.
- Use conventional commit messages:
  - `feat:` new behavior
  - `fix:` bug fix
  - `docs:` documentation only
  - `refactor:` no behavior change
  - `chore:` tooling, build, or maintenance

  Keep the subject imperative and under 72 characters.

## Licensing

By contributing, you agree that your contribution is licensed under the MIT License in `LICENSE`.
