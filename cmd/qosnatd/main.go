package main

import (
	"log"
	"os"

	"github.com/hk59775634/qosnat2/internal/api"
	"github.com/hk59775634/qosnat2/internal/ebpf"
	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/store"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "apply-state":
			runApplyState()
			return
		case "acme-ip-ssl":
			runAcmeIPSSL()
			return
		}
	}
	runServer()
}

func runApplyState() {
	env := api.LoadEnv()
	if err := api.ValidateEnv(env); err != nil {
		log.Fatalf("config: %v", err)
	}
	st := store.New(env.StateFile)
	if err := st.Load(); err != nil {
		log.Fatalf("load state: %v", err)
	}
	_ = netif.EnsureIFB()
	bpfM := ebpf.New()
	if err := bpfM.Load(); err != nil {
		log.Printf("ebpf load: %v", err)
	}
	srv := api.New(env, st, bpfM)
	if !st.Get().SetupComplete {
		log.Fatalf("apply-state: initial setup not complete (open Web UI wizard)")
	}
	log.Printf("apply-state: sysctl+tc+nft LAN=%s WAN=%s", env.DevLAN, env.DevWAN)
	if err := srv.ApplyAll(); err != nil {
		log.Fatalf("apply: %v", err)
	}
	log.Println("state applied")
}

func runServer() {
	env := api.LoadEnv()
	if err := api.ValidateEnv(env); err != nil {
		log.Fatalf("config: %v", err)
	}
	st := store.New(env.StateFile)
	if err := st.Load(); err != nil {
		log.Fatalf("load state: %v", err)
	}
	if err := st.Save(); err != nil {
		log.Printf("init state file: %v", err)
	}
	_ = netif.EnsureIFB()
	bpfM := ebpf.New()
	if err := bpfM.Load(); err != nil {
		log.Printf("ebpf load (P1): %v — 请确认已 make bpf 且 /usr/lib/qosnat2/classify.bpf.o 存在", err)
	} else {
		defer bpfM.Close()
	}
	srv := api.New(env, st, bpfM)
	srv.ReconcileTLSOnBoot()
	if err := srv.ApplyAll(); err != nil {
		log.Printf("apply on start: %v", err)
	}
	srv.StartBackground()
	if err := srv.Listen(); err != nil {
		log.Fatal(err)
	}
}
