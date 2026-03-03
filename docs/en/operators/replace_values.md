# `replace_values` Operator

`replace_values` replaces substring occurrences in string values.

## When To Use

- Replace host/env fragments in generated payloads
- Normalize values before final merge

## Fields

```yaml
- kind: replace_values
  source:
    from: file|state|layer
    file: ./input.yaml
    path: app.payload
  target:
    path: app.payload
  replace_values:
    old: foo
    new: bar
    recursive: false
    print_original: false
```

- `replace_values.old` is required and cannot be empty
- `replace_values.new` default: empty string
- `recursive` default: `false`
- `print_original` default: `false`
- Only values are replaced; object keys are not changed

## Example

Layer:

```yaml
operators:
  - kind: replace_values
    source:
      from: state
      path: app.payload
    target:
      path: app.payload
    replace_values:
      old: foo
      new: bar
      recursive: true
---
app: {}
```

If current state has:

```yaml
app:
  payload:
    url: http://foo.example
    labels: [foo-a]
```

Result:

```yaml
app:
  payload:
    url: http://bar.example
    labels: [bar-a]
```
