# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

SNMP 监听 0.0.0.0:161 并开放 IF-MIB，支持 NMS 远程采集 WAN/LAN 接口流量。

## 新增

- SNMP GET 返回 LAN/WAN ifIndex 与 ifHCIn/OutOctets OID 模板（配置页展示）

## 优化

- snmpd 启用时 `agentAddress udp:0.0.0.0:161`；启用后 WAN 防火墙按 allowed_networks 自动放行 UDP
- 服务启动/重启先应用 qosnat2 配置，避免 systemd 使用未托管的默认 snmpd.conf

## 修复

- systemonly 视图补充 IF-MIB（.1.3.6.1.2.1.2 / .31），监控系统可发现接口流量

## 删除

- （无）

## 其他

- 启用 SNMP 时必须填写 allowed_networks（rocommunity 源网段 ACL）
