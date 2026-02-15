# yaml-compose

`yaml-compose` is a small CLI tool that merges one base YAML file with layered override files.

## How it works

- Input base file: `base.yaml`
- Layer directory: `base.yaml.d/`
- Layer file pattern: `*.yaml` / `*.yml`
- Apply order: sort by numeric prefix in file name, then by name
  - Example: `1-default.yaml`, `10-prod.yaml`

## Merge semantics

- `map + map`: deep merge recursively
- `list + list`: override with layer list
- scalar values: override with layer value
- `null`: override to `null` (keep key, do not delete)

## Usage

```bash
yaml-compose base.yaml
yaml-compose base.yaml -o out.yaml
```

## Example

`base.yaml`:

```yaml
app:
  db:
    host: base
    pool: 10
    ports:
      - 5432
feature: true
```

`base.yaml.d/1-override.yaml`:

```yaml
app:
  db:
    host: layer
    ports:
      - 5433
feature: null
```

Output:

```yaml
app:
  db:
    host: layer
    pool: 10
    ports:
    - 5433
feature: null
```

## Build and test

```bash
make build
make test
```
