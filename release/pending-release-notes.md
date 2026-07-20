# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

QoS 策略 mask 共享限速（数据面）+ 活跃桶观测（实测速率与成员展开）

## 新增

- 活跃限速桶观测：`host_flow` map；Status 页展示共享池、配置/实测速率，可展开成员主机
- 策略 `mask<32` 时同前缀共享 throttle/token_bucket 限速桶（数据面）

## 优化

- （无）

## 修复

- 策略 `mask` 写入 BPF `rate_val.host_mask`；重建时清空旧桶键
- 仪表盘 Top 主机改为基于字节增量实测速率，不再误用配置 bps 当流量

## 删除

- （无）

## 其他

- 更新 `docs/QOS-DATAPLANE-RFC.md`；升级后若 pinned map 值尺寸不兼容会自动重建
