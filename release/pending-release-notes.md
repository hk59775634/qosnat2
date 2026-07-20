# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

支持经 sing-box TUN 接入 HTTP/HTTPS/SOCKS5 独立 IP 代理出口，并与 WanLink/出站策略联动

## 新增

- ProxyEgress：配置 SOCKS5 / HTTP / HTTPS 代理，安装固定版本 sing-box，启动 `qpe*` TUN 并自动创建 `policy_only` 托管 WanLink
- API：`/api/v1/network/proxy-egress`（CRUD / install / connect / disconnect / status / task）
- Web：WanLinks 新增「独立 IP 代理」页签，支持安装、添加、连接/断开与出口 IP 展示
- nft 放行 `qpe*` 转发/入站；boot 回放与 watchdog 对已启用代理保活重连

## 优化

- （无）

## 修复

- （无）

## 删除

- （无）

## 其他

- 出站策略绑定代理 WanLink 后，匹配流量经策略路由进入 TUN，由代理服务商提供独立出口 IP（本机对代理口 MASQUERADE）
