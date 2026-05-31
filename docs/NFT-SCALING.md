# nft 规模化与增量应用

大规模防火墙规则（数百条以上）时，每次 `nft -f` 全表 reload 会造成短暂 dataplane 抖动。可通过环境变量启用 **单条增量** 路径。

## 启用

在 `/etc/qosnat2/env` 或 systemd 单元中设置：

```bash
QOSNAT_NFT_INCREMENTAL=1
```

接受值：`1` / `true` / `yes`（不区分大小写）。

## 行为

| 操作 | 增量路径 | 失败回退 |
|------|----------|----------|
| POST 防火墙规则 | `nft add rule` | 全表 reload |
| PATCH 防火墙规则 | 同脚本内 `delete handle` + `add rule` | 全表 reload |
| DELETE 防火墙规则 | `nft delete rule handle` | 全表 reload |
| 规则排序 PUT | 全表 reload | — |
| NAT / 端口转发 / 别名等 | 全表 reload | — |

增量仅适用于 `forward` / `input` 链上带 `qosnat2:rid:<id>` 注释的用户规则。成功后仍会 **重写 rules 文件** 与 state 中的 auto 规则，保持磁盘与内核一致。

## 观测

`GET /api/v1/ops/metrics` 中 `nft_reload` 块记录最近一次 reload 耗时与错误；增量成功时通常不记入全表 reload 指标。

## 限制

- 需要 `nft` CLI 与 live ruleset 中已有对应 handle。
- 不支持跨链移动规则（PATCH 若改 chain 仍可能回退全表 reload）。
- 与 `nftApplyMu` 串行化，避免并发增量与全表 apply 交错。

详见 `docs/PROMETHEUS-METRICS.md` 与 `internal/nft/incremental.go`。
