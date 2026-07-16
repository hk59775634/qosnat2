# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

生产网关 FRR 托管路由自动恢复，并默认禁止 unattended-upgrades 自动升级 systemd/netplan 等关键包。

## 新增

- 路由守护后台：周期性检测内核 FIB 缺失的托管路由，networkd/FRR 扰动后自动重放
- 生产网关 apt lockdown：`configure-gateway-apt.sh` 与默认 deploy 集成，禁止 apt-daily / unattended-upgrades
- qosnatd 启动兜底：尚未配置 apt 限制时自动 lockdown（含仅二进制升级场景）

## 优化

- Boot 路由回放：FRR 模式下等待服务就绪，重试增至 6 次并记录成功/失败日志
- FRR 安装后自动 `PrepareInstalled` + 回放托管路由；主 `frr.conf` 自动 include qosnat2 配置
- `qos-nat.service` 增加 `After=frr.service`，启动等待 12s
- `rebuild.sh` 升级时若未配置 gateway apt 则自动补 lockdown

## 修复

- 修复 FRR 冷启动/升级后托管路由未进内核（主 frr.conf 未引用 qosnat2 include、boot 时序竞争）
- 修复 unattended-upgrades 升级 systemd/netplan 导致 networkd 重启、10.0.0.0/8 等路由丢失

## 删除

- （无）

## 其他

- 卸载脚本移除 qosnat2 apt 配置并恢复 apt 定时器；详见 `docs/SECURITY-HARDENING.md`
