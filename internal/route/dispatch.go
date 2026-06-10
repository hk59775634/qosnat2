package route

import (
	"github.com/hk59775634/qosnat2/internal/frr"
	"github.com/hk59775634/qosnat2/internal/store"
)

const (
	BackendKernel = "kernel"
	BackendFRR    = "frr"
)

// NormalizeBackend 默认 kernel。
func NormalizeBackend(b string) string {
	switch b {
	case BackendFRR:
		return BackendFRR
	default:
		return BackendKernel
	}
}

func countEnabled(routes []store.RouteEntry) int {
	n := 0
	for _, r := range routes {
		if r.Enabled {
			n++
		}
	}
	return n
}

// ApplyFromState 按 state 回放托管路由与 FRR 动态路由。
func ApplyFromState(st store.State) (ApplyResult, error) {
	return ApplyManagedRoutesWithDynamic(st.Routes, st.System.RouteBackend, st.DynamicRouting)
}

// ApplyManagedRoutesWithDynamic 回放托管路由，FRR 模式下附带动态路由。
func ApplyManagedRoutesWithDynamic(routes []store.RouteEntry, backend string, dr store.DynamicRoutingState) (ApplyResult, error) {
	backend = NormalizeBackend(backend)
	if backend == BackendFRR {
		if err := frr.ApplyManaged(routes); err != nil {
			return ApplyResult{Backend: BackendFRR}, err
		}
		if err := frr.ApplyDynamic(dr); err != nil {
			return ApplyResult{Backend: BackendFRR}, err
		}
		return ApplyResult{Backend: BackendFRR, Applied: countEnabled(routes)}, nil
	}
	return ApplyAllDiff(routes)
}
