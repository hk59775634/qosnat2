package main

import (
	"fmt"
	"log"
	"os"

	"github.com/hk59775634/qosnat2/internal/api"
)

func runSetPassword() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: qosnatd set-password <password>")
		os.Exit(2)
	}
	if err := api.ChangeAdminPassword(os.Args[2]); err != nil {
		log.Fatalf("set-password: %v", err)
	}
	fmt.Println("admin password updated")
	runStatusAfterRestart()
}
