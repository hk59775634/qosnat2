package main

import (
	"fmt"
	"os"
)

func isHelpArg(s string) bool {
	return s == "-h" || s == "--help" || s == "help"
}

func printHelp() {
	fmt.Fprintf(os.Stdout, `qosnatd - qosnat2 Web control plane

Usage:
  qosnatd                         run HTTP/HTTPS server (default)
  qosnatd <command> [args]

Commands:
  status                          show URL, username, password, listen port
  set-port <port>                 change admin listen port and restart
  set-user <username>             change admin username and restart
  set-password <password>         change admin password and restart
  apply-state                     apply sysctl+tc+nft from state.json
  acme-ip-ssl [flags]             request Let's Encrypt IP certificate
                                  flags: --ip, --email, --staging

Options:
  -h, --help                      show this help

`)
}
