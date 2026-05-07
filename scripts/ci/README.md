# CI Scripts

This directory hosts repository checks that must stay runnable from both local
development and GitHub Actions.

## Local-Service Go Style

Run the local-service style gate before submitting backend Go changes:

```powershell
go run ./scripts/ci/local-service-style
go vet ./services/local-service/...
go run honnef.co/go/tools/cmd/staticcheck@latest ./services/local-service/...
go test ./services/local-service/...
```

`local-service-style` verifies that `goimports` has been applied to changed Go
files under `services/local-service` and rejects newly added Chinese Go
comments in local-service diffs. In pull requests, CI passes the base SHA so
the guard checks only new changes instead of failing on historical debt that is
outside the current change boundary.
