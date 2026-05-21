package shaper

import (
	"fmt"
	"net"
)

// ProfileSubnetMinor 网段模板在 ifb 上的共享 HTB minor（0x300 + profile id）
func ProfileSubnetMinor(profileID int) uint32 {
	m := uint32(0x300) | (uint32(profileID) & 0xff)
	if m == 1 {
		m++
	}
	return m
}

// EnsureProfileSubnetOnIFB 为 /24 等网段预建 ifb 类 + u32，避免首包落在默认 10gbit
func (h *HostShaper) EnsureProfileSubnetOnIFB(cidr string, profileID int, upBPS uint64) error {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}
	ones, bits := n.Mask.Size()
	if ones == bits {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	minor := ProfileSubnetMinor(profileID)
	up := bpsToTC(upBPS)
	cid := fmt.Sprintf("1:%x", minor)
	if err := h.ensureClass(IFBDev, cid, up, up); err != nil {
		return fmt.Errorf("ifb subnet %s: %w", cidr, err)
	}
	if err := installIFBUploadFilterCIDR(cidr, minor); err != nil {
		return err
	}
	return nil
}

func (h *HostShaper) RemoveProfileSubnetFromIFB(cidr string, profileID int) {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		return
	}
	ones, bits := n.Mask.Size()
	if ones == bits {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	_ = removeIFBUploadFilterCIDR(cidr)
	cid := fmt.Sprintf("1:%x", ProfileSubnetMinor(profileID))
	_ = h.delClass(IFBDev, cid)
}
