# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

修复版本升级时安装 dnsmasq-chnroutes 因 text file busy 失败、阻断升级的问题。

## 新增

- （无）

## 优化

- 版本切换时 dnsmasq-chnroutes 安装失败仅告警，不再阻断 qosnatd 升级

## 修复

- dnsmasq-chnroutes 安装改为 staging+rename，并在替换前停止服务，避免覆盖运行中的 `/usr/sbin/dnsmasq`（ETXTBSY）

## 删除

- （无）

## 其他

- （无）
