# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

WARP 显示服务类型并支持 License Key 激活 WARP+；改端口、HTTPS 与版本切换统一密码确认

## 新增

- WARP 通过 Cloudflare `cdn-cgi/trace` 显示服务类型（普通 WARP / WARP+ 等）及注册账户类型
- 启用 WARP 时可填写 WARP+ License Key，连接时自动执行 `registration license`
- 系统设置：改管理端口、切换 HTTPS、切换版本前弹出密码确认（SecurityConfirmModal）

## 优化

- 保存管理端口或 HTTPS 变更后，等待服务就绪并自动跳转到新访问地址
- TLS 状态 API 返回证书 SAN 与建议访问主机名（`cert_hostnames` / `access_host`）
- HTTPS 设置页 ACME 申请/续期改为弹窗确认后再执行；证书管理页续期不再二次确认

## 修复

- （无）

## 删除

- （无）

## 其他

- OpenAPI 补充 WARP `exit_info` 与 `license_key` 请求字段说明
