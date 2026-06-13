# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

LVS 支持 UDP 与 TCP+UDP 协议，新增 OpenConnect 内网多节点集群一键配置及 VIP 防火墙自动放行。

## 新增

- 虚拟服务协议 `tcp_udp`（同 VIP:port 同时创建 TCP/UDP IPVS 规则）
- API `POST /api/v1/lvs/ocserv-cluster`：内网多台 ocserv 集群（默认 443、会话保持 3600s、调度 sh）
- Web LVS 页「OpenConnect 集群」快捷表单；手动添加 VS 可选 TCP + UDP
- LVS 启用时 WAN input 自动放行 VIP:port（`auto-input-lvs-*`）

## 优化

- 应用 LVS 时同步防火墙规则并重载 nft
- 检测本机 OCServ 与 LVS 集群端口冲突并提示/拒绝

## 修复

- （无）

## 删除

- （无）

## 其他

- NAT 模式下各 ocserv Real Server 默认网关须指向本机 Director
