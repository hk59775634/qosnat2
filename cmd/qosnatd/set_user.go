package main

import (
	"fmt"
	"log"
	"os"

	"github.com/hk59775634/qosnat2/internal/api"
)

func runSetUser() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: qosnatd set-user <username>")
		os.Exit(2)
	}
	user, err := api.ChangeAdminUser(os.Args[2])
	if err != nil {
		log.Fatalf("set-user: %v", err)
	}
	fmt.Printf("admin user set to %s\n", user)
	runStatusAfterRestart()
}
