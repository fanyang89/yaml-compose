# `merge` Operator

`merge` merges an object into current composed state.

## When To Use

- Apply normal layer overrides
- Tune map/list merge behavior by path
- Merge data from `layer`, `file`, or `state`

## Fields

```yaml
- kind: merge
  source:
    from: layer|file|state
    file: ./extra.yaml
    path: app
  merge:
    defaults:
      map: deep|override
      list: override|append|prepend
    paths:
      app.db:
        map: override
      app.db.ports:
        list: append
```

- `merge.defaults.map` default: `deep`
- `merge.defaults.list` default: `override`
- `merge.paths` overrides defaults for exact paths

## Example

`base.yaml`:

```yaml
app:
  db:
    host: base
    pool: 10
    ports: [5432]
```

`base.yaml.d/1-prod.yaml`:

```yaml
operators:
  - kind: merge
    source:
      from: layer
    merge:
      paths:
        app.db.ports:
          list: append
---
app:
  db:
    host: prod
    ports: [5433]
```

Output:

```yaml
app:
  db:
    host: prod
    pool: 10
    ports:
      - 5432
      - 5433
```
