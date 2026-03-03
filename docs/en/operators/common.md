# Common Operator Fields

## Metadata Shape

Each layer can have one or two YAML documents:

1. Metadata document (optional)
2. Data document

Metadata uses `operators` only:

```yaml
operators:
  - kind: merge
    source:
      from: layer
```

## `source`

```yaml
source:
  from: layer|file|state
  file: ./inventory.yaml
  path: app.backends
```

- `from`:
  - `layer`: read from current layer data document
  - `file`: read from external YAML file
  - `state`: read from current composed state
- `file`: required only when `from=file`
- `path`: optional for `merge`, required for list and replace operators

## `target`

```yaml
target:
  path: app.backends
```

- Used by `list_filter`, `list_extract`, and `replace_values`
- If omitted on list operators, defaults to `source.path`

## Path Syntax

- Dot path: `app.backends`
- Array index: `app.backends[0].name`
- Array selector: `app.backends[name=api].name`
- Quoted selector value: `groups[name="abc 123"].ports`

Selectors must match exactly one array item for write operations.
