# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

QoS 限速完全以用户配置的 profiles 为准，移除 default_profile 隐式限速。

## 新增

- （无）

## 优化

- （无）

## 修复

- 移除 `policy_cidr` + `default_profile` 写入 BPF 的路径；向导不再设置 default_profile 速率
- 加载 state 时自动清除 legacy `default_profile` 速率并持久化
- `default_profile` 单独存在不再触发 QoS「已配置」推断

## 删除

- 删除 `ReplayPolicyCIDRToBPF` 及 profiles 非空时仍写默认网段限速的逻辑

## 其他

- `policy_cidr` 仍用于 NAT/路由语义，不参与 Per-IP 整形
