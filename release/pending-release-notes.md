# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

修复整机重启后 NAT/路由偶发失效；启动 apply 更稳健（nft 优先、后台重试、二次 apply 兜底）。

## 新增

- 静态路由保存/下发时自动推断 `device`（`ip route get`），避免仅填 gateway 时 `Nexthop has invalid gateway`。
- 启动时 `ApplyAllOnBoot()`：首次同步 apply，失败或 nft 表缺失时后台 goroutine 重试（不阻塞 Web UI）。
- nft `Apply()` load 失败时尝试恢复先前 live ruleset，降低 reload 窗口无表风险。

## 优化

- `ApplyAll` 顺序调整为 nft(NAT) 先于 shaper；shaper 失败仅 warn，不再阻断 NAT。
- 无托管 netplan 配置时跳过 `netplan generate/apply`，避免启动打乱系统 default 路由。
- netplan 实际 apply 后 sleep 3s 并二次 `applyManagedRoutesWithRetry()`。
- 启动回放走 `applyNatStackLenient()`：jool/unbound/dnsmasq 失败不 rollback nft。
- `qos-nat.service` 增加 `ExecStartPre=sleep 8`；setup 已完成时 deploy/启动自动 enable 作二次 `apply-state` 兜底。

## 修复

- 修复重启后 NAT 规则未加载或 default 路由被 netplan 清空导致「NAT 失效」的问题。

## 删除

- （无）

## 其他

- （无）
