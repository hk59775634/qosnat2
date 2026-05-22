# Dashboard 小组件顺序

- 主区与底栏各有一套顺序，保存在浏览器 `localStorage`（`qosnat2-dash-main` / `qosnat2-dash-bottom`）。
- 标题栏 **↑ / ↓** 调整顺序；刷新页面后保持。
- 未做拖拽排序（避免与折叠/移动端冲突）；若需拖拽可后续接 `vue-draggable-plus`。
