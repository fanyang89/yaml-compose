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
  merge:
    defaults:
      list: override|append|prepend
```

- Used by `list_filter`, `list_extract`, `list_remove`, and `replace_values`
- If omitted on list operators, defaults to `source.path`
- `target.merge.defaults.list` is supported by `list_filter` and `list_extract` only, default `override`
- `target.merge` supports `defaults.list` only
- `target.ignore_not_found` is supported by `list_extract` only, default `false`

## Layer Template Variables

You can inject variables from CLI and render them in layer files:

```bash
yaml-compose base.yaml --var URL=https://api.example.com --var ENV=prod
```

```yaml
app:
  url: "{{.URL}}"
```

- Template rendering applies to layer files only
- Base file and `source.from=file` inputs are not templated
- Missing keys return an error when rendering

## Path Syntax

- Dot path: `app.backends`
- Array index: `app.backends[0].name`
- Array selector: `app.backends[name=api].name`
- Quoted selector value: `groups[name="abc 123"].ports`

Selectors must match exactly one array item for write operations.
