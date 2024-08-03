package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"github.com/progrium/env86"

	"github.com/progrium/go-netstack/vnet"
	"tractor.dev/toolkit-go/engine/cli"
)

func bootCmd() *cli.Command {
	var (
		noKeyboard  bool
		noMouse     bool
		enableTTY   bool
		noConsole   bool
		exitOn      string
		enableNet   bool
		coldBoot    bool
		saveOnExit  bool
		portForward string
		consoleURL  bool
		useCDP      bool
	)
	cmd := &cli.Command{
		Usage: "boot <image>",
		Short: "boot and run a VM",
		Args:  cli.MinArgs(1),
		Run: func(ctx *cli.Context, args []string) {
			image, err := env86.LoadImage(args[0])
			if err != nil {
				log.Fatal(err)
			}

			cfg, err := image.Config()
			if err != nil {
				log.Fatal(err)
			}
			cfg.DisableKeyboard = noKeyboard
			cfg.DisableMouse = noMouse
			cfg.ColdBoot = coldBoot
			cfg.SaveOnExit = saveOnExit
			cfg.NoConsole = noConsole
			cfg.EnableTTY = enableTTY
			cfg.ExitPattern = exitOn
			cfg.EnableNetwork = enableNet
			cfg.ChromeDP = useCDP

			cfg.ConsoleAddr = env86.ListenAddr()

			vm, err := env86.New(image, cfg)
			if err != nil {
				log.Fatal(err)
			}
			vm.Start()

			if consoleURL {
				fmt.Printf("Console URL: http://%s/console.html\n", env86.LocalhostAddr(cfg.ConsoleAddr))
			}

			if !cfg.EnableTTY {
				go func() {
					buffer := make([]byte, 1024)
					for {
						_, err := os.Stdin.Read(buffer)
						if err == io.EOF {
							vm.Exit("Ctrl-D detected")
							return
						}
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

			vm.Wait()
		},
	}
	cmd.Flags().BoolVar(&useCDP, "cdp", false, "use headless chrome")
	cmd.Flags().BoolVar(&consoleURL, "console-url", false, "show the URL to the console")
	cmd.Flags().BoolVar(&saveOnExit, "save", false, "save initial state to image on exit")
	cmd.Flags().BoolVar(&coldBoot, "cold", false, "cold boot without initial state")
	cmd.Flags().BoolVar(&enableNet, "net", false, "enable networking")
	cmd.Flags().BoolVar(&enableNet, "n", false, "enable networking (shorthand)")
	cmd.Flags().BoolVar(&noConsole, "no-console", false, "disable console window")
	cmd.Flags().BoolVar(&enableTTY, "ttyS0", false, "open TTY over serial0")
	cmd.Flags().BoolVar(&noMouse, "no-mouse", false, "disable mouse")
	cmd.Flags().BoolVar(&noKeyboard, "no-keyboard", false, "disable keyboard")
	cmd.Flags().StringVar(&exitOn, "exit-on", "", "exit when string is matched in serial TTY")
	cmd.Flags().StringVar(&portForward, "p", "", "forward TCP port (ex: 8080:80)")
	return cmd
}

func forwardPort(vn *vnet.VirtualNetwork, spec string) error {
	parts := strings.Split(spec, ":")
	l, err := net.Listen("tcp", ":"+parts[0])
	if err != nil {
		return err
	}
	defer l.Close()
	targetAddr := "192.168.127.2:" + parts[1]
	handle := func(conn net.Conn) {
		defer conn.Close()
		backend, err := vn.Dial("tcp", targetAddr)
		if err != nil {
			log.Printf("Failed to connect to target server: %v", err)
			return
		}
		defer backend.Close()
		done := make(chan struct{})
		go func() {
			io.Copy(backend, conn)
			done <- struct{}{}
		}()
		go func() {
			io.Copy(conn, backend)
			done <- struct{}{}
		}()
		<-done
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			continue
		}
		go handle(conn)
	}
}
