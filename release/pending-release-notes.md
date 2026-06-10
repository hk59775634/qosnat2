# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

新增 QoS 总开关：关闭后彻底清除运行中 TC/eBPF/ifb 整形，可作为纯 NAT/路由控制器使用。

## 新增

- QoS 策略页「启用 QoS 流量整形」复选框；关闭时卸载 mirred、HTB、BPF 与 ifb0
- API：`GET/PUT /api/v1/shaper/enabled`（`apply: true` 立即切换数据面）
- 全新安装默认 `shaper.enabled: false`（纯 NAT 模式）

## 优化

- QoS 关闭时仍可保存策略配置，再次启用后自动应用
- 升级迁移：旧系统若已有 QoS 规则则自动推断 `enabled: true`

## 修复

- （无）

## 删除

- （无）

## 其他

- `deploy-qos-nat.sh` 初始 state 写入 `enabled: false`
