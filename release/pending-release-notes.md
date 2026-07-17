# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

修复 ocserv 源码安装在 Ubuntu 24.04 / libradcli 下因 `PW_TUNNELGROUPNAME` 未定义导致编译失败。

## 新增

- （无）

## 优化

- （无）

## 修复

- ocserv Route B 补丁补充定义 Cisco ASA TunnelGroupName（vendor 3076 / VSA 146），兼容系统 radcli 头文件

## 删除

- （无）

## 其他

- （无）
