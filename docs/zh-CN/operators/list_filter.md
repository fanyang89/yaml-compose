# `list_filter` 算子

`list_filter` 根据 include/exclude 正则规则过滤列表。

## 适用场景

- 只保留符合命名规则的条目
- 在合并前去掉 canary/deprecated 等条目

## 字段说明

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
    rewrite:
      prefix: svc-
      path: name
```

- 输入列表支持 `[]string` 与 `[]object`
- `[]object` 必须设置 `match_path`，且该路径值必须是字符串
- `include_mode` 默认值为 `any`
- `target.merge.defaults.list` 默认值为 `override`
- `rewrite.prefix` 用于给命中的字段增加前缀
- `rewrite.path` 可选，默认使用 `match_path`

## 示例

layer:

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

`app.backends` 结果：

```yaml
- name: prod-a
```
