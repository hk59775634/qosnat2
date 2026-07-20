package ebpf

import (
	"encoding/binary"
	"fmt"
	"net"
)

// RateVal 与 bpf rate_val 对齐（24 字节）
type RateVal struct {
	DownBPS    uint64
	UpBPS      uint64
	ClassMinor uint32
	HostMask   uint8 // 1–32；0/32=每主机；<32 时按前缀聚合共享限速桶
	Pad        [3]byte
}

// NormalizeHostMask 规范化策略掩码：0 或越界视为每主机 /32。
func NormalizeHostMask(mask int) uint8 {
	if mask <= 0 || mask > 32 {
		return 32
	}
	return uint8(mask)
}

// AggregateHostKey 将 hostKey（与 IPToHostKey 相同的大端 IPv4 数值）按 host_mask 归并为共享桶键。
func AggregateHostKey(hostKey uint32, hostMask uint8) uint32 {
	if hostMask == 0 || hostMask >= 32 {
		return hostKey
	}
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, hostKey)
	masked := net.IP(b).Mask(net.CIDRMask(int(hostMask), 32))
	return binary.BigEndian.Uint32(masked)
}

// LPMKey profile_lpm key
type LPMKey struct {
	Prefixlen uint32
	Addr      uint32 // network byte order
}

func IPToLPMKey(cidr string) (LPMKey, error) {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		return LPMKey{}, err
	}
	ones, _ := n.Mask.Size()
	ip := n.IP.To4()
	if ip == nil {
		return LPMKey{}, fmt.Errorf("not ipv4: %s", cidr)
	}
	return LPMKey{
		Prefixlen: uint32(ones),
		Addr:      binary.BigEndian.Uint32(ip),
	}, nil
}

func IPToHostKey(ip string) (uint32, error) {
	p := net.ParseIP(ip)
	if p == nil {
		return 0, fmt.Errorf("invalid ip: %s", ip)
	}
	p4 := p.To4()
	if p4 == nil {
		return 0, fmt.Errorf("not ipv4: %s", ip)
	}
	return binary.BigEndian.Uint32(p4), nil
}

func HostKeyToIP(k uint32) string {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, k)
	return net.IP(b).String()
}

func (r RateVal) Marshal() []byte {
	b := make([]byte, 24)
	binary.LittleEndian.PutUint64(b[0:], r.DownBPS)
	binary.LittleEndian.PutUint64(b[8:], r.UpBPS)
	binary.LittleEndian.PutUint32(b[16:], r.ClassMinor)
	mask := r.HostMask
	if mask == 0 {
		mask = 32
	}
	b[20] = mask
	return b
}

func (k LPMKey) Marshal() []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint32(b[0:], k.Prefixlen)
	// data 与 skb 源地址相同：按网络序四字节写入（勿 LittleEndian）
	binary.BigEndian.PutUint32(b[4:], k.Addr)
	return b
}
