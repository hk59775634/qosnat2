# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

修复 src+dst（mode=both）出站策略因 main 旁路同向而永不生效，并确保策略型 WanLink 不写入 main default。

## 新增

- （无）

## 优化

- （无）

## 修复

- `mode=both` 的 main 旁路改为回程方向（from/to 对调、不加 iif），并清理历史错误的同向 main 规则
- `PrimaryWanLinkID` / `SyncWanRoutes` 跳过 `policy_only` 策略型 WanLink，避免其写入或抢占主表 default

## 删除

- （无）

## 其他

- （无）
