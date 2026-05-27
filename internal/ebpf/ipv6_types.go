package ebpf

import (
	"encoding/binary"
	"fmt"
	"net"
)

const mapProfile6 = "profile_lpm6"

// LPMKeyV6 profile_lpm6 key（与 bpf lpm_v6_key 对齐）
type LPMKeyV6 struct {
	Prefixlen uint32
	Addr      [16]byte
}

func IPToLPMKeyV6(cidr string) (LPMKeyV6, error) {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		return LPMKeyV6{}, err
	}
	if n.IP.To4() != nil {
		return LPMKeyV6{}, fmt.Errorf("not ipv6: %s", cidr)
	}
	ones, _ := n.Mask.Size()
	ip := n.IP.To16()
	if ip == nil {
		return LPMKeyV6{}, fmt.Errorf("not ipv6: %s", cidr)
	}
	var k LPMKeyV6
	k.Prefixlen = uint32(ones)
	copy(k.Addr[:], ip)
	return k, nil
}

func (k LPMKeyV6) Marshal() []byte {
	b := make([]byte, 20)
	binary.LittleEndian.PutUint32(b[0:], k.Prefixlen)
	copy(b[4:], k.Addr[:])
	return b
}

func profileMapForCIDR(cidr string) (v4, v6 bool, err error) {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		return false, false, err
	}
	if n.IP.To4() != nil {
		return true, false, nil
	}
	if n.IP.To16() != nil {
		return false, true, nil
	}
	return false, false, fmt.Errorf("invalid cidr: %s", cidr)
}
