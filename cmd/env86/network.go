package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/progrium/env86"
	"github.com/progrium/env86/network"

	"github.com/progrium/go-netstack/vnet"
	"tractor.dev/toolkit-go/engine/cli"
)

func networkCmd() *cli.Command {
	cmd := &cli.Command{
		Usage: "network",
		Short: "run virtual network and relay",
		// Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			vn, err := vnet.New(&vnet.Configuration{
				Debug:             false,
				MTU:               1500,
				Subnet:            "192.168.127.0/24",
				GatewayIP:         "192.168.127.1",
				GatewayMacAddress: "5a:94:ef:e4:0c:dd",
				GatewayVirtualIPs: []string{"192.168.127.253"},
			})
			if err != nil {
				log.Fatal(err)
			}

			addr := env86.ListenAddr()
			hostname := strings.ReplaceAll(addr, "0.0.0.0", "localhost")
			fmt.Printf("Network URL: ws://%s\n", hostname)
			if err := http.ListenAndServe(addr, network.Handler(vn)); err != nil {
				log.Fatal(err)
			}
		},
	}
	return cmd
}
