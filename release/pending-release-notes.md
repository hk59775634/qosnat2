# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

WARP License Key 任意状态可编辑、独立保存，断开重连后生效

## 新增

- `PUT /api/v1/network/warp/license` 持久化 WARP+ License Key（不立即调用 warp-cli）

## 优化

- License Key 输入框在 WARP 已连接时也可显示与编辑
- 启用 WARP 前若输入框有未保存内容会提示先保存

## 修复

- （无）

## 删除

- 连接 WARP 接口不再接受 `license_key` 参数（改由独立保存接口）

## 其他

- OpenAPI 更新 WARP license 与 connect 说明
