export default {
  mark: {
    title: 'Mark isolation policy',
    description: 'Audit of skb mark usage vs nft/TC',
    ok: 'Policy audit passed',
    issues: 'Issues found',
  },
  active: {
    title: 'Active per-IP shaping',
    description: 'Live Per-IP EDT shaping from eBPF maps',
    downCfg: 'Down config',
    upCfg: 'Up config',
    activityDown: 'Down activity',
    activityUp: 'Up activity',
    noEntries: 'No active entries',
  },
  ebpf: {
    title: 'eBPF maps & TC',
    description: 'Attached programs and map counters',
    reloading: 'Re-attaching…',
    reattach: 'Re-attach TC to LAN',
    tcPrograms: 'TC programs',
    notAttached: 'Not attached',
    noProgInfo: 'No program info',
    mapSummary: 'Map summary',
    reattached: 'eBPF re-attached to LAN',
  },
}
