# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

ocserv 源码安装体验优化；自建 GitHub 镜像每日同步与 gh-proxy 加速下载

## 新增

- GitHub Actions `mirror-ocserv`：每 24 小时将 GitLab 官方 ocserv 同步至 `hk59775634/ocserv`

## 优化

- `install-ocserv.sh` 优先从自建镜像经 v4.gh-proxy 下载，GitLab/infradead 直连作回退
- ocserv 安装提示合并为一条；安装任务 running 时锁定「从源码安装」按钮

## 修复

- ocserv 安装进行中重复点击返回 409 时恢复轮询而非报错

## 删除

- （无）

## 其他

- 首次同步需在 qosnat2 配置 Secret `OCSERV_MIRROR_TOKEN` 并手动运行 mirror-ocserv
