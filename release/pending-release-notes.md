# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

修复 EDT 模式启用 QoS 时 fq qdisc 安装失败与 ListHosts 空指针崩溃。

## 新增

- （无）

## 优化

- （无）

## 修复

- `SetupEDTDevice` 误用 `tc fq codel`，改为 plain `fq`（EDT 数据面要求）
- EDT 模式下 `ListHosts`/`ListProfiles`/`ListActive`/`DeleteHost`/`PurgeActive`/`EachClassid`/`lookupRatesLocked` 访问 HTB map 导致 panic

## 删除

- （无）

## 其他

- 升级后请确认 `/usr/lib/qosnat2/rate_edt.bpf.o` 存在（版本切换会自动安装）
