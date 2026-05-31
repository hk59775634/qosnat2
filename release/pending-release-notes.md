# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

修复 ocserv 1.4.2 源码安装后缺少 systemd 单元导致无法启动的问题

## 新增

- （无）

## 优化

- `install-ocserv.sh` 适配 ocserv 1.4.2 的 standalone systemd 模板路径，找不到上游模板时写入内置 unit

## 修复

- 修复源码安装后 `ocserv.service does not exist`：`Apply()` / 服务启停前自动补全 systemd 单元

## 删除

- （无）

## 其他

- （无）
