# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

WireGuard Peer 列表支持多选批量删除。

## 新增

- API `POST /api/v1/vpn/wireguard/instances/{id}/peers/batch-delete`（body：`names[]`）
- Web：Peer 表格复选框、全选与「删除选中」按钮

## 优化

- （无）

## 修复

- （无）

## 删除

- （无）

## 其他

- （无）
