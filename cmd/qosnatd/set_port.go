package main

import (
	"fmt"
	"log"
	"os"

	"github.com/hk59775634/qosnat2/internal/api"
)

func runSetPort() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: qosnatd set-port <port>")
		os.Exit(2)
	}
	port, err := api.ChangeAdminPort(os.Args[2])
	if err != nil {
		log.Fatalf("set-port: %v", err)
	}
	fmt.Printf("admin port set to %s\n", port)
	runStatusAfterRestart()
}
