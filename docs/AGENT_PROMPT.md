# 角色
你是 Linux 内核网络、eBPF（TC clsact）、tc HTB/fq、IFB、nftables 与 Go 后端专家。请在本地仓库 **/opt/qosnat2** 从零实现 **qosnat2** 项目，不要回到旧 qosnat 的 netns/flowtable 架构。

# 必读文档（动手前先完整阅读）
- /opt/qosnat2/单机双网卡-QoS-NAT-开发说明.md（唯一架构准则，含 API↔eBPF 规范、pfSense UI 蓝图、开发阶段 §13、自检 §17）
- /opt/qosnat2/README.md
- /opt/qosnat2/reference/ 仅作对照，禁止照搬部署

# 已确认架构（不得擅自改回旧方案）
## 拓扑
- 单机双网卡：宿主机 DEV_LAN（内网）+ DEV_WAN（外网），**禁止** netns、ipvlan、veth、WAN 移入 netns、nft flowtable fastpath。

## QoS（核心）
- **数据面**：Per-IP Shaping = TC clsact BPF 分类 + **HTB** + 叶子 **fq_codel**（默认）+ **IFB(ifb0)** 做上行整形。
  - 下行（客户下载）：LAN **egress**，按 ip->daddr /32 选 HTB class。
  - 上行（客户上传）：LAN **ingress**，按 ip->saddr /32，**mirred** 到 ifb0 再 HTB。
- **禁止** 使用旧版 ratelimit.bpf.c 的令牌桶 + **TC_ACT_SHOT**（Policing）；必须 Shaping（排队）。
- **控制面**：Go 服务 **qosnatd**，用 **github.com/cilium/ebpf** 统一管理 BPF 生命周期与 bpffs Map。

## eBPF Map（控制面真相源，API 必须操作 Map）
- profile_lpm（LPM trie）：网段模板，如 10.0.0.0/8 → 默认 8mbit
- host_exact（hash）：/32 单 IP 覆盖（首包或 profile `/32`），最长匹配优先于 profile
- active_host（LRU hash）：活跃主机，供状态页 **Iterate** 导出
- 速率 Value：**字节/秒**（API 接收 mbit 后换算 bps = mbit * 125000）

## REST API 强制行为（§7）
- 添加/更新限速：必须 bpf_map_update_elem + 同步 netlink 更新 HTB（LAN + ifb）
- 删除：必须 bpf_map_delete_elem + tc class del
- 状态页：必须 Map.Iterate(active_host) 返回 JSON
- NAT 与 QoS：skb->tc_classid 仅给 HTB；nft 仅用 mark 低 30 位；IFB 用 mirred，**不要**用 mark 做 IFB 分流

## NAT
- 宿主机 nftables 表 inet qosnat：SNAT 池、1:1、prefix、端口转发；forward 非对称回程 drop（公网源直达 LAN 上 10.x 丢弃）

## 前端（后续阶段，先定 API）
- pfSense 风格：Vue3 + Vite + Tailwind，菜单见开发说明 §10
- 本期若时间紧，可先完成后端 + OpenAPI + 最小静态页，但须预留 /api/v1/ 契约

# 目标仓库结构（请按此创建，勿沿用旧 nat-admin 单文件结构）
qosnat2/
├── cmd/qosnatd/main.go
├── internal/{ebpf,shaper,nft,store,api,sysctl}/
├── bpf/classify.bpf.c
├── api/openapi.yaml
├── deploy-qos-nat.sh
├── web/                    # P4 再深做
├── docs/单机双网卡-QoS-NAT-开发说明.md
└── README.md

# 可从 reference/ 迁移的思路（需重写而非复制）
- /opt/qosnat2/reference/legacy/nat-admin/、nat-qos-bpf/：仅参考 NAT state 模型、鉴权；**不要**复制 netns、flowtable、policer SHOT 逻辑

# 本阶段交付（P0，必须可验收）
1. 初始化 Go module，创建 cmd/qosnatd 与 internal 包骨架。
2. 编写 api/openapi.yaml（/api/v1/shaper/*、/api/v1/nat/*、/api/v1/stats、鉴权）。
3. deploy-qos-nat.sh：sysctl、加载 nft 基础 SNAT、创建 ifb0、HTB 根、clsact 占位、安装 qosnatd；**readlink -f** 安装到 /usr/local/bin/。
4. nft：policy_routes 驱动 SNAT；forward 非对称回程 drop；**无 flowtable**。
5. state.json 持久化路径 /var/lib/qosnat2/state.json，启动时回放。
6. 提供 README 开发构建说明（go build、make bpf、systemd 单元）。
7. 所有变更仅在 /opt/qosnat2；提交前自测：内网 10.x 经 NAT 能访问公网（可用最小环境说明）。

# 后续阶段顺序（不要跳步乱做）
P1 → bpf Pin + Map CRUD API
P2 → classify.bpf + 动态 HTB 建类 + ringbuf
P3 → Iterate active_host + Dashboard API
P4 → Vue 流量整形/防火墙页
P5 → mark 隔离与 RSS 监控
P6 ✅ WireGuard + tcpdump 抓包

# 编码约束
- Go 1.22+，错误处理完整，避免过度抽象。
- 不写用户未要求的超大功能；P0 聚焦可运行最小闭环。
- 网卡名必须环境变量 DEV_LAN/DEV_WAN，禁止写死 ens18/ens19。
- 前端 onclick 若用 JSON.stringify 嵌入 HTML，外层用单引号：onclick='fn(...)'。
- 不要创建 git commit，除非我明确要求。

# 验收标准（P0 完成时汇报）
- [ ] `go build ./cmd/qosnatd` 通过
- [ ] `deploy-qos-nat.sh start` 无 netns，WAN 在宿主机
- [ ] nft list ruleset 有 SNAT + forward drop 非对称规则
- [ ] curl http://<LAN>:8080/api/v1/health 或等价健康检查
- [ ] 文档列出 P0 已知限制与 P1 待办

请先阅读开发说明全文，给出 5 行以内的实施计划，然后直接在 /opt/qosnat2 创建文件并编码实现 P0。