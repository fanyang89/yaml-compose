# yaml-compose

`yaml-compose` 用于将一个基础 YAML 文件与按顺序组织的 layer 文件进行合并。

英文文档请查看 [英文文档](README.md)。

## 安装

- 使用 Go 安装：

```bash
go install github.com/fanyang89/yaml-compose@latest
```

- 二进制下载：[GitHub Releases](https://github.com/fanyang89/yaml-compose/releases)

## 快速开始

1. 创建 `base.yaml`。
2. 创建目录 `base.yaml.d/`。
3. 添加 layer 文件，例如 `base.yaml.d/1-prod.yaml`。
4. 此时文件结构应如下：

```text
.
|-- base.yaml
`-- base.yaml.d
    `-- 1-prod.yaml
```
5. 运行 `yaml-compose base.yaml`。

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

运行：

```bash
yaml-compose base.yaml
```

输出：

```yaml
app:
  db:
    host: prod
    ports:
      - 5432
      - 5433
```

更多命令：

```bash
yaml-compose base.yaml
yaml-compose base.yaml -o out.yaml
yaml-compose base.yaml --layer 2-debug.yaml
yaml-compose --base ./config/base.yaml --layer-dir ./config/layers
yaml-compose base.yaml --var URL=https://api.example.com --var ENV=prod
```

- `--base`：base yaml 文件路径（可替代位置参数）。
- `--layer-dir`：layer yaml 目录路径（默认 `<base>.d`）。
- `-o, --output`：将合成结果写入文件。
- `--layer`：只执行单个 layer 文件（便于排查某一层）。
- `--var`：注入 layer 模板变量（`KEY=VALUE`，可重复）。

## 合并规则速览

- layer 文件命名必须为 `<order>-<name>.yaml` 或 `<order>-<name>.yml`。
- 执行顺序为：先按数字前缀，再按文件名。
- 默认规则：
  - map：深度合并
  - list：覆盖
  - scalar：layer 覆盖 base
  - null：显式覆盖为 null（保留 key）
- 如需按路径定制行为，可在 layer metadata 的 `operators` 中配置。

## 详细文档

- [英文文档索引](docs/en/README.md)
- [中文文档索引](docs/zh-CN/README.md)

## 许可证

MIT，详见 [LICENSE 文件](LICENSE)。
