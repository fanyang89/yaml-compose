# `merge` 算子

`merge` 用于把一个对象合并到当前状态。

## 适用场景

- 进行常规 layer 覆盖
- 按路径调整 map/list 合并策略
- 从 `layer`、`file` 或 `state` 读取待合并数据

## 字段说明

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

- `merge.defaults.map` 默认值：`deep`
- `merge.defaults.list` 默认值：`override`
- `merge.paths` 用于精确路径覆盖默认策略

## 示例

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

输出：

```yaml
app:
  db:
    host: prod
    pool: 10
    ports:
      - 5432
      - 5433
```
