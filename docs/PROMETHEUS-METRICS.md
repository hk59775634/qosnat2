# Prometheus 指标与告警

qosnatd 暴露运维指标，供 Prometheus / Grafana 采集。

## 端点

| 路径 | 格式 | 鉴权 |
|------|------|------|
| `GET /api/v1/metrics/prometheus` | Prometheus text exposition 0.0.4 | Session 或 `X-API-Key` |
| `GET /api/v1/metrics/ops` | JSON（Dashboard 同源） | 同上 |

## 指标一览

| 名称 | 类型 | 说明 |
|------|------|------|
| `qosnat_conntrack_count` | gauge | 当前 conntrack 条目数 |
| `qosnat_conntrack_max` | gauge | `/proc/sys/net/netfilter/nf_conntrack_max` |
| `qosnat_cpu_percent` | gauge | 主机 CPU 利用率（%） |
| `qosnat_mem_percent` | gauge | 主机内存利用率（%） |
| `qosnat_nft_reload_total` | counter | nft 全表/增量 reload 累计次数 |
| `qosnat_nft_reload_last_ms` | gauge | 最近一次 reload 耗时（毫秒） |
| `qosnat_nat_stack_apply_total` | counter | NAT64/NPTv6 栈 apply 累计次数 |
| `qosnat_nat_stack_apply_last_ms` | gauge | 最近一次 NAT stack apply 耗时（毫秒） |

JSON 端点另含 `nft_reload.last_error`、`nat_stack_apply.last_error` 等字段，适合 UI 展示。

## scrape 配置示例

```yaml
scrape_configs:
  - job_name: qosnat2
    scheme: https          # 或 http
    tls_config:
      insecure_skip_verify: true   # 自签证书时；生产请用正确 CA
    metrics_path: /api/v1/metrics/prometheus
    static_configs:
      - targets: ['gateway.example.com:8080']
    authorization:
      credentials: 'YOUR_API_KEY'   # 或改用 bearer / basic 由反向代理注入
```

若 Prometheus 无法携带 Cookie，请使用 **只读或 admin API Key**（`authorization.credentials` 对应请求头 `X-API-Key` 的值；部分 Prometheus 版本需 `authorization.type: Bearer` 并在代理层转换）。

## 告警规则示例

```yaml
groups:
  - name: qosnat2
    rules:
      - alert: QosnatConntrackHigh
        expr: qosnat_conntrack_count / qosnat_conntrack_max > 0.85
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "conntrack 使用率 > 85%"

      - alert: QosnatNftReloadFailed
        expr: increase(qosnat_nft_reload_total[10m]) > 0 and qosnat_nft_reload_last_ms == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "nft reload 计数增加但 last_ms 为 0（检查 ops JSON last_error）"

      - alert: QosnatNatStackSlow
        expr: qosnat_nat_stack_apply_last_ms > 30000
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "NAT stack apply 超过 30s"

      - alert: QosnatHostMemoryHigh
        expr: qosnat_mem_percent > 90
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "qosnat 主机内存 > 90%"
```

`QosnatNftReloadFailed` 为启发式规则；更可靠做法是定期抓取 `/api/v1/metrics/ops` 并对 `nft_reload.last_error != ""` 做 blackbox 检查，或后续在 Prometheus 中暴露 `last_error` 计数器。

## 防火墙增量 reload（可选）

设置环境变量 `QOSNAT_NFT_INCREMENTAL=1` 后，防火墙规则 **单条增删** 在通过全表 `nft -c` 预检后，可尝试 `nft add rule` / 按 `qosnat2:rid:<id>` 删除，失败自动回退全表 reload。规则文件 `/etc/qosnat2/nftables-qosnat.nft` 仍会全量重写以保持重启一致。

## 相关文档

- [API-ZH.md](./API-ZH.md) — REST 指标端点
- [API-AUTH.md](./API-AUTH.md) — API Key 角色
- [SECURITY-HARDENING.md](./SECURITY-HARDENING.md) — 生产加固
