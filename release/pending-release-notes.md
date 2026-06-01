# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

版本切换支持选择下载线路（直连、GitHub 代理、多 WAN 出口）；版本管理页布局与 rebuild 脚本改进。

## 新增

- 系统 → 版本管理：切换 release 时可选择**下载线路**（直连、v4.gh-proxy.org、cdn.gh-proxy.org、多 WAN 出口 1/2）；多 WAN 线路在下载期间临时按目标地址走对应出口，完成后自动清理。

## 优化

- 版本管理页将「下载线路」与「切换到版本」拆为两行全宽下拉，操作区单独一行，避免挤在同一行。
- `rebuild.sh`：健康检查从 `/etc/qosnat2/env` 读取 `ADMIN_PORT` 并支持 HTTPS；支持 `RELEASE=1` 与 `SKIP_WEB` / `SKIP_BPF` 快捷选项。

## 修复

- （无）

## 删除

- （无）

## 其他

- OpenAPI 与版本切换 API 增加 `download_route`、`download_routes` 字段说明。
