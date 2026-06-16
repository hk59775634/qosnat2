# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

加固 API 密钥权限与敏感信息脱敏，修复 OCServ 虚拟主机与数据面一致性，并支持 RADIUS 模式下 IPv4 地址池留空。

## 新增

- 系统状态导出改为 POST，需管理员会话或 API Key 并校验当前密码
- sysctl 调优与主机名校验，防止写入非法内核参数或主机名
- 进程收到 SIGINT/SIGTERM 时取消后台 ACME/证书续期任务

## 优化

- RADIUS 模式下 IPv4 网段/掩码留空时不写入 ocserv.conf 的 ipv4-network，由 RADIUS 下发客户端 IP
- Web UI 增加 RADIUS 地址池留空说明；WARP/SNMP 密钥不再通过 GET 明文返回
- 出口策略、静态路由、WAN 链路 nftables 应用统一加锁与回滚错误传播

## 修复

- OCServ 禁用虚拟主机不再被规范化逻辑丢弃；全局保存时合并各 vhost 用户密码
- 终端 WebSocket 拒绝非管理员 API Key；状态导入禁止 URL 查询参数传密码
- 出口策略保存前先删旧规则导致配置不一致；路由先落库再写内核
- WARP 许可证保存后「应用」按钮因脱敏无法点击；保存时引用未定义变量

## 删除

- （无）

## 其他

- plain 本地认证仍默认 10.250.0.0/24 地址池；OCServ 默认证书路径改为 /etc/ocserv/certs/
