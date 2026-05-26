export default {
  mark: {
    title: 'Mark 隔离策略审计',
    description: 'skb mark 与 nft/TC 一致性审计',
    ok: '规则审计通过',
    issues: '发现问题',
  },
  active: {
    title: '活跃主机（按 IP）',
    description: 'eBPF 分类后的 HTB 叶子队列',
    downCfg: '下行配置',
    upCfg: '上行配置',
    noEntries: '无活跃条目',
  },
  ebpf: {
    title: 'eBPF 映射',
    description: '已附加 TC 程序与 Map 计数',
    reloading: '重载中…',
    reattach: '重新附加 TC',
    tcPrograms: 'TC 程序',
    notAttached: '未附加',
    noProgInfo: '无程序信息',
    mapSummary: 'Map 摘要',
    reattached: 'eBPF 已重新附加到 LAN',
  },
}
