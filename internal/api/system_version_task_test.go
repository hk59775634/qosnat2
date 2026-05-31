package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hk59775634/qosnat2/internal/releasecatalog"
)

func TestReconcileStaleVersionSwitchRunningApplied(t *testing.T) {
	dir := t.TempDir()
	restore := swapVersionSwitchPaths(t, dir)
	defer restore()

	versionSwitchMu.Lock()
	versionSwitchRunning = false
	versionSwitchMu.Unlock()

	target := "2026053107"
	_ = os.WriteFile(filepath.Join(dir, "release-tag"), []byte(target+"\n"), 0644)
	saveVersionSwitchStatus(versionSwitchStatus{
		State:     warpInstallStateRunning,
		Message:   "restarting qosnatd",
		TargetTag: target,
		StartedAt: "2026-05-31T10:00:00Z",
	})

	st := getVersionSwitchStatus()
	if st.State != warpInstallStateOK {
		t.Fatalf("state = %q, want ok", st.State)
	}
	if releasecatalog.NormalizeID(st.TargetTag) != target {
		t.Fatalf("target_tag = %q", st.TargetTag)
	}
}

func TestReconcileStaleVersionSwitchRunningFailed(t *testing.T) {
	dir := t.TempDir()
	restore := swapVersionSwitchPaths(t, dir)
	defer restore()

	versionSwitchMu.Lock()
	versionSwitchRunning = false
	versionSwitchMu.Unlock()

	_ = os.WriteFile(filepath.Join(dir, "release-tag"), []byte("2026053106\n"), 0644)
	saveVersionSwitchStatus(versionSwitchStatus{
		State:     warpInstallStateRunning,
		Message:   "downloading release",
		TargetTag: "2026053107",
		StartedAt: "2026-05-31T10:00:00Z",
	})

	st := getVersionSwitchStatus()
	if st.State != warpInstallStateFailed {
		t.Fatalf("state = %q, want failed", st.State)
	}
}

func swapVersionSwitchPaths(t *testing.T, dir string) func() {
	t.Helper()
	oldStatus := versionSwitchStatusFile
	oldTag := qosnatReleaseTag
	versionSwitchStatusFile = filepath.Join(dir, "version-switch-status.json")
	qosnatReleaseTag = filepath.Join(dir, "release-tag")
	return func() {
		versionSwitchStatusFile = oldStatus
		qosnatReleaseTag = oldTag
	}
}
