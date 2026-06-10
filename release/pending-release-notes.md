# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

FRR 模式下新增 BGP/OSPF 动态路由结构化配置与 UI，托管静态路由与动态协议分文件管理。

## 新增

- `state.json` 字段 `dynamic_routing`（BGP 邻居/宣告网段、OSPF 网段/区域）
- API：`GET/PUT /api/v1/frr/dynamic-routing`、应用与状态查询端点
- 静态路由页 FRR 区块：BGP/OSPF 表单、保存并应用、vtysh 运行摘要
- 自动启用 `bgpd`/`ospfd`（写入 `/etc/frr/daemons`）并渲染 `dynamic-routing.conf`

## 优化

- FRR 回放（托管路由 / 启动 ApplyAll）同步应用动态路由配置
- FRR include 链增加 dynamic-routing.conf；配置文件编辑器新增「动态路由」标签

## 修复

- （无）

## 删除

- （无）

## 其他

- OpenAPI 补充 DynamicRoutingState 与 FRR dynamic-routing 路径文档
