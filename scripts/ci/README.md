# CI Scripts

This directory hosts repository checks that must stay runnable from both local
development and GitHub Actions.

## Local-Service Go Style

Run the local-service style gate before submitting backend Go changes:

```powershell
go run ./scripts/ci/local-service-style
go vet ./services/local-service/...
go run honnef.co/go/tools/cmd/staticcheck ./services/local-service/...
go test ./services/local-service/...
```

`local-service-style` verifies that `goimports` has been applied to changed Go
files under `services/local-service`, rejects newly added Chinese Go comments in
local-service diffs, and scans all current `services/local-service` Go comments
so historical comment debt cannot silently return, and enforces a 4,000-line
ceiling for changed non-test Go files. The file-size guard only reports files
that both exceed the ceiling and grow relative to the base or local pre-change
snapshot, so historical oversized files can still receive focused fixes without
making the size debt worse. In pull requests, CI passes the base SHA so
added-comment diagnostics still point at the PR diff. In local mode, the
added-comment guard evaluates staged hunks against the index snapshot and
unstaged hunks against the working tree, so partial staging in the same file does
not skew reported line numbers.

The repository pins `goimports` and `staticcheck` through the root [`go.mod`](../go.mod),
so local runs and CI stay on the same tool versions until the module is
intentionally updated.
