package store

// Google WebRTC / STUN / TURN 拦截预置：缓解 VPN+NAT 下 WebRTC 泄漏导致 Gemini 等服务异常。
// 仅阻断 Google STUN 主机与 Meet/WebRTC 媒体网段上的 UDP WebRTC 端口，不影响普通 HTTPS。

const (
	AliasGoogleWebRTCStun  = "google_webrtc_stun"
	AliasGoogleWebRTCMedia = "google_webrtc_media"
	AliasGoogleWebRTCPorts = "google_webrtc_ports"

	RuleIDGoogleWebRTCStun  = "fr-gwebrtc-stun"
	RuleIDGoogleWebRTCMedia = "fr-gwebrtc-media"
)

// GoogleWebRTCStunDomains Google 公开 STUN 主机名。
func GoogleWebRTCStunDomains() []string {
	return []string{
		"stun.l.google.com",
		"stun1.l.google.com",
		"stun2.l.google.com",
		"stun3.l.google.com",
		"stun4.l.google.com",
	}
}

// GoogleWebRTCMediaCIDRsV4 Google Meet / Workspace WebRTC 媒体与 TURN IPv4 网段。
// 来源：Google Workspace「Prepare your network for Meet」文档公布的媒体地址。
func GoogleWebRTCMediaCIDRsV4() []string {
	return []string{
		"74.125.250.0/24",
		"74.125.247.128/32",
		"142.250.82.0/24",
	}
}

// GoogleWebRTCPortMembers STUN/TURN 常用 UDP 端口。
func GoogleWebRTCPortMembers() []string {
	return []string{"19302-19309", "3478"}
}

// GoogleWebRTCBlockAliases 返回预置别名（未做 FQDN 解析）。
func GoogleWebRTCBlockAliases() []AliasSet {
	return []AliasSet{
		{
			Name:    AliasGoogleWebRTCStun,
			Type:    "fqdn",
			Domains: GoogleWebRTCStunDomains(),
			Comment: "Google STUN (WebRTC); block to prevent VPN IP leak",
		},
		{
			Name:    AliasGoogleWebRTCMedia,
			Type:    "ipv4_addr",
			Members: GoogleWebRTCMediaCIDRsV4(),
			Comment: "Google Meet/WebRTC media & TURN IPv4 ranges",
		},
		{
			Name:    AliasGoogleWebRTCPorts,
			Type:    "port",
			Members: GoogleWebRTCPortMembers(),
			Comment: "Google WebRTC STUN/TURN UDP ports",
		},
	}
}

// GoogleWebRTCBlockRules 返回 forward 丢弃规则（固定 ID，可幂等 upsert）。
func GoogleWebRTCBlockRules() []FilterRule {
	return []FilterRule{
		{
			ID:       RuleIDGoogleWebRTCStun,
			Chain:    "forward",
			Action:   "drop",
			Proto:    "udp",
			DstAlias: AliasGoogleWebRTCStun,
			Comment:  "Block Google STUN (WebRTC leak / Gemini)",
			Enabled:  true,
			Counter:  true,
		},
		{
			ID:           RuleIDGoogleWebRTCMedia,
			Chain:        "forward",
			Action:       "drop",
			Proto:        "udp",
			DstAlias:     AliasGoogleWebRTCMedia,
			DstPortAlias: AliasGoogleWebRTCPorts,
			Comment:      "Block Google WebRTC media/TURN UDP",
			Enabled:      true,
			Counter:      true,
		},
	}
}

// UpsertAliasesByName 按名称替换或追加别名；返回合并后的列表与新增/更新数量。
func UpsertAliasesByName(existing []AliasSet, upserts []AliasSet) (out []AliasSet, added, updated int) {
	out = append([]AliasSet(nil), existing...)
	for _, u := range upserts {
		found := false
		for i := range out {
			if out[i].Name == u.Name {
				out[i] = u
				updated++
				found = true
				break
			}
		}
		if !found {
			out = append(out, u)
			added++
		}
	}
	return out, added, updated
}

// UpsertFilterRulesByID 按 ID 替换或追加规则；返回合并后的列表与新增/更新数量。
func UpsertFilterRulesByID(existing []FilterRule, upserts []FilterRule) (out []FilterRule, added, updated int) {
	out = append([]FilterRule(nil), existing...)
	for _, u := range upserts {
		found := false
		for i := range out {
			if out[i].ID == u.ID {
				// 保留用户可能改过的 Enabled；预置再次点击时强制启用拦截。
				out[i] = u
				updated++
				found = true
				break
			}
		}
		if !found {
			out = append(out, u)
			added++
		}
	}
	return out, added, updated
}
