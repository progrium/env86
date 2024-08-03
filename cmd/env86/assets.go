package main

import (
	"io"
	"log"
	"os"

	"github.com/progrium/env86/assets"

	"tractor.dev/toolkit-go/engine/cli"
)

func assetsCmd() *cli.Command {
	cmd := &cli.Command{
		Hidden: true,
		Usage:  "assets <filename>",
		Short:  "",
		Args:   cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			if args[0] == "env86.min.js" {
				b, err := assets.BundleJavaScript()
				if err != nil {
					log.Fatal(err)
				}
				os.Stdout.Write(b)
				return
			}
			f, err := assets.Dir.Open(args[0])
			if err != nil {
				log.Fatal(err)
			}
			defer f.Close()
			io.Copy(os.Stdout, f)
		},
	}
	return cmd
}
