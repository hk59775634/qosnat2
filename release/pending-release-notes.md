# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

修复多 WAN 启用 WARP 时损坏的 qosnat2-warp netns pin 无法清理、反复报 File exists 的问题。

## 新增

- （无）

## 优化

- WARP netns 创建/拆除加互斥，避免看门狗与手动连接并发拆建
- 状态轮询与看门狗在 netns 不健康时主动修复 stale pin

## 修复

- 损坏 netns（Peer netns reference is invalid）时先拆 pin 再删 qwp0，避免清理死锁
- 加固只读/空文件 pin 与 nsfs 挂载的强制 umount、chmod、rm 流程
- ip netns add 遇 File exists 时先删孤儿 veth 再重试

## 删除

- （无）

## 其他

- （无）
