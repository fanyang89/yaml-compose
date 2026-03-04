# `list_extract` Operator

`list_extract` extracts one string field from each object list item.

## When To Use

- Build a `[]string` from object list data
- Produce name/id lists from inventory-like files

## Fields

```yaml
- kind: list_extract
  source:
    from: file|state|layer
    file: ./inventory.yaml
    path: inventory.backends
  target:
    path: app.backend_names
    merge:
      defaults:
        list: override|append|prepend
  list_extract:
    extract_path: meta.name
    include: ["prod-"]
    exclude: ["-canary$"]
    include_mode: any|all
```

- Source path must resolve to `[]object`
- `extract_path` is required
- Extracted value must be string
- `include_mode` default is `any`
- `target.merge.defaults.list` default is `override`

## Example

Layer:

```yaml
operators:
  - kind: list_extract
    source:
      from: file
      file: ./inventory.yaml
      path: inventory.backends
    target:
      path: app.backend_names
    list_extract:
      extract_path: meta.name
      include: ["prod-"]
---
app:
  backend_names: []
```

`inventory.yaml`:

```yaml
inventory:
  backends:
    - meta:
        name: prod-a
    - meta:
        name: dev-a
```

Result at `app.backend_names`:

```yaml
- prod-a
```
