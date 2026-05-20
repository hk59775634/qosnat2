package ebpf

import (
	"encoding/binary"
	"fmt"
	"net"
)

// RateVal 与 bpf rate_val 对齐
type RateVal struct {
	DownBPS     uint64
	UpBPS       uint64
	ClassMinor  uint32
	Pad         [4]byte
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
	return b
}

func (k LPMKey) Marshal() []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint32(b[0:], k.Prefixlen)
	binary.LittleEndian.PutUint32(b[4:], k.Addr)
	return b
}
