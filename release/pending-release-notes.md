# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

开启 QoS 且未添加 profile 时不再隐式写入 policy_cidr 限速；medium 档默认开启 WAN RPS。

## 新增

- （无）

## 优化

- NAT 网关 medium 性能档默认启用 `rps_wan`，与 multiqueue 场景下 LAN RPS 对齐

## 修复

- profiles 为空时跳过 `policy_cidr` + `default_profile` 写入 BPF，避免「未配策略仍被 8mbit 限速」
- 重新 ReplayState 会 flush 旧 profile_lpm，升级/重开 QoS 后清除遗留隐式规则

## 删除

- （无）

## 其他

- 添加 profile 后，`policy_cidr` + `default_profile` 仍作网段内 Per-IP 默认速率（行为不变）
