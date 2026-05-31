package api

import (
	"log"
	"strings"
	"time"

	"github.com/hk59775634/qosnat2/internal/nft"
	"github.com/hk59775634/qosnat2/internal/store"
)

type filterIncrementalOp int

const (
	filterOpNone filterIncrementalOp = iota
	filterOpAdd
	filterOpDelete
)

func (srv *Server) reloadFilterWithOptionalIncremental(backup []store.FilterRule, op filterIncrementalOp, rule store.FilterRule) error {
	if op == filterOpNone || !nft.IncrementalEnabled() {
		return srv.reloadNftWithFilterRevert(backup)
	}
	chain := strings.ToLower(strings.TrimSpace(rule.Chain))
	if chain != "forward" && chain != "input" {
		return srv.reloadNftWithFilterRevert(backup)
	}
	if op == filterOpAdd && rule.NftRuleLine() == "" {
		return srv.reloadNftWithFilterRevert(backup)
	}

	start := time.Now()
	var usedFullReload bool
	err := srv.withNftApply(func() error {
		var incErr error
		switch op {
		case filterOpAdd:
			incErr = nft.AddFilterRuleLine(chain, rule.NftRuleLine())
		case filterOpDelete:
			incErr = nft.DeleteFilterRuleByID(chain, rule.ID)
		}
		if incErr != nil {
			usedFullReload = true
			return srv.reloadNftLocked()
		}
		st := srv.syncedFirewallState(srv.store.Get())
		body, err := nft.Render(srv.nftCfg(), st)
		if err != nil {
			return err
		}
		if err := nft.WriteRulesFile(body); err != nil {
			return err
		}
		srv.persistAutoFirewallRules()
		srv.reconcileWarpAfterNft()
		return nil
	})
	if !usedFullReload {
		srv.dataplaneMetrics.recordNftReload(time.Since(start), err)
	}
	if err != nil {
		srv.setFilterRules(backup)
		if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
		return srv.reloadNftWithFilterRevert(backup)
	}
	return nil
}
