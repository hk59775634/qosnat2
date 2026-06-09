# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

修复 RADIUS 未启用 groupconfig 时组 API 仍拒绝编辑的问题。

## 新增

- （无）

## 优化

- （无）

## 修复

- ocserv 组 API：仅在 RADIUS 且 `groupconfig=true` 时禁止本地组增删改；取消勾选后可正常保存组与 config-per-group
- `groupconfig` 状态持久化到 state.json（显式 true/false），前端按 `groupconfig === true` 判断是否外部管理

## 删除

- （无）

## 其他

- （无）
