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

// ApplyManagedRoutes 按 backend 回放托管路由。
func ApplyManagedRoutes(routes []store.RouteEntry, backend string) (ApplyResult, error) {
	backend = NormalizeBackend(backend)
	if backend == BackendFRR {
		if err := frr.ApplyManaged(routes); err != nil {
			return ApplyResult{Backend: BackendFRR}, err
		}
		return ApplyResult{Backend: BackendFRR, Applied: countEnabled(routes)}, nil
	}
	return ApplyAllDiff(routes)
}
