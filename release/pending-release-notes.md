# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

修复 FRR vtysh 应用失败与静态路由误推断 lo 出接口的问题。

## 新增

- （无）

## 优化

- 托管路由配置文件改为 FRR 原生 include 语法（不含 configure terminal）

## 修复

- FRR 配置应用改为 vtysh stdin 批处理，不再误用 `vtysh -f` 导致 Unknown command
- 动态路由（BGP/OSPF）应用同样改为 stdin 方式
- 仅填网关时不再将 `ip route get` 推断出的 lo 写入路由出接口

## 删除

- （无）

## 其他

- （无）
