# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

修复 WireGuard 修改隧道地址后 apply 不生效（syncconf 未更新 Address）。

## 新增

- （无）

## 优化

- （无）

## 修复

- WireGuard 接口已存在时，若隧道地址与运行态不一致，改为 `wg-quick down/up` 重建接口，使 Address 生效；仅改密钥/peer 时仍走 syncconf 热更新

## 删除

- （无）

## 其他

- （无）
