package main

import (
	"context"
	"log"
	"os"

	"tractor.dev/toolkit-go/desktop"
	"tractor.dev/toolkit-go/engine/cli"
)

var Version = "dev"

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	root := &cli.Command{
		Version: Version,
		Usage:   "env86",
		Long:    `env86 manages v86 emulated virtual machines`,
	}

	root.AddCommand(bootCmd())
	root.AddCommand(prepareCmd())
	root.AddCommand(networkCmd())
	root.AddCommand(serveCmd())
	root.AddCommand(createCmd())

	desktop.Start(func() {
		if err := cli.Execute(context.Background(), root, os.Args[1:]); err != nil {
			log.Fatal(err)
		}
		desktop.Stop()
	})
}
