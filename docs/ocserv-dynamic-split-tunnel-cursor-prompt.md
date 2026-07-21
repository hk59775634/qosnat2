# Cursor 提示词：ocserv 1.5.0 + SPEC-01 + 动态拆分隧道

> 用途：在新仓库（如 `ocserv-tunnel`）中，以 **上游 ocserv 1.5.0** 为基线，**先移植并集成 qosnat2 的 SPEC-01（Route B）补丁**，再实现 Cisco ASAv/AnyConnect 兼容的**按域名动态拆分隧道（DST / SPEC-02）**。  
> 用法：将下方「--- 复制起点 ---」到「--- 复制终点 ---」整段粘贴到新项目的 Cursor 首条消息，或保存为 `.cursor/rules/ocserv-dst.md` / `AGENTS.md`。

---

## --- 复制起点 ---

你是资深 C / VPN 协议工程师。本项目目标是在 **ocserv 1.5.0** 上交付一套可生产安装的补丁栈，供 **qosnat2** 回灌使用：

1. **SPEC-01（必须先完成）**：Route B / TunnelGroupName（从 qosnat2 现有 1.4.2 补丁**移植**到 1.5.0）
2. **SPEC-02（本项目主功能）**：Cisco AnyConnect 兼容的动态拆分隧道（Dynamic Split Tunneling）

二者必须共存；安装产物必须同时包含打了补丁的 **`ocserv` + `ocserv-worker`**。

---

### 上游基线（强制）

- **官方 tag：`1.5.0`**（`openconnect/ocserv` / GitLab `gitlab.com/openconnect/ocserv.git`）
- **禁止**默认使用 1.4.2 作为新项目基线（qosnat2 历史基线是 1.4.2，本项目负责升级到 1.5.0）
- 构建：Meson + Ninja；安装前缀 `/usr/local`；配置目录 `/etc/ocserv`
- 必须启用 RADIUS（`libradcli`）以便验证 SPEC-01

```bash
git clone --depth 1 --branch 1.5.0 https://gitlab.com/openconnect/ocserv.git ocserv-src
# 或等价镜像 / 完整 clone 后 checkout 1.5.0
```

---

### SPEC-01：Route B / TunnelGroupName（必须集成）

#### 业务背景（qosnat2 生产需求）

VPN 平台 Route B 要求：客户端用**短用户名**（如 `test333`）连接

```text
https://{pop}/{tunnel_group}    # 例：https://sg.example.com/demoagent1-sg
```

ocserv **必须**在 RADIUS Access-Request（及 Accounting）中发送 Cisco-ASA VSA：

| 项 | 值 |
|----|-----|
| Vendor | 3076（Cisco） |
| VSA | **TunnelGroupName = 146** |
| 内容 | URL 路径选出的 tunnel group 名（如 `demoagent1-sg`） |

平台 FreeRADIUS / api-go 依赖该属性解析租户接入点。缺省会导致：

- 日志：`radius-auth: no selected_group; TunnelGroupName omitted for user '...'`
- Access-Request 无 TunnelGroupName → 平台 `/api/radius/auth` 返回 400
- Legacy `app_id#user` 仍可登录（不依赖 TunnelGroupName），但 Route B 失败

#### 参考实现来源（必须 vendored / 移植，不要从零猜）

从 **qosnat2** 仓库拷贝并适配：

| 文件 | 作用 |
|------|------|
| `patches/ocserv/apply-spec01-edits.py` | 对干净源码树做确定性编辑的权威脚本 |
| `patches/ocserv/README.md` | 覆盖范围说明 |
| `scripts/install-ocserv.sh` 中 `apply_spec01` / `install_binaries` / `verify_spec01_binaries` | 安装与校验契约 |

**不要**使用早期残缺的 `*.patch` / `*.legacy-incomplete`。

#### SPEC-01 完整链路（缺一不可）

| 区域 | 作用 |
|------|------|
| `worker-auth.c` | 解析 URL / OpenConnect `<group-access>`；`parse_group_access_url`；`auth_cont` 携带组名 |
| `sec-mod-auth.c` | 密码阶段前 `radius_auth_bind_group`；持久化 `req_group_name` |
| `auth/radius.c` | Access-Request 发送 TunnelGroupName（VSA 146） |
| `acct/radius.c` | Accounting 发送 TunnelGroupName |
| `ipc.proto` | `sec_auth_cont_msg.group_name`（及关联字段） |
| `sec-mod.h` 等 | `groupname` / `selected_group` / `req_group_name` 结构体字段 |

OpenConnect 客户端在 init 阶段会发：

```xml
<group-access>https://host/tunnel_group</group-access>
```

worker 必须解析该标签（不仅是 HTTP path），并把组名经 sec-mod IPC 传到 RADIUS 模块。

#### 移植到 1.5.0 的硬性要求

qosnat2 现有 `apply-spec01-edits.py` 是针对 **1.4.2** 锚点编写的。你必须：

1. 在 **干净 1.5.0 树** 上尝试应用脚本；
2. 若锚点失败：对照 1.4.2 ↔ 1.5.0 diff，**更新脚本锚点与插入文本**，保持语义等价；
3. 脚本须 **idempotent**（重复执行不破坏）；
4. 更新脚本文件头注释为 `ocserv 1.5.0`；
5. 不得削弱功能（worker + main + auth + acct + ipc 全链路仍在）。

#### SPEC-01 安装与校验契约（对齐 qosnat2）

编译安装后**必须同时**安装：

- `/usr/local/sbin/ocserv`
- `/usr/local/sbin/ocserv-worker`

（isolate-workers 场景下，只更新主程序、留下旧 worker 会导致 Route B 残缺。）

安装后二进制校验（失败则拒绝交付）：

| 二进制 | 必须包含的字符串 |
|--------|------------------|
| `ocserv` | `radius_auth_bind_group` |
| `ocserv` | `TunnelGroupName` |
| `ocserv-worker` | `parse_group_access_url` |
| `ocserv-worker` | `<group-access>` |

建议提供：

```bash
tests/verify-spec01-symbols.sh   # 对 ocserv + ocserv-worker 做上述 strings 检查
```

#### SPEC-01 功能回归（必须可测）

1. OpenConnect / AnyConnect 连接 `https://host/demoagent1-sg`，短用户名登录；
2. RADIUS Access-Request 含 TunnelGroupName=`demoagent1-sg`；
3. Accounting 同样携带（若启用 acct）；
4. 无组路径时行为明确（省略或空），不得崩溃。

---

### SPEC-02：动态拆分隧道（DST）

#### 目标

在 ocserv 服务端**原生支持**（不依赖手工拼 `custom-header`）：

1. **Dynamic Split Include**：匹配域名的解析 IP 由客户端动态加入 VPN 隧道路由；
2. **Dynamic Split Exclude**（可选第二阶段）：tunnel-all 下匹配域名走本地；
3. **Enhanced DST**（可选）：同时配置 include + exclude（AnyConnect ≥ 4.6）。

#### ASAv 机制（必须理解）

DST **不是**服务端按域名转发，而是：

1. 服务端认证成功后，通过 AnyConnect 私有 XML 下发域名列表；
2. **客户端**（Cisco AnyConnect / Secure Client ≥ 4.6）监听 DNS，命中后动态改路由表；
3. 主要验证目标：**Win/macOS**；Linux 官方 AnyConnect 支持有限；开源 OpenConnect **不要求**支持 DST 路由。

已知社区 workaround（ocserv#352）：注入 `X-CSTP-Post-Auth-XML`。  
**关键**：AnyConnect **忽略**主认证 XML 里的 `dynamic-split-*`，必须放在 **`X-CSTP-Post-Auth-XML`**。

```conf
custom-header = X-CSTP-Post-Auth-XML: <?xml version="1.0" encoding="UTF-8"?><config-auth client="vpn" type="complete" aggregate-auth-version="2"><config client="vpn" type="private"><opaque is-for="vpn-client"><custom-attr><dynamic-split-include-domains><![CDATA[corp.example.com,app.example.com]]></dynamic-split-include-domains></custom-attr></opaque></config></config-auth>
```

本项目要把上述能力变成正式配置项，由服务端自动生成该头。

#### 非目标

- 不做服务端「DNS → ipset → 策略路由」；
- 不要求开源 OpenConnect 客户端动态改路由；
- **不得改变或削弱 SPEC-01 行为**。

#### 配置语法（全局 / per-group / per-user）

```conf
dynamic-split-include-domains = corp.example.com, app.example.com
# dynamic-split-exclude-domains = www.cisco.com, tools.cisco.com   # Phase 2

# 与静态拆分配合（已有）：
# route = 10.0.0.0/255.0.0.0
# no-route = 192.168.0.0/255.255.0.0
# split-dns = corp.example.com
```

| split 策略 | include | exclude | 预期 |
|------------|---------|---------|------|
| split-include（仅 `route`，无 default） | 有 | 无 | 匹配域名 + 静态 route 走隧道 |
| tunnel-all（`route = default`） | 无 | 有 | 默认全隧道，exclude 走本地 |
| split-include | 有 | 有 | Enhanced（Cisco 4.6+） |

文档中明确：如何从 `route`/`no-route` 推断策略，或是否需要显式 `split-tunnel-policy`。

#### Post-Auth XML 结构

```xml
<?xml version="1.0" encoding="UTF-8"?>
<config-auth client="vpn" type="complete" aggregate-auth-version="2">
  <config client="vpn" type="private">
    <opaque is-for="vpn-client">
      <custom-attr>
        <dynamic-split-include-domains><![CDATA[domain1.com,domain2.com]]></dynamic-split-include-domains>
      </custom-attr>
    </opaque>
  </config>
</config-auth>
```

- 域名去空格、逗号分隔；尾逗号是否保留以 AnyConnect 抓包为准；
- 若已有 `custom-header = X-CSTP-Post-Auth-XML`，**合并** opaque/custom-attr，勿覆盖；
- 作用域合并：全局 → group → user；域名去重保序。

---

### 补丁应用顺序（强制）

```text
干净 ocserv 1.5.0 源码树
        │
        ▼
python3 scripts/apply-spec01-edits.py ocserv-src   # SPEC-01（必须）
        │
        ▼
python3 scripts/apply-dst-edits.py ocserv-src      # SPEC-02 DST
        │
        ▼
meson setup build --prefix=/usr/local && ninja -C build
        │
        ▼
安装 ocserv + ocserv-worker，并跑 verify-spec01 + verify-dst
```

DST 补丁必须基于 **已打 SPEC-01 的 1.5.0 树** 编写锚点；冲突时优先保证 SPEC-01 语义。

---

### 建议仓库布局

```text
ocserv-tunnel/
├── README.md
├── scripts/
│   ├── apply-spec01-edits.py      # 从 qosnat2 移植并适配 1.5.0
│   ├── apply-dst-edits.py         # SPEC-02
│   └── install-ocserv.sh          # 可选：对齐 qosnat2 安装契约
├── docs/
│   ├── SPEC-01-ROUTE-B.md         # TunnelGroupName 设计 + 1.5.0 移植说明
│   ├── SPEC-02-DST.md
│   └── ASAV-PARITY.md
├── tests/
│   ├── verify-spec01-symbols.sh
│   ├── verify-dst-symbols.sh
│   ├── route-b-smoke.sh           # RADIUS 含 TunnelGroupName（能 mock 更好）
│   └── dst-smoke.sh
└── examples/
    ├── ocserv.route-b.conf
    └── ocserv.dst.conf
```

---

### 实现阶段

#### Phase 0 — 基线与 SPEC-01 移植（阻塞后续）

1. Clone **1.5.0**；
2. Vendored qosnat2 `apply-spec01-edits.py`，适配 1.5.0 锚点直至可重复应用；
3. 编译；安装双二进制；`verify-spec01-symbols.sh` 通过；
4. 写 `docs/SPEC-01-ROUTE-B.md`（含相对 1.4.2 的移植差异）。

#### Phase 1 — DST MVP

1. 配置解析：`dynamic-split-include-domains`（全局 + group + user）；
2. 认证成功后自动注入 `X-CSTP-Post-Auth-XML`；
3. 与用户自定义 `custom-header` 合并；
4. debug 日志打印最终域名列表；
5. `apply-dst-edits.py` + `verify-dst-symbols.sh`。

#### Phase 2 — DST 完整 parity

- `dynamic-split-exclude-domains`；Enhanced include+exclude；
- RADIUS/group-policy 下发预留（可先文档）。

#### Phase 3 — qosnat2 回灌说明

输出集成清单（见下），代码改动可在 qosnat2 另 PR。

---

### 关键源码探索路径

在 **1.5.0**（及 SPEC-01 已改文件）中搜索：

| 关键词 | 用途 |
|--------|------|
| `custom-header` / `X-CSTP-` | 自定义头与 CSTP |
| `split-dns` / `route` / `no-route` | 静态拆分 |
| `config-per-group` | 组配置 |
| `worker-auth.c` / `group-access` | SPEC-01 组选择 |
| `radius_auth_bind_group` / `TunnelGroupName` | SPEC-01 RADIUS |
| 认证完成 / HTTP 响应构造处 | Post-Auth XML 注入点 |

最小 diff；先读清再改。

---

### 测试计划

1. **编译**：`meson setup build && ninja -C build`
2. **SPEC-01 符号**：`verify-spec01-symbols.sh`（双二进制）
3. **SPEC-01 功能**：Route B 登录 → RADIUS 含 TunnelGroupName=146
4. **DST 协议**：响应含 `X-CSTP-Post-Auth-XML` + `dynamic-split-include-domains`
5. **DST 功能**（Win/macOS AnyConnect）：匹配域名动态进路由表
6. **共存回归**：SPEC-01 + DST 同时启用，互不破坏

---

### 交付物清单

- [ ] `apply-spec01-edits.py`（适配 **1.5.0**，idempotent，完整链路）
- [ ] `docs/SPEC-01-ROUTE-B.md`（含 1.4.2→1.5.0 移植说明）
- [ ] `tests/verify-spec01-symbols.sh`
- [ ] `apply-dst-edits.py`（在已打 SPEC-01 的 1.5.0 树上）
- [ ] `docs/SPEC-02-DST.md` / `ASAV-PARITY.md`
- [ ] `examples/ocserv.route-b.conf` / `ocserv.dst.conf`
- [ ] `tests/verify-dst-symbols.sh` / `dst-smoke.sh`
- [ ] README：构建顺序、与 ASAv 对照、已知限制
- [ ] 「qosnat2 集成说明」一节

---

### qosnat2 回灌集成说明（交付时必须写出）

回灌时 qosnat2 大致需改：

| 位置 | 改动 |
|------|------|
| `OCSERV_SPEC01_BASELINE` / 默认 version | **1.4.2 → 1.5.0** |
| `patches/ocserv/apply-spec01-edits.py` | 替换为本项目 1.5.0 版 |
| `patches/ocserv/apply-dst-edits.py` | 新增 |
| `scripts/install-ocserv.sh` | 拉 1.5.0 → SPEC-01 → DST → 双二进制安装 → 双校验 |
| `docs/OCSERV.md` / `patches/ocserv/README.md` | 基线与补丁说明更新 |
| `internal/ocserv` RenderConf / groups / vhost | 输出 `dynamic-split-*-domains` |
| store + Web UI | 域名列表字段 |

安装顺序必须与本项目一致；**禁止**只装 `ocserv` 不装 `ocserv-worker`。

---

### 编码规范

- 遵循 ocserv C 风格；最小 diff；
- 补丁脚本 idempotent；
- SPEC-02 不得破坏 SPEC-01 已改语义；
- 新配置项写入手册/示例。

### 工作流程（严格执行）

1. Clone **ocserv 1.5.0**；
2. 从 qosnat2 拷贝并**移植** SPEC-01 到 1.5.0，校验双二进制；
3. 在已打 SPEC-01 的树上设计并实现 DST；
4. 跑完全部 verify / smoke；
5. 写清 qosnat2 回灌步骤（含版本号变更）。

协议行为以 **抓包 + AnyConnect verbose** 为准；与文档冲突时更新 `ASAV-PARITY.md`。

## --- 复制终点 ---

---

## 附录：qosnat2 侧现状参考（写提示词时的上下文）

当前 qosnat2 生产仍为 **ocserv 1.4.2 + SPEC-01**。本新项目负责：

1. 基线升级到 **1.5.0**；
2. 移植完整 SPEC-01；
3. 新增 DST（SPEC-02）；
4. 再回灌 qosnat2 安装链。

参考：`patches/ocserv/README.md`、`docs/OCSERV.md`、`scripts/install-ocserv.sh`。

## 附录：参考链接

- [ocserv#352 Dynamic tunnel exclusion/inclusion](https://gitlab.com/openconnect/ocserv/-/issues/352)
- [Cisco Dynamic Split Tunneling](https://www.cisco.com/c/en/us/support/docs/security/anyconnect-secure-mobility-client/215383-asa-anyconnect-dynamic-split-tunneling.html)
- [vpn-slice#68 Dynamic Split Include](https://github.com/dlenski/vpn-slice/issues/68)
- qosnat2：`patches/ocserv/apply-spec01-edits.py`、`patches/ocserv/README.md`、`docs/OCSERV.md`
