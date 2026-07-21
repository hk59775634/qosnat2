# ocserv Route B（SPEC-01）+ DST（SPEC-02）补丁

**基线：** ocserv **1.5.0**（由 [ocserv-tunnel](https://github.com/hk59775634/ocserv) 移植/维护）。

`install-ocserv.sh` 在编译前对干净源码树依次执行：

```bash
python3 patches/ocserv/apply-spec01-edits.py <ocserv-src>
python3 patches/ocserv/apply-dst-edits.py <ocserv-src>
```

不要使用早期残缺的 `*.patch`（`*.legacy-incomplete` 仅作历史参考）。

## SPEC-01（Route B / TunnelGroupName）

| 区域 | 作用 |
|------|------|
| `worker-auth.c` | URL / `<group-access>` 解析；`auth_cont` 携带组名 |
| `sec-mod-auth.c` | `radius_auth_bind_group`（密码阶段前绑定） |
| `auth/radius.c` | Access-Request 发送 TunnelGroupName（VSA 146） |
| `acct/radius.c` | Accounting 发送 TunnelGroupName |
| `ipc.proto` | `sec_auth_cont_msg.group_name` |

## SPEC-02（动态拆分隧道）

| 区域 | 作用 |
|------|------|
| 配置 / IPC | `dynamic-split-include-domains` / `dynamic-split-exclude-domains` |
| `worker-vpn.c` | CONNECT 下发 `X-CSTP-Post-Auth-XML` |

安装后须同时存在打了补丁的 `/usr/local/sbin/ocserv` 与 `ocserv-worker`。
