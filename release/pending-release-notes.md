# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

修复 ocserv 完整 SPEC-01 安装，并支持出站策略「仅路由 / 不 SNAT」接入机模式。

## 新增

- 内置 `patches/ocserv/apply-spec01-edits.py`（完整 SPEC-01：worker group-access、radius_auth_bind_group、acct VSA）
- 出站策略 `no_snat`：源网段仅策略路由到 WanLink 网关（远端 NAT），本机不做 SNAT

## 优化

- `install-ocserv.sh` 固定 1.4.2 基线，显式安装并校验 `ocserv` + `ocserv-worker`
- 多 WAN 页支持勾选「仅路由 / 不 SNAT」

## 修复

- 不再使用早期残缺 `0001-radius-tunnel-group-name.patch`（会漏掉 OpenConnect `<group-access>` 与 bind_group 链路）
- `no_snat` 流量跳过 catch-all masquerade，且不进入非对称回程 drop

## 删除

- （无）

## 其他

- （无）
