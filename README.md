# yaml-compose

`yaml-compose` is a small CLI tool that merges one base YAML file with layered override files.

## How it works

- Input base file: `base.yaml`
- Layer directory: `base.yaml.d/`
- Layer file pattern: `<order>-<name>.yaml` / `<order>-<name>.yml`
- Apply order: sort by numeric prefix in file name, then by name
  - Example: `1-default.yaml`, `10-prod.yaml`
  - Note: layer file names must include a numeric prefix (for example, `1-...`).

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
At most two YAML documents are supported.

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
```

Options:

- `-o, --output`: write result to a file (otherwise prints to stdout)

Breaking change: `--extract-layer` has been removed.

## Transform list filter (v2)

Layer metadata supports a built-in transform to load an external YAML list and filter it with `grep` + `grep -v` style rules before merge.

Transform schema:

```yaml
transform:
  kind: list_filter
  source:
    file: ./inventory.yaml
    path: app.backends
  target:
    path: app.backends
  list_filter:
    match_path: name
    include: ["prod-", "cn-"]
    exclude: ["-canary$", "-deprecated$"]
    include_mode: any
```

Rules:

- `source.file`: external YAML file path (relative to base YAML directory if not absolute)
- `source.path`: dot path in source YAML; must resolve to a list
- `target.path`: where filtered list is written into current layer (defaults to `source.path`)
- `list_filter.include`: keep matched items (`grep` behavior)
- `list_filter.exclude`: remove matched items (`grep -v` behavior)
- `list_filter.include_mode`: `any` (default) or `all`
- Input list supports:
  - `[]string`: match each string item directly
  - `[]object`: require `match_path`, match the string at that object path
- For `[]object`, missing `match_path` or non-string value returns an error

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

`base.yaml.d/2-transform.yaml`:

```yaml
transform:
  kind: list_filter
  source:
    file: inventory.yaml
    path: app.backends
  target:
    path: app.backends
  list_filter:
    include: ["prod-"]
    exclude: ["-canary$"]
---
app:
  backends: []
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
task build
task test
task install
```
