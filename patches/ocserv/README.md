# ocserv Route B（SPEC-01）补丁

**基线：** ocserv **1.4.2**（与 [ocserv-tunnel](https://github.com/hk59775634/ocserv-tunnel) 生产验证一致）。

`install-ocserv.sh` 在编译前对干净源码树执行：

```bash
python3 patches/ocserv/apply-spec01-edits.py <ocserv-src>
```

不要使用早期残缺的 `*.patch`（`*.legacy-incomplete` 仅作历史参考，安装脚本不会应用）。

完整链路覆盖：

| 区域 | 作用 |
|------|------|
| `worker-auth.c` | URL / `<group-access>` 解析；`auth_cont` 携带组名 |
| `sec-mod-auth.c` | `radius_auth_bind_group`（密码阶段前绑定） |
| `auth/radius.c` | Access-Request 发送 TunnelGroupName（VSA 146） |
| `acct/radius.c` | Accounting 发送 TunnelGroupName |
| `ipc.proto` | `sec_auth_cont_msg.group_name` |

安装后须同时存在打了补丁的 `/usr/local/sbin/ocserv` 与 `ocserv-worker`。
