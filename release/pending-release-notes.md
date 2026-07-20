# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

修复 sing-box 1.11 因弃用 inet4_address 导致 ProxyEgress TUN 无法创建的问题

## 新增

- （无）

## 优化

- （无）

## 修复

- ProxyEgress sing-box 配置改用 `address` 数组（替代已移除的 `inet4_address`），避免启动报 FATAL 且 TUN qpe* 创建失败

## 删除

- （无）

## 其他

- （无）
