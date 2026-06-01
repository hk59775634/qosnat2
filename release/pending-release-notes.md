# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

启动时支持从 state.json.bak 自动恢复；补全 WARP License 应用接口 OpenAPI 文档。

## 新增

- （无）

## 优化

- （无）

## 修复

- `state.json` 损坏或缺失时，优先从同目录 `state.json.bak` 加载并写回主文件，降低配置丢失风险。
- OpenAPI 补充 `POST /api/v1/network/warp/license/apply`，与实现及 UI 对齐。
- warpnetns resolv 单元测试改为临时目录，避免已安装 WARP 环境下 `go test` 失败。

## 删除

- （无）

## 其他

- （无）
