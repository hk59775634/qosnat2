# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

ocserv 源码安装增加 Route B TunnelGroupName 补丁，并修复一键安装时 git fetch 标签冲突。

## 新增

- ocserv 编译安装自动应用 `patches/ocserv/0001-radius-tunnel-group-name.patch`（URL 选组写入 RADIUS VSA 146）
- radcli 字典增加 Cisco `TunnelGroupName`（VSA 146）属性

## 优化

- （无）

## 修复

- 一键安装 `install.sh`：已有 git 仓库时 `git fetch --tags` 因本地 tag 与远端不一致失败（would clobber existing tag）

## 删除

- （无）

## 其他

- 补丁源自 [ocserv-tunnel](https://github.com/hk59775634/ocserv-tunnel) SPEC-01，并补全 sec-mod 组名传递
