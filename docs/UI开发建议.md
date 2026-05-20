一、Dashboard（仪表盘）

这是用户登录后的首页。

pfSense Dashboard 组件
系统状态
CPU 使用率
内存使用率
磁盘使用率
温度
风扇
电源状态
网络状态
WAN/LAN 接口状态
接口流量图
在线 VPN 用户
当前会话数（states）
NAT sessions
PPS/BPS
服务状态
DHCP
DNS
IPsec
OpenVPN
WireGuard
HAProxy
Captive Portal
日志摘要
最近系统日志
最近防火墙拦截
VPN 登录失败
RADIUS 认证失败
网关监控
RTT
丢包率
SLA 状态
小组件机制

pfSense Dashboard 是 widget 化设计：

可拖拽
可折叠
用户级保存布局
二、Interfaces（接口管理）⭐

这是最核心模块之一。

基础接口功能
接口列表
WAN
LAN
OPT1/OPT2…
VLAN
LAGG
PPPoE
GRE
GIF
VXLAN（pfSense 原生弱）
接口配置
IP
DHCP/static
MTU/MSS
MAC spoof
Gateway
IPv6 RA
DHCP6
高级功能
接口 enable/disable
硬件 offload
flow control
SR-IOV VF 绑定（你建议增强）
RSS/RPS/XPS
多队列
三、Firewall（防火墙策略）⭐⭐⭐

这是 UI 最复杂部分。

1. Rules（规则）
pfSense 规则字段
Action
Interface
Address Family
Protocol
Source
Destination
Port
Gateway
Queue/QoS
Schedule
Log
Description
高级字段
TCP flags
state type
max states
source tracking
rate limit
reply-to
policy routing
2. Aliases（对象组）⭐⭐⭐

极重要。

类型
Host
Network
Port
URL Table
GeoIP
ASN
MAC
FQDN
建议增强

你非常适合做：

Redis 实时 Alias
动态 DNS/IPSET
API 更新对象组
大规模 CDN ASN 对象
3. NAT
Outbound NAT
自动
Hybrid
Manual
Port Forward
DNAT
Reflection
Hairpin NAT
1:1 NAT
NPt（IPv6）
4. Floating Rules

跨接口全局规则。

通常用于：

QoS
IPS
高级限速
fastpath bypass
四、Traffic Shaper（流控/QoS）⭐⭐⭐

这是你最值得重点超越 pfSense 的部分。

pfSense 原生做得并不好。

pfSense 现有 UI
Limiter
上传/下载限速
fq_codel
CoDel
PRIQ
Queue
子队列
优先级
Wizard
游戏
VoIP
多WAN
你建议新增（非常关键）
SDN/QoS UI
用户级 QoS
租户级 QoS
VXLAN/VNI QoS
ASN QoS
国家 QoS
APP QoS
Linux tc/VPP 级能力
HTB
CAKE
fq_codel
ETF
XDP
eBPF
可视化
实时队列图
热门流
elephant flow
per-user bandwidth
五、VPN ⭐⭐⭐
1. IPsec
Phase1
IKEv1/v2
auth
cert
DH group
Phase2
subnet
encryption
状态
SA
DPD
rekey
2. OpenVPN
服务端
server mode
TLS auth
CCD
客户端
export bundle
用户
在线用户
kill session
3. WireGuard（新版才有）
Tunnel
Peer
Allowed IPs
Keepalive
4. 你建议新增（重点）

你自己的平台建议：

MASQUE
H3 VPN
XTLS
Reality
AnyConnect Compatible
RADIUS Dynamic Policy
API 下发 ACL/QoS
多 POP SDN
六、Routing（路由）⭐⭐⭐
基础
Static Route
Gateway
多 WAN
Tier
Weight
Policy Routing
动态路由（pfSense插件）
FRR
BGP
OSPF
RIP
IS-IS
你建议增强
EVPN/VXLAN
SRv6
WireGuard Mesh
Flow Routing
Anycast POP
七、Services（服务）
DHCP
DHCPv4
DHCPv6
Static mapping
DNS Resolver

pfSense 用：

Unbound

功能：

DNSSEC
DoT
Host override
DNS Forwarder
dnsmasq
NTP
Dynamic DNS

支持：

Cloudflare
DynDNS
Captive Portal
Portal page
Voucher
RADIUS
UPnP
Wake-on-LAN
八、High Availability（高可用）⭐⭐⭐
CARP

虚拟 IP 漂移。

pfsync

同步：

states
NAT
rules
XMLRPC Sync

同步配置。

你建议增强
分布式控制平面
etcd/Redis 配置同步
Raft
多节点控制器
九、Diagnostics（诊断）⭐⭐⭐

这个是 pfSense 最好用的模块之一。

常见功能
Ping
Traceroute
DNS Lookup
Packet Capture
ARP Table
NDP Table
States
Sockets
Routes
pfTop
高级建议
eBPF trace
conntrack viewer
nft monitor
tcpdump web terminal
实时 PPS Top
十、Status（状态监控）
Interfaces
流量图
Errors
Drops
Gateways
RTT
Loss
CARP
DHCP leases
IPsec/OpenVPN/WG status
System logs

分类：

firewall
DHCP
VPN
auth
routing
十一、System（系统）
General Setup
hostname
DNS
timezone
User Manager ⭐⭐⭐
用户
Group
Privilege
建议增强

RBAC：

租户管理员
只读监控
API token
Cert Manager ⭐⭐⭐
CA
Cert
CSR
ACME
Package Manager

类似插件市场。

Update
Backup/Restore ⭐⭐⭐
XML backup
自动云备份（建议）
API ⭐⭐⭐⭐（你一定要重点做）

pfSense 原生 API 很弱。

你建议：

REST API
OpenAPI
WebSocket
gRPC
RBAC API
Terraform Provider
十二、Reporting（报表）
流量图
RRD
PPS
CPU
VPN 用户统计
Top Talker
NetFlow/sFlow/IPFIX ⭐⭐⭐

这个你非常适合重点做。

十三、通知系统 ⭐⭐⭐

pfSense 很弱。

你建议：

Telegram
Discord
Webhook
Slack
飞书
企业微信
邮件
短信
十四、你应该重点超越 pfSense 的部分（非常重要）

pfSense 本质上还是：

FreeBSD
pf
单机时代思维

而你的目标明显已经接近：

SDN
VPN Provider
高性能云网络
多节点 POP
API-first

所以你真正应该强化的是：

1. API-first 架构 ⭐⭐⭐⭐⭐

不要像 pfSense 那样：

UI 调 shell
shell 改 XML

你应该：

Web UI
   ↓
REST/gRPC
   ↓
Control Plane
   ↓
Agent
   ↓
nftables/tc/VPP
2. 实时状态数据库 ⭐⭐⭐⭐⭐

pfSense 没有：

Redis
Kafka
ClickHouse

你应该有：

实时流量
用户在线
QoS
telemetry
3. 多节点 SDN ⭐⭐⭐⭐⭐

pfSense 完全不适合：

多 POP
Anycast
EVPN
Overlay

而你已经非常适合做：

WireGuard Mesh
VXLAN EVPN
分布式 QoS
中央控制器
十五、建议你的 UI 菜单结构（推荐）

这个更现代：

Dashboard
Network
 ├ Interfaces
 ├ VLAN
 ├ Routing
 ├ BGP
 ├ DNS
Security
 ├ Firewall
 ├ NAT
 ├ IPS/IDS
 ├ GeoIP
VPN
 ├ WireGuard
 ├ MASQUE
 ├ SSLVPN
 ├ Users
Traffic
 ├ QoS
 ├ Policies
 ├ App Control
Observability
 ├ Logs
 ├ NetFlow
 ├ Metrics
 ├ Packet Capture
Cluster
 ├ Nodes
 ├ Sync
 ├ SDN
System
 ├ Users
 ├ API
 ├ Certificates
 ├ Backup