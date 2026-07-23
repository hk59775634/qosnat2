# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

显著缩短 qosnatd 启动时间：Web 管理面立即可用，dnsmasq/jool/unbound 等外部组件改为后台异步拉起。

## 新增

- （无）

## 优化

- 启动时先监听 HTTP/HTTPS，核心数据面（nft、路由、QoS 等）在后台回放，不再阻塞 Web UI
- dnsmasq、jool、unbound 从同步启动路径移出，由后台 reconcile 检测并按需拉起
- 路由下发跳过尚未创建的出接口（如 WARP qwp0），避免启动阶段空等重试；route guard 后续补发
- FRR 未 active 时启动阶段不再阻塞等待 30s，交由 route guard 处理

## 修复

- （无）

## 删除

- （无）

## 其他

- （无）
