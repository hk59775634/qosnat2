# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

新增 LVS（IPVS）四层负载均衡：虚拟服务 VIP → Real Server，支持 NAT/DR 模式。

## 新增

- `state.json` 字段 `lvs`：虚拟服务、调度算法、会话保持、Real Server 权重
- API：`/api/v1/lvs`、apply、install、virtual-servers CRUD
- Web：**安全 / NAT → LVS 负载均衡** 配置页（安装 ipvsadm、添加 VS/RS、保存并应用）
- 启动回放时自动 apply IPVS；可选 Auto VIP 绑定 WAN/32

## 优化

- 添加 VS 时检测与 WAN 端口转发 VIP:port 冲突

## 修复

- （无）

## 删除

- （无）

## 其他

- NAT 模式：Real Server 默认网关需指向本机；DR 模式需 RS 侧 lo 绑 VIP 与 ARP 配置
