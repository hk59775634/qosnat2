# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

修复 LVS 安装 ipvsadm 时因缺少 linux-modules-extra 导致 modprobe ip_vs 失败的问题。

## 新增

- （无）

## 优化

- 按当前运行内核检查 ip_vs.ko 是否存在，再决定是否 apt 安装 linux-modules-extra

## 修复

- 安装 ipvsadm 前自动 apt update 并安装 linux-modules-extra-$(uname -r)，安装后校验模块文件
- modprobe 失败时提示需安装的包名及内核升级后可能需要重启

## 删除

- （无）

## 其他

- （无）
