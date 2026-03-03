# 算子通用字段

## Metadata 结构

每个 layer 可以有 1 或 2 个 YAML 文档：

1. metadata 文档（可选）
2. data 文档

metadata 仅支持 `operators`：

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
  - `layer`：读取当前 layer 的 data 文档
  - `file`：读取外部 YAML 文件
  - `state`：读取当前已合成状态
- `file`：仅当 `from=file` 时必填
- `path`：`merge` 可选；列表与替换算子通常必填

## `target`

```yaml
target:
  path: app.backends
```

- `list_filter`、`list_extract`、`replace_values` 会使用
- 列表算子未设置时默认等于 `source.path`

## 路径语法

- 点路径：`app.backends`
- 数组下标：`app.backends[0].name`
- 数组选择器：`app.backends[name=api].name`
- 带引号的选择器值：`groups[name="abc 123"].ports`

写入时，选择器必须且只能匹配一个数组元素。
