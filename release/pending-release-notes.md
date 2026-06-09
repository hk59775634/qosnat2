# 待发版更新说明

> 发布前请编辑本文件：按 **新增 / 优化 / 修复 / 删除 / 其他** 梳理变动；CI 将据此生成 GitHub Release 说明并归档。
> 完成后与功能代码一并提交；发布成功后本文件会自动重置为模板。

## 概要

修复高负载长时间运行后内网 ping 高延迟/丢包：排除内网流量 IFB 误整形、对齐 ifb0 队列深度，并降低 HTB 同步风暴。

## 新增

- （无）

## 优化

- ifb0 txqueuelen 与 LAN 对齐（SetupP0 / 系统调优应用时设置，默认 5000）
- HTB 同步改为增量补建（每轮最多 64 个缺失类），EnsureHost 幂等跳过已安装类
- GC 不再每轮对所有 active_host 执行 EnsureHost repair

## 修复

- LAN ingress mirred 增加 prio 5「dst 在策略网段 → action ok」规则，ping 回复与内网互访不再导入 ifb0
- ensureClass 在类已存在时不再反复删建 fq_codel leaf，缓解 `htb: too many events!`

## 删除

- （无）

## 其他

- 验收脚本增加 prio 5 local skip 检查
