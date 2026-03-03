# `replace_values` 算子

`replace_values` 对字符串值做子串替换。

## 适用场景

- 批量替换 URL/环境片段
- 最终合并前做值归一化

## 字段说明

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

- `replace_values.old` 必填，且不能为空
- `replace_values.new` 默认空字符串
- `recursive` 默认 `false`
- `print_original` 默认 `false`
- 仅替换值，不修改对象 key

## 示例

layer:

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

若当前 state 为：

```yaml
app:
  payload:
    url: http://foo.example
    labels: [foo-a]
```

结果为：

```yaml
app:
  payload:
    url: http://bar.example
    labels: [bar-a]
```
