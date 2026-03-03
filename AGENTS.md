# AGENTS.md

Guidance for coding agents working in this repository.

## Scope

- Repository: `yaml-compose`
- Language: Go
- App type: CLI (`cobra`) that composes YAML files with layered overrides
- Main entrypoint: `main.go`
- CLI wiring: `cmd/root.go`
- Core merge logic: `v1/compose/c.go`
- FS helpers: `v1/fsutils/fsutils.go`

## Environment And Toolchain

- Go version is defined in `go.mod` (`go 1.26.0`)
- Use module-aware commands from repository root
- Preferred task runner: `task` (see `Taskfile.yml`)
- CI runs tests with plain `go test ./...`

## Build / Test / Lint Commands

### Primary Commands

- Build binary: `task build`
- Run all tests: `task test`
- Install locally: `task install`
- Format code: `task fmt`

### Direct Go Equivalents

- Build binary: `go build -o yaml-compose ./main.go`
- Run all tests: `go test ./...`
- Install module binary: `go install .`
- Format all packages: `go fmt ./...`

### Run A Single Test (Important)

- Single test in one package:
  - `go test ./cmd -run TestRootCmdWritesOutputFile`
  - `go test ./v1/compose -run TestComposeExtractLayerPath`
  - `go test ./v1/fsutils -run TestDirExistsOnMemFs`
- Match by regex:
  - `go test ./v1/compose -run 'TestCompose.*Path'`
- Verbose single-test output:
  - `go test -v ./v1/compose -run TestComposeMapOverrideByPath`

### Focused Package Test Commands

- CLI package only: `go test ./cmd`
- Compose engine only: `go test ./v1/compose`
- Filesystem utils only: `go test ./v1/fsutils`

### Optional Extra Validation

- Race checks (optional local pass): `go test -race ./...`
- Vet (optional, not wired in Taskfile): `go vet ./...`

## Project Structure

- `main.go`: minimal executable entrypoint, delegates to `cmd.Execute()`
- `cmd/root.go`: CLI flags, input/output orchestration, dependency wiring
- `v1/compose/c.go`: file ordering, parsing, merge strategies, path extraction
- `v1/fsutils/fsutils.go`: filesystem existence checks with `afero`
- Tests are colocated with packages (`*_test.go`)

## Existing Conventions To Follow

### Formatting

- Always run `go fmt ./...` (or `task fmt`) after edits
- Keep standard Go formatting; do not hand-format against gofmt
- Prefer short, readable functions with explicit early returns

### Imports

- Group imports in three blocks when applicable:
  1. Go standard library
  2. Third-party dependencies
  3. Internal module imports (`github.com/fanyang89/yaml-compose/...`)
- Keep imports gofmt-sorted inside each group
- Avoid unused imports; rely on compiler/test feedback

### Types And Data Modeling

- Prefer concrete structs for domain behavior (`Compose`, strategy structs)
- Use small interfaces for dependency seams (example: `composeRunner` in `cmd`)
- Keep unexported helpers unexported unless cross-package use is needed
- Use `map[string]interface{}` when interacting with dynamic YAML nodes

### Naming

- Exported identifiers: `CamelCase` with clear domain meaning
- Unexported identifiers: `camelCase`
- Test names: `Test<Behavior>` style, explicit and scenario-driven
- File names: short and lowercase; package directories reflect domain

### Error Handling

- Return errors; do not panic for recoverable failures
- Wrap errors with context at boundaries (`fmt.Errorf("context: %w", err)`) when preserving cause
- Existing code also uses `%s` in a few locations; prefer `%w` for new wrapped paths
- Validate input early and fail fast (e.g., layer filename checks)
- Keep error messages actionable and specific to operation stage

### Control Flow

- Prefer guard clauses for invalid states and IO failures
- Keep command execution path linear and explicit
- Avoid hidden side effects; make filesystem writes obvious

### Filesystem And IO

- Use `afero` in code paths that need testable filesystem access
- In tests, prefer `afero.NewMemMapFs()` unless OS behavior is required
- Use explicit file modes (`0644`, `0755`) consistent with repository style

### YAML / Merge Logic Expectations

- Layer files must follow `<order>-<name>.yaml|yml` naming
- Sort layers by numeric prefix, then name
- Respect merge strategy metadata in optional first YAML document
- Preserve current semantics:
  - map default: deep merge
  - list default: override
  - scalar: layer overrides base
  - null: explicit null override (key retained)

## Testing Style Guidelines

- Test framework: `testify/require`
- Typical pattern:
  - `require := require.New(t)`
  - setup fixture data
  - run unit under test
  - assert expected value and error behavior
- Prefer focused tests for one behavior each
- Cover failure paths, not only success paths
- Use `t.Helper()` for reusable setup helpers
- Use deterministic in-memory fixtures unless permissions/OS semantics are under test

## CLI And Dependency Injection Patterns

- Keep CLI setup in `newRootCmd(...)`
- Keep operational logic in testable helpers (`runRootCommand`)
- Pass dependencies via small struct (`commandDeps`) for test overrides
- Keep `Execute()` thin; avoid embedding business logic there

## CI / Release Notes For Agents

- CI workflow currently enforces tests (`go test ./...`) on push/PR to `main`
- Release workflow also runs tests before cross-platform build artifacts
- Do not introduce commands that diverge from CI without clear reason

## Cursor / Copilot Rules

- `.cursor/rules/`: not present
- `.cursorrules`: not present
- `.github/copilot-instructions.md`: not present
- If any of these files are added later, treat them as higher-priority agent instructions

## Change Checklist For Agents

- Run `go fmt ./...`
- Run targeted tests for changed package(s)
- Run `go test ./...` before finalizing significant changes
- Keep public behavior and merge semantics backward compatible unless explicitly asked to change
- Update `README.md` when CLI flags or merge semantics change
