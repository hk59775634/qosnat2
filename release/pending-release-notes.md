# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

一键拦截 Google WebRTC，并加固 WARP 看门狗防抖以避免误拆隧道导致抖动。

## 新增

- 防火墙预置「一键拦截 Google WebRTC」：创建 STUN/媒体别名与 forward 丢弃规则，缓解 VPN+NAT 下 WebRTC 泄漏导致 Gemini 等服务异常（`POST /api/v1/firewall/presets/google-webrtc-block`）

## 优化

- WARP 看门狗改为连续 5 次探测失败（约 100s）才动作；优先 veth 修复与 `warp-cli connect` 软重连，仅在 netns 缺失时才全量重建
- 看门狗不再因瞬时 `NeedsReset` 自动 `ResetBroken` / scrub，降低误判导致的隧道抖动

## 修复

- （无）

## 删除

- （无）

## 其他

- （无）
