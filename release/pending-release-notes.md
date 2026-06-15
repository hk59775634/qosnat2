# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

LVS 支持为已有虚拟服务追加或移除 Real Server，无需删除重建。

## 新增

- API `POST/DELETE /api/v1/lvs/virtual-servers/real-servers`：向指定虚拟服务添加/删除后端 RS（支持批量 `real_servers`），保存后自动应用 IPVS 并同步防火墙。
- Web「LVS 负载均衡」列表每行可直接添加 RS、删除单个 RS（至少保留一个）。

## 优化

- （无）

## 修复

- （无）

## 删除

- （无）

## 其他

- （无）
