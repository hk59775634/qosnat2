package api

import "testing"

func TestEvaluateWarpWatchdog_HealthyResetsFailCount(t *testing.T) {
	fail, action := evaluateWarpWatchdog(4, warpWatchdogSnapshot{Connected: true})
	if fail != 0 || action != warpWatchHealthy {
		t.Fatalf("got fail=%d action=%s want 0/healthy", fail, action)
	}
}

func TestEvaluateWarpWatchdog_DebounceUntilThreshold(t *testing.T) {
	fail, action := evaluateWarpWatchdog(0, warpWatchdogSnapshot{})
	if fail != 1 || action != warpWatchDebounce {
		t.Fatalf("1st fail: fail=%d action=%s", fail, action)
	}
	fail, action = evaluateWarpWatchdog(3, warpWatchdogSnapshot{NetnsExists: true, ServiceRunning: true})
	if fail != 4 || action != warpWatchDebounce {
		t.Fatalf("4th fail: fail=%d action=%s want debounce", fail, action)
	}
	if fail >= warpWatchdogFailThreshold {
		t.Fatal("threshold should be 5")
	}
}

func TestEvaluateWarpWatchdog_SoftReconnectAtThreshold(t *testing.T) {
	fail, action := evaluateWarpWatchdog(4, warpWatchdogSnapshot{
		NetnsExists:    true,
		ServiceRunning: true,
	})
	if fail != 5 || action != warpWatchSoftReconnect {
		t.Fatalf("got fail=%d action=%s want soft", fail, action)
	}
	// svc 短暂消失但仍有 netns：仍走软重连（可拉起 svc），不拆栈。
	fail, action = evaluateWarpWatchdog(4, warpWatchdogSnapshot{
		NetnsExists:    true,
		ServiceRunning: false,
	})
	if action != warpWatchSoftReconnect {
		t.Fatalf("netns without svc should soft reconnect, got %s", action)
	}
}

func TestEvaluateWarpWatchdog_FullReconnectOnlyWhenNetnsGone(t *testing.T) {
	fail, action := evaluateWarpWatchdog(4, warpWatchdogSnapshot{
		NetnsExists:    false,
		ServiceRunning: false,
	})
	if fail != 5 || action != warpWatchFullReconnect {
		t.Fatalf("got fail=%d action=%s want full", fail, action)
	}
}

func TestEvaluateWarpWatchdog_OpActiveDoesNotEscalate(t *testing.T) {
	fail, action := evaluateWarpWatchdog(2, warpWatchdogSnapshot{OpActive: true})
	if fail != 2 || action != warpWatchNone {
		t.Fatalf("got fail=%d action=%s want unchanged/none", fail, action)
	}
}

func TestWarpWatchdogFailThresholdIsFive(t *testing.T) {
	if warpWatchdogFailThreshold != 5 {
		t.Fatalf("threshold=%d want 5", warpWatchdogFailThreshold)
	}
}
