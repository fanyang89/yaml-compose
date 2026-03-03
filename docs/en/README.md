# Documentation (English)

This folder contains operator manuals for the operators-only model.

Chinese docs: [Chinese documentation index](../zh-CN/README.md)
Project overview: [root README](../../README.md)

## Start Here

- [Common fields and path syntax](operators/common.md)
- [`merge` operator](operators/merge.md)
- [`list_filter` operator](operators/list_filter.md)
- [`list_extract` operator](operators/list_extract.md)
- [`replace_values` operator](operators/replace_values.md)

## Operator Quick Picks

- Merge maps/lists from layer/file/state into target: [`merge`](operators/merge.md)
- Keep list items matching conditions: [`list_filter`](operators/list_filter.md)
- Extract values from list items to another path: [`list_extract`](operators/list_extract.md)
- Replace values by exact mapping rules: [`replace_values`](operators/replace_values.md)

## Important

- Metadata supports only `operators` at top level.
- Legacy metadata fields are not supported: `merge`, `transform`, `transforms`.
