package main

import (
	"log"
	"os"
	"strings"

	"github.com/progrium/env86"
	"golang.org/x/term"

	"tractor.dev/toolkit-go/engine/cli"
)

func runCmd() *cli.Command {
	var (
		enableNet   bool
		portForward string
		useCDP      bool
		mountSpec   string
	)
	cmd := &cli.Command{
		Usage: "run <image> <cmd> [<args>...]",
		Short: "run a program in the VM (requires guest service)",
		Args:  cli.MinArgs(2),
		Run: func(ctx *cli.Context, args []string) {
			image, err := env86.LoadImage(args[0])
			if err != nil {
				log.Fatal(err)
			}

			cfg, err := image.Config()
			if err != nil {
				log.Fatal(err)
			}
			cfg.EnableNetwork = enableNet
			cfg.ChromeDP = useCDP
			cfg.ConsoleAddr = env86.ListenAddr()
			cfg.NoConsole = true

			vm, err := env86.New(image, cfg)
			if err != nil {
				log.Fatal(err)
			}
			vm.Start()

			if vm.Guest() == nil {
				log.Fatal("guest not found")
			}

			if err := vm.Guest().ResetNetwork(); err != nil {
				log.Fatal(err)
			}

			if mountSpec != "" {
				parts := strings.SplitN(mountSpec, ":", 2)
				go func() {
					if err := vm.Guest().Mount(parts[0], parts[1]); err != nil {
						log.Println(err)
					}
				}()
			}

			if enableNet {
				// todo: not sure how to decide when to do this since most of the
				// time we don't really want to ...
				if cfg.PreserveMAC {
					// this adds the vm nic to the switch route table
					// so we can immediately dial the vm nic in port forwarding
					vm.Console().SendText("ping -c 1 192.168.127.1\n")
				}

				if portForward != "" {
					go forwardPort(vm.Network(), portForward)
				}
			}

			cmd := vm.Guest().Command(args[1], args[2:]...)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			oldstate, err := term.MakeRaw(int(os.Stdin.Fd()))
			if err != nil {
				log.Fatal(err)
			}
			status, err := cmd.Run()
			term.Restore(int(os.Stdin.Fd()), oldstate)
			if err != nil {
				log.Fatal(err)
			}
			vm.Stop()
			os.Exit(status)
		},
	}
	cmd.Flags().BoolVar(&useCDP, "cdp", false, "use headless chrome")
	cmd.Flags().BoolVar(&enableNet, "net", false, "enable networking")
	cmd.Flags().BoolVar(&enableNet, "n", false, "enable networking (shorthand)")
	cmd.Flags().StringVar(&portForward, "p", "", "forward TCP port (ex: 8080:80)")
	cmd.Flags().StringVar(&mountSpec, "m", "", "mount a directory (ex: .:/mnt/host)")
	return cmd
}
