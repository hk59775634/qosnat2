# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

LVS 支持 Director / Real Server 角色切换，RS 模式可通过 Web 自动配置 lo VIP、ARP 抑制与 LAN 防火墙放行。

## 新增

- LVS 设备角色：`director`（入口）与 `rs`（DR 后端）；RS 模式在 `lo` 绑定 VIP/32，写入 `arp_ignore=1`、`arp_announce=2`
- RS 模式 Web 管理 VIP:端口绑定列表，保存并应用后同步防火墙 LAN input 规则
- API 返回 `rs_status`（lo VIP 与 ARP 参数摘要）

## 优化

- Director 与 RS 角色互斥：虚拟服务、OCServ 集群、追加 RS 等操作仅 Director 可用

## 修复

- （无）

## 删除

- （无）

## 其他

- 旧配置无 `role` 字段时默认为 `director`，行为与此前版本兼容
