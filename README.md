# yaml-compose

`yaml-compose` is a small CLI tool that builds one final YAML file from:

- one base file (for example `app.yaml`)
- one layer directory with the same name plus `.d` (for example `app.yaml.d/`)

Layer files are loaded in order and merged onto the base document.

## How it works

Given:

- `config.yaml`
- `config.yaml.d/*.yaml` or `config.yaml.d/*.yml`

`yaml-compose` will:

1. sort layer files by numeric prefix before `-` (e.g. `1-...`, `10-...`)
2. load each layer in that order
3. overwrite base top-level keys with layer values
4. output the merged YAML

## Install

### Build binary

```bash
make build
```

This creates `./yaml-compose`.

### Install to `GOPATH/bin`

```bash
make install
```

## Usage

```bash
yaml-compose [YAML-FILE]
```

Options:

- `-o, --output`: write result to a file (otherwise prints to stdout)

## Example

Directory layout:

```text
config.yaml
config.yaml.d/
  1-dev.yaml
  10-local.yaml
```

`config.yaml`:

```yaml
service: app
replicas: 2
debug: false
```

`config.yaml.d/1-dev.yaml`:

```yaml
replicas: 1
debug: true
```

Run:

```bash
yaml-compose config.yaml -o merged.yaml
```

`merged.yaml`:

```yaml
service: app
replicas: 1
debug: true
```

## Notes

- Only files ending with `.yaml` or `.yml` are considered layers.
- Layer filenames must include `-` (for example `10-feature.yaml`).
- Merge strategy is key replacement at the root level.
