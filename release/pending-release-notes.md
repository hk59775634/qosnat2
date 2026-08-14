# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

修复 WARP 网关模式回程黑洞，并避免 systemd 私有 mount ns 导致 qwp0 netns 针脚失效。

## 新增

- （无）

## 优化

- （无）

## 修复

- WARP netns：为 `198.18.0.0/30` 安装回程路由/策略规则，避免表 65743 把 qwp0 回复再次送进 CloudflareWARP 造成 SYN-ACK 黑洞
- WARP netns：在 PID 1 mount ns 中创建/删除 `/run/netns` 针脚，避免控制面私有 mount ns 下主机侧看到空文件、`qwp0` 对端失效
- 安装脚本：`qosnatd.service` 默认关闭 `PrivateTmp` / `ProtectHome` / `ProtectControlGroups`，与 WARP netns 要求一致

## 删除

- （无）

## 其他

- （无）
