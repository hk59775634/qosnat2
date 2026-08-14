# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

修复一键拦截 Google WebRTC 因非法 nft「udp counter」语法导致应用失败。

## 新增

- （无）

## 优化

- （无）

## 修复

- 防火墙规则在仅匹配 tcp/udp 且无端口时改为 `meta l4proto`，避免 `udp counter drop` 触发 FIREWALL_NFT_INVALID
- Google WebRTC STUN 预置规则补上 UDP 端口别名匹配，与媒体规则一致

## 删除

- （无）

## 其他

- （无）
