# API 包结构（internal/api）

`Server` 类型与路由注册仍集中在 `server.go`；按职责拆分的同包文件如下。

| 文件 | 职责 |
|------|------|
| `server.go` | `Env` / `Server` 定义、`New`、`routes`、HTTP 入口（health/login/static/listen） |
| `server_boot.go` | `ApplyAll`、eBPF 回放、`StartBackground` |
| `server_nft.go` | `reloadNft*`、`nftCfg`、WARP 协调、自动防火墙规则同步 |
| `api_response.go` | 统一 JSON 错误 envelope（`error` + `code`） |
| `nft_incremental.go` | 防火墙单条增删改增量与回退 |
| `*_handlers.go` | 各 REST 资源 handler |

同包拆分 **不改变** 导出符号；新增 handler 时优先放入对应 `*_handlers.go`，boot/nft 编排逻辑勿再堆回 `server.go`。
