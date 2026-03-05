# `list_remove` Operator

`list_remove` removes object list items whose `match_path` value matches a condition.

## When To Use

- Remove stale entries from a list
- Clean up items whose nested field is empty

## Fields

```yaml
- kind: list_remove
  source:
    from: file|state|layer
    file: ./inventory.yaml
    path: app.backends
  target:
    path: app.backends
  list_remove:
    match_path: meta.tags
    when:
      is_empty: true
      equals: prod
      not_equals: dev
      has: blue
      has_not: green
    remove: all|single
```

- Source path must resolve to `[]object`
- `match_path` is required
- Configure exactly one condition under `when`:
  - `is_empty: true`
  - `equals: <value>`
  - `not_equals: <value>`
  - `has: <value>`
  - `has_not: <value>`
- Empty means: `null`, `""`, `{}`, `[]`
- `has` behavior:
  - string field: substring match
  - list field: contains item by equality
  - object field: contains key (expected value must be string)
- `has_not` uses the same rules as `has`, but negates the match result
- `remove` default is `all`
- `remove: single` removes the first matched item only

## Example

Layer:

```yaml
operators:
  - kind: list_remove
    source:
      from: state
      path: app.backends
    target:
      path: app.backends
    list_remove:
      match_path: meta.tags
      when:
        is_empty: true
      remove: all
---
app: {}
```
