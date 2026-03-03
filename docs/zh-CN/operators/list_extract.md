# `list_extract` 算子

`list_extract` 从对象列表中提取字符串字段，输出 `[]string`。

## 适用场景

- 从对象列表生成名称/ID 列表
- 从 inventory 类文件中提取关键字段

## 字段说明

```yaml
- kind: list_extract
  source:
    from: file|state|layer
    file: ./inventory.yaml
    path: inventory.backends
  target:
    path: app.backend_names
  list_extract:
    extract_path: meta.name
    include: ["prod-"]
    exclude: ["-canary$"]
    include_mode: any|all
```

- `source.path` 必须解析为 `[]object`
- `extract_path` 必填
- 提取值必须是字符串
- `include_mode` 默认值为 `any`

## 示例

layer:

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

`app.backend_names` 结果：

```yaml
- prod-a
```
