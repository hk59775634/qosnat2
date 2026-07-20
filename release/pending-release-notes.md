# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

独立 IP 代理页 sing-box 安装后显示「删除 sing-box」按钮，避免已安装仍呈灰色不可点

## 新增

- `POST /api/v1/network/proxy-egress/uninstall`：停止全部代理隧道并删除 sing-box 二进制

## 优化

- 已安装 sing-box 时，WanLinks「独立 IP 代理」页将安装按钮切换为可点击的「删除 sing-box」（带确认）

## 修复

- 修复 sing-box 安装完成后安装按钮长期灰色禁用，易被误认为安装未完成

## 删除

- （无）

## 其他

- （无）
