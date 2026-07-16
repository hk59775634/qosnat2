package route

import "github.com/hk59775634/qosnat2/internal/store"

// MissingManaged 返回 state 中已启用但内核 FIB 中不存在的托管路由。
func MissingManaged(routes []store.RouteEntry) ([]store.RouteEntry, error) {
	idx, err := buildLiveIndex(routes)
	if err != nil {
		return nil, err
	}
	var missing []store.RouteEntry
	for _, r := range routes {
		if !r.Enabled {
			continue
		}
		entry := r
		if needsInfer(entry) {
			entry = InferRouteDevices(entry)
		}
		if routeAlreadyApplied(entry, idx) {
			continue
		}
		missing = append(missing, r)
	}
	return missing, nil
}
