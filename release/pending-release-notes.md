# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

修正 OCServ RADIUS 地址池策略：ocserv.conf 中 ipv4-network 必填，groupconfig=true 时 RADIUS 属性方可覆盖。

## 新增

- （无）

## 优化

- Web UI 说明 groupconfig 与地址池覆盖关系，移除「地址池留空由 RADIUS 下发」误导提示

## 修复

- 撤销 v2026061506 允许 RADIUS 模式下省略 ipv4-network 的行为；地址池留空时恢复默认 10.250.0.0/24 并始终写入 ocserv.conf
- groupconfig=true 控制 RADIUS Access-Accept（Framed-IP 等）是否覆盖本地地址池，未启用时仅使用本地池

## 删除

- （无）

## 其他

- plain 与 RADIUS 模式统一地址池校验与默认值逻辑
