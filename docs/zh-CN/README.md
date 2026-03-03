# 文档索引（中文）

本目录包含 operators-only 模型下的各算子使用手册。

英文文档：[英文文档索引](../en/README.md)
项目概览： [根 README](../../README.zh-CN.md)

## 从这里开始

- [通用字段与路径语法](operators/common.md)
- [`merge` 算子](operators/merge.md)
- [`list_filter` 算子](operators/list_filter.md)
- [`list_extract` 算子](operators/list_extract.md)
- [`replace_values` 算子](operators/replace_values.md)

## 算子选型速查

- 将 layer/file/state 数据合并到目标：[`merge`](operators/merge.md)
- 按条件筛选列表项：[`list_filter`](operators/list_filter.md)
- 从列表项提取字段到目标路径：[`list_extract`](operators/list_extract.md)
- 按映射规则替换值：[`replace_values`](operators/replace_values.md)

## 重要说明

- metadata 顶层仅支持 `operators`。
- 旧字段不再支持：`merge`、`transform`、`transforms`。
