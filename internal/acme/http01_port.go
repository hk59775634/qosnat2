package acme

import "fmt"

// HTTP01PortHook 在 HTTP-01 校验期间临时放开/恢复防火墙 tcp/80（由 api 注册）。
// target 为域名或公网 IPv4；open=false 时 target 可忽略。
var HTTP01PortHook func(open bool, target string) error

// SetHTTP01PortHook 注册防火墙回调；open=true 放开，open=false 恢复。
func SetHTTP01PortHook(fn func(open bool, target string) error) {
	HTTP01PortHook = fn
}

func withHTTP01PortOpen(target string, fn func() error) error {
	if fn == nil {
		return fmt.Errorf("nil fn")
	}
	if HTTP01PortHook == nil {
		return fn()
	}
	if err := HTTP01PortHook(true, target); err != nil {
		return fmt.Errorf("open tcp/80 for http-01: %w", err)
	}
	defer func() { _ = HTTP01PortHook(false, target) }()
	return fn()
}
