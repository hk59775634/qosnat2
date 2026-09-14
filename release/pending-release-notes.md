# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

按接口增加「长肥链路」开关：专线口 fq 队列与 MSS clamp，任一接口开启则整机 BBR 与大 TCP 窗口。

## 新增

- 网络接口 `lfn_enabled` / `lfn_mss_clamp`：无整形的上联口改为 mq+fq（或单队列 fq）并加大 txqueuelen；转发 TCP SYN 做 MSS clamp（默认 1280）。
- 任一接口开启时长肥 sysctl 写入 `99-qosnat2.conf`（BBR、fq、64MB 窗口、MTU probing）并加载 `tcp_bbr`；最后一个口关闭后恢复 catalog 默认。已有 clsact/HTB 的口不 replace 根队列，只下发 MSS 并告警。

## 优化

- （无）

## 修复

- （无）

## 删除

- （无）

## 其他

- 不要在已做 Per-IP 整形的 LAN 上开启。VXLAN 不是 WAN 加速。过路用户 TCP 仍是客户端拥塞控制；本机作端点才会接近 BBR 结果。发版后可淘汰现场旁路 `99-z-lfn-bbr.conf` 与独立 `sslvpn-mssclamp.service`。
