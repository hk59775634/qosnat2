# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

发布流程要求编写结构化更新说明，并在 Web 版本页展示

## 新增

- 发布前须维护 `release/pending-release-notes.md`，CI 校验后写入 GitHub Release 与 `releases/notes/<版本>.md`
- 版本清单增加 `summary` 字段；版本管理页展示选中版本的更新说明

## 优化

- 新增 `scripts/release-notes.sh`（draft / validate / finalize / reset）与发布策略文档 `release/RELEASE.md`

## 修复

- （无）

## 删除

- （无）

## 其他

- 归档 v2026053106–v2026053111 历史更新说明
