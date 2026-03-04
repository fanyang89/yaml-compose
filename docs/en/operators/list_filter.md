# `list_filter` Operator

`list_filter` filters a list by include/exclude regex rules.

## When To Use

- Keep only items that match naming patterns
- Remove canary/deprecated items before merge

## Fields

```yaml
- kind: list_filter
  source:
    from: file|state|layer
    file: ./inventory.yaml
    path: app.backends
  target:
    path: app.backends
    merge:
      defaults:
        list: override|append|prepend
  list_filter:
    match_path: name
    include: ["prod-"]
    exclude: ["-canary$"]
    include_mode: any|all
```

- Input list can be `[]string` or `[]object`
- For `[]object`, `match_path` is required and must resolve to a string
- `include_mode` default is `any`
- `target.merge.defaults.list` default is `override`

## Example

Layer:

```yaml
operators:
  - kind: list_filter
    source:
      from: file
      file: ./inventory.yaml
      path: inventory.backends
    target:
      path: app.backends
    list_filter:
      match_path: name
      include: ["prod-"]
      exclude: ["-canary$"]
---
app:
  backends: []
```

`inventory.yaml`:

```yaml
inventory:
  backends:
    - name: prod-a
    - name: prod-canary
    - name: dev-a
```

Result at `app.backends`:

```yaml
- name: prod-a
```
