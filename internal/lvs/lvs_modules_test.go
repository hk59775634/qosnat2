package lvs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKernelModuleFileExists(t *testing.T) {
	if kernelModuleFileExists("ip_vs") {
		t.Log("ip_vs present on running kernel")
		return
	}
	kver := runningKernel()
	if kver == "" {
		t.Skip("no kernel version")
	}
	dir := filepath.Join("/lib/modules", kver, "kernel", "net", "netfilter", "ipvs")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Logf("ipvs module dir missing: %s", dir)
	}
}

func TestRunningKernel(t *testing.T) {
	if runningKernel() == "" {
		t.Fatal("expected kernel version")
	}
}
