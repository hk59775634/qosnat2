# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

qosnatd 新增 CLI 子命令：查看状态、修改端口/用户名/密码，以及 `-h`/`--help` 帮助。

## 新增

- `qosnatd status`：输出管理 URL、用户名、密码与监听端口状态
- `qosnatd set-port <port>`：修改管理端口，同步 env/nft 并重启服务
- `qosnatd set-user <username>`：修改管理员用户名并重启服务
- `qosnatd set-password <password>`：修改管理员密码（bcrypt 持久化）并重启服务
- `qosnatd -h` / `--help` / `help`：列出支持的 CLI 命令

## 优化

- （无）

## 修复

- （无）

## 删除

- （无）

## 其他

- （无）
