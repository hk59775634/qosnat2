# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

WireGuard Peer 支持可选是否将 AllowedIPs 加载到系统路由表

## 新增

- WireGuard Peer：「加载到系统路由表」选项（默认开启）；不勾选时 AllowedIPs 仅用于加密路由，不写入系统路由
- 存在未勾选 Peer 时，wg-quick 使用 `Table = off`，并为仍需路由的 Peer 生成 PostUp/PreDown

## 优化

- （无）

## 修复

- （无）

## 删除

- （无）

## 其他

- 修改后需「保存并 wg-quick apply」生效
