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
yaml-compose base.yaml --layer 2-transform.yaml
```

Options:

- `-o, --output`: write result to a file (otherwise prints to stdout)
- `--layer`: run only one layer file (debugging a specific layer)

Breaking change: `--extract-layer` has been removed.

## Transform list filter (v2)

Layer metadata supports a built-in transform to load an external YAML list and filter it with `grep` + `grep -v` style rules before merge.

You can configure either:

- `transform`: one transform
- `transforms`: multiple transforms (executed in order)

Do not set both in the same layer.

Transform schema:

```yaml
transform:
  kind: list_filter
  source:
    from: file
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

Multiple transforms example:

```yaml
transforms:
  - kind: list_extract
    source:
      file: ./inventory.yaml
      path: app.backends
    target:
      path: app.backend_names
    list_extract:
      extract_path: name
  - kind: list_filter
    source:
      file: ./inventory.yaml
      path: app.backends
    target:
      path: app.backends
    list_filter:
      match_path: name
      include: ["prod-"]
```

Rules:

- `source.file`: external YAML file path (relative to base YAML directory if not absolute)
- `source.from`: `file` (default) or `state`
- `source.file`: required when `source.from=file`; must be empty when `source.from=state`
- `source.path`: dot path in source YAML; must resolve to a list
- `target.path`: where filtered list is written into current layer (defaults to `source.path`)
  - supports array index segments, for example: `app.backends[0].names`
  - supports array object selector segments, for example: `app.backends[name=api].names`
  - selector values can be quoted strings, for example: `groups[name="abc 123"].ports`
  - selector must match exactly one item (0 or multiple matches return an error)
- `list_filter.include`: keep matched items (`grep` behavior)
- `list_filter.exclude`: remove matched items (`grep -v` behavior)
- `list_filter.include_mode`: `any` (default) or `all`
- Input list supports:
  - `[]string`: match each string item directly
  - `[]object`: require `match_path`, match the string at that object path
- For `[]object`, missing `match_path` or non-string value returns an error

## Transform list extract

`list_extract` reads a `[]object` from external YAML, extracts one string field from each object, and writes a `[]string` to `target.path`.

```yaml
transform:
  kind: list_extract
  source:
    from: file
    file: ./inventory.yaml
    path: app.backends
  target:
    path: app.backend_names
  list_extract:
    extract_path: meta.name
    include: ["prod-"]
    exclude: ["-canary$"]
    include_mode: any
```

Rules:

- `source.path` must resolve to `[]object`
- `list_extract.extract_path` is required
- `list_extract.include`: keep matched extracted strings (`grep` behavior)
- `list_extract.exclude`: remove matched extracted strings (`grep -v` behavior)
- `list_extract.include_mode`: `any` (default) or `all`
- Each extracted value must be `string`; missing path or non-string value returns an error

Use `source.from: state` to read from current composed state (after previously applied layers):

```yaml
transform:
  kind: list_extract
  source:
    from: state
    path: app.backends
  target:
    path: app.backend_names
  list_extract:
    extract_path: meta.name
```

## Transform replace values

`replace-values` replaces substring occurrences in string values.

```yaml
transform:
  kind: replace-values
  source:
    from: state
    path: app.payload
  target:
    path: app.payload
  replace_values:
    old: foo
    new: bar
    recursive: false
    print_original: false
```

Rules:

- `replace_values.old` is required and cannot be empty
- `replace_values.new` defaults to empty string when omitted
- `replace_values.recursive` defaults to `false`
- `replace_values.print_original` defaults to `false`; when `true`, each replaced original string value is printed
- Only values are replaced; object keys are never modified
- `recursive: false` replaces only immediate string values on the source node
- `recursive: true` traverses nested objects and arrays, and also replaces string items in arrays
- `source.path` can point to a single string value, object, or array

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
