# `list_remove` 算子

`list_remove` 会删除对象列表中满足条件的元素。

## 适用场景

- 从列表中删除过期条目
- 清理某个嵌套字段为空的项

## 字段说明

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
    remove: all|single
```

- `source.path` 必须解析为 `[]object`
- `match_path` 必填
- `when` 下必须且只能配置一个条件：
  - `is_empty: true`
  - `equals: <value>`
  - `not_equals: <value>`
  - `has: <value>`
- empty 判定包含：`null`、`""`、`{}`、`[]`
- `has` 规则：
  - 字符串字段：子串匹配
  - 列表字段：按等值判断是否包含
  - 对象字段：判断是否包含指定 key（期望值需为字符串）
- `remove` 默认值为 `all`
- `remove: single` 只删除第一个匹配项

## 示例

layer:

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
