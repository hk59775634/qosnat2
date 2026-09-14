package api

import (
	"log"
	"strings"

	"github.com/hk59775634/qosnat2/internal/lfn"
	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/sysctl"
)

type ifaceLFNStatus struct {
	TCPCongestionControl string `json:"tcp_congestion_control,omitempty"`
	Qdisc                string `json:"qdisc,omitempty"`
	MSSClamp             int    `json:"mss_clamp,omitempty"`
	QdiscProtected       bool   `json:"qdisc_protected,omitempty"`
	Warning              string `json:"warning,omitempty"`
}

func (srv *Server) applyLFNDataplane(revert []string) []string {
	st := srv.store.Get()
	if err := srv.applySystemTuning(st); err != nil {
		log.Printf("lfn sysctl: %v", err)
	}
	warns := srv.applyLFNQdiscs(st, revert)
	if w := srv.tryReloadNft(); w != "" {
		warns = append(warns, w)
	}
	return warns
}

func (srv *Server) applyLFNQdiscs(st store.State, revert []string) []string {
	var warns []string
	var shaperDevs []string
	if srv.shaperEnabled() {
		shaperDevs = srv.shaperAttachDevices(st)
	}
	seen := map[string]struct{}{}
	for _, ic := range st.Network.Ifaces {
		if !ic.LfnEnabled {
			continue
		}
		seen[ic.Device] = struct{}{}
		warns = append(warns, srv.applyLFNDevice(st, ic.Device, true, shaperDevs)...)
	}
	for _, dev := range revert {
		dev = strings.TrimSpace(dev)
		if dev == "" {
			continue
		}
		if _, on := seen[dev]; on {
			continue
		}
		warns = append(warns, srv.applyLFNDevice(st, dev, false, shaperDevs)...)
	}
	return warns
}

func (srv *Server) applyLFNDevice(st store.State, dev string, enable bool, shaperDevs []string) []string {
	live := lfn.QdiscShow(dev)
	prot := lfn.DeviceProtected(dev, shaperDevs, live)
	txq := store.LFNTxQueueLen
	if !enable {
		txq = srv.lfnRestoreTxQLen(st, dev)
	}
	p := lfn.QdiscPlan(dev, enable, prot, lfn.QueueCount(dev), txq)
	var warns []string
	if p.Warning != "" {
		warns = append(warns, dev+": "+p.Warning)
	}
	if err := lfn.ExecPlan(p); err != nil {
		log.Printf("lfn qdisc %s: %v", dev, err)
		warns = append(warns, err.Error())
	}
	return warns
}

func (srv *Server) lfnRestoreTxQLen(st store.State, dev string) int {
	if dev == srv.env.DevLAN {
		return netif.EffectiveTxQLen(st.System.TxQueueLenLAN)
	}
	if dev == srv.env.DevWAN {
		return netif.EffectiveTxQLen(st.System.TxQueueLenWAN)
	}
	return 5000
}

func (srv *Server) ifaceLFNStatus(st store.State, dev string, ic *store.IfaceConfig, liveCC string) ifaceLFNStatus {
	show := lfn.QdiscShow(dev)
	var shaperDevs []string
	if srv.shaperEnabled() {
		shaperDevs = srv.shaperAttachDevices(st)
	}
	prot := lfn.DeviceProtected(dev, shaperDevs, show)
	stt := ifaceLFNStatus{
		TCPCongestionControl: liveCC,
		Qdisc:                firstQdiscLine(show),
		QdiscProtected:       prot,
	}
	if ic != nil && ic.LfnEnabled {
		stt.MSSClamp = store.EffectiveLFNMSS(true, ic.LfnMssClamp, lfn.LinkMTU(dev))
		if prot {
			stt.Warning = "shaper/clsact present; skip root qdisc, MSS only"
		}
	}
	return stt
}

func firstQdiscLine(show string) string {
	show = strings.TrimSpace(show)
	if show == "" {
		return ""
	}
	if i := strings.IndexByte(show, '\n'); i >= 0 {
		return strings.TrimSpace(show[:i])
	}
	return show
}

func liveTCPCongestion() string {
	m := sysctl.ReadLive([]string{"net.ipv4.tcp_congestion_control"})
	return m["net.ipv4.tcp_congestion_control"]
}
