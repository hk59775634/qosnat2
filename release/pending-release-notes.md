# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

EDT QoS 审计项全量修复：API/UI 对齐、WG/OCServ 重建路径、移除 HTB 遗留误导配置。

## 新增

- Profile CIDR 重叠校验（添加/更新策略时拒绝冲突网段）

## 优化

- `/shaper/active` 返回 `activity_down/up` 与配置速率字段，Dashboard Top 主机排序同步
- `/shaper/tc` 与 Profiles 页仅暴露 EDT 根 `fq` 的 flows/quantum 参数
- 硬件推荐与高级调参改为 `shaper.fq_flows` / `shaper.fq_quantum`（移除无效 leaf/idle_timeout）
- eBPF reload 统一走 `rebuildShaperDataPlane`，避免重复 ReplayState

## 修复

- `rebuildShaperDataPlane` 补全 `applyWGShapers` + `setupOCServShaper`；attach/teardown 设备列表分离
- WG peer 速率仅写 `host_exact`，不再污染 `profiles[]`
- TC egress BPF 挂载失败时回滚已挂 ingress filter
- 禁用 WG/OCServ 实例时正确 detach TC

## 删除

- 删除 HTB 遗留 `internal/shaper/leaf.go` 及无效 idle_timeout GC 路径

## 其他

- OpenAPI、ActiveHosts/Profiles/Dashboard 文案更新为 EDT 语义
