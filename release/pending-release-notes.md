# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

修复 WireGuard 启用并保存后重启不随开机启动的问题。

## 新增

- （无）

## 优化

- （无）

## 修复

- WireGuard：`apply` 启用时同步 `systemctl enable wg-quick@iface`，停用时 `disable`，与 ocserv/dnsmasq 一致
- 启动回放：`ApplyAllOnBoot` 对 state 中已启用的 WireGuard 实例执行 apply，避免仅运行时 up、未 enable 时重启后接口缺失

## 删除

- （无）

## 其他

- 已启用实例请重新「保存」一次（或升级后重启 qosnatd）以写入 systemd enable；之后重启主机应自动拉起对应 wg 接口
