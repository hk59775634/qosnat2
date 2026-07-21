package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/hk59775634/qosnat2/internal/linknet"
)

// FindWireGuardInstance 按 id 查找，返回下标与指针（state 需为可变切片元素地址时由调用方索引）
func FindWireGuardInstance(list []WireGuardInstance, id string) (int, bool) {
	id = strings.TrimSpace(id)
	for i := range list {
		if strings.TrimSpace(list[i].ID) == id {
			return i, true
		}
	}
	return -1, false
}

// NewWireGuardInstanceID 生成短随机 id（新实例）
func NewWireGuardInstanceID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "wg-" + hex.EncodeToString(b[:])
}

// NormalizeWireGuardInstance 缺省字段与 peers 切片
func NormalizeWireGuardInstance(inst *WireGuardInstance) {
	if inst == nil {
		return
	}
	if strings.TrimSpace(inst.ID) == "" {
		inst.ID = "default"
	}
	inst.ID = strings.TrimSpace(inst.ID)
	inst.Name = strings.TrimSpace(inst.Name)
	if inst.Mode != WGModeClient {
		inst.Mode = WGModeServer
	}
	if strings.TrimSpace(inst.Interface) == "" {
		inst.Interface = "wg0"
	}
	if inst.ListenPort == 0 {
		inst.ListenPort = 51820
	}
	if strings.TrimSpace(inst.Address) == "" {
		inst.Address = linknet.WireGuardDefaultAddress
	}
	if inst.Peers == nil {
		inst.Peers = []WGPeer{}
	}
	if inst.DNS == nil {
		inst.DNS = []string{}
	}
}

// MigrateLegacyWireGuardToInstances 将旧单实例写入 wireguards[0]（仅当 wireguards 为空）
func MigrateLegacyWireGuardToInstances(v *VPNState) {
	if v == nil {
		return
	}
	if v.LegacyWireGuard == nil {
		return
	}
	if len(v.WireGuards) > 0 {
		v.LegacyWireGuard = nil
		return
	}
	leg := *v.LegacyWireGuard
	v.WireGuards = []WireGuardInstance{
		{ID: "default", Name: "default", Mode: WGModeServer, WireGuardState: leg},
	}
	v.LegacyWireGuard = nil
	for i := range v.WireGuards {
		NormalizeWireGuardInstance(&v.WireGuards[i])
	}
}

// ValidateWireGuardInstancePatch 校验 PUT 后的实例（与列表内其他项不冲突由 API 层做）
func ValidateWireGuardInstancePatch(inst WireGuardInstance) error {
	if strings.TrimSpace(inst.ID) == "" {
		return fmt.Errorf("id required")
	}
	if inst.Mode != WGModeServer && inst.Mode != WGModeClient {
		return fmt.Errorf("mode must be server or client")
	}
	return nil
}
