# yaml-compose

[![CI](https://github.com/fanyang89/yaml-compose/actions/workflows/ci.yml/badge.svg)](https://github.com/fanyang89/yaml-compose/actions/workflows/ci.yml)
![Coverage](https://img.shields.io/badge/Coverage-83.9%25-brightgreen)

`yaml-compose` merges one base YAML file with ordered layer files.

For Chinese docs, see the [Chinese documentation](README.zh-CN.md).

## Install

- Go:

```bash
go install github.com/fanyang89/yaml-compose@latest
```

- Binary packages: [GitHub Releases](https://github.com/fanyang89/yaml-compose/releases)

## Quick Start

1. Create `base.yaml`.
2. Create layer directory `base.yaml.d/`.
3. Add layer file like `base.yaml.d/1-prod.yaml`.
4. Your files should look like this:

```text
.
|-- base.yaml
`-- base.yaml.d
    `-- 1-prod.yaml
```
5. Run `yaml-compose base.yaml`.

`base.yaml`:

```yaml
app:
  db:
    host: base
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

Run:

```bash
yaml-compose base.yaml
```

Output:

```yaml
app:
  db:
    host: prod
    ports:
      - 5432
      - 5433
```

More commands:

```bash
yaml-compose base.yaml
yaml-compose base.yaml -o out.yaml
yaml-compose base.yaml --layer 2-debug.yaml
```

- `-o, --output`: write composed YAML to a file.
- `--layer`: run only one layer file (useful for debugging a specific layer).

## Merge Rules At A Glance

- Layer files must be named as `<order>-<name>.yaml` or `<order>-<name>.yml`.
- Layers are applied by numeric order, then by name.
- Default behavior:
  - map: deep merge
  - list: override
  - scalar: layer overrides base
  - null: explicit null override (key remains)
- You can customize behavior per path with `operators` metadata in each layer.

## Documentation

- [English documentation index](docs/en/README.md)
- [Chinese documentation index](docs/zh-CN/README.md)

## License

MIT. See the [LICENSE file](LICENSE).
