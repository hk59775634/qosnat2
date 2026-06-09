# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

ocserv 组管理支持可选是否写入 select-group 列表，未选中时仅生成 config-per-group 配置文件。

## 新增

- 添加/编辑组时可勾选「加入客户端可选列表（select-group）」；不勾选则仅写 config-per-group，不在 ocserv.conf 列出

## 优化

- 组列表增加「select-group」列，显示是否加入可选列表

## 修复

- （无）

## 删除

- （无）

## 其他

- （无）
