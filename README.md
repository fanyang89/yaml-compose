# yaml-compose

`yaml-compose` is a small CLI tool that merges one base YAML file with layered override files.

## How it works

- Input base file: `base.yaml`
- Layer directory: `base.yaml.d/`
- Layer file pattern: `*.yaml` / `*.yml`
- Apply order: sort by numeric prefix in file name, then by name
  - Example: `1-default.yaml`, `10-prod.yaml`

## Merge semantics

- `map + map`: default is `deep` merge recursively
- `list + list`: default is `override` with layer list
- scalar values: override with layer value
- `null`: override to `null` (keep key, do not delete)

### Layer merge metadata

Layer files can optionally use two YAML documents separated by `---`:

1. metadata document (strategy config)
2. data document (actual layer values)

When only one YAML document is present, it is treated as data (backward compatible).

Supported strategies:

- `map`: `deep` | `override`
- `list`: `override` | `append` | `prepend`

Metadata schema:

```yaml
merge:
  defaults:
    map: deep
    list: override
  paths:
    app.db:
      map: override
    app.db.ports:
      list: append
```

Path rules:

- Dot path uses exact match only (no wildcard)
- If a key contains `.`, escape it with `\\.` in metadata path (for example: `app.db\\.main.ports`)
- Priority: `paths` > `defaults` > built-in defaults (`map=deep`, `list=override`)

## Usage

```bash
yaml-compose base.yaml
yaml-compose base.yaml -o out.yaml
yaml-compose base.yaml -e app.db
```

Options:

- `-o, --output`: write result to a file (otherwise prints to stdout)
- `-e, --extract-layer`: extract a field path from each layer before compose (dot path, supports `\.` escape)

When `--extract-layer` is set, each layer file is treated as a large YAML source, and only the extracted field is used as layer payload. The extracted value is wrapped back to its original path before merge.

Example (`--extract-layer app.db`):

- Layer input has `app.db`
- Effective layer payload becomes only `app: { db: ... }`
- Other keys in that layer are ignored
- If a layer does not contain that path, it is skipped

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
merge:
  paths:
    app.db.ports:
      list: append
---
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
    - 5432
    - 5433
feature: null
```

## Build, test, install

```bash
make build
make test
make install
```
