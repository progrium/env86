package env86

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/progrium/env86/assets"
	"github.com/progrium/env86/namespacefs"

	"github.com/chromedp/chromedp"
	"github.com/progrium/go-netstack/vnet"
	"tractor.dev/toolkit-go/desktop"
	"tractor.dev/toolkit-go/desktop/app"
	"tractor.dev/toolkit-go/desktop/window"
	"tractor.dev/toolkit-go/duplex/fn"
	"tractor.dev/toolkit-go/duplex/talk"
	"tractor.dev/toolkit-go/engine/fs"
)

type VM struct {
	image     *Image
	config    Config
	console   *Console
	addr      string
	fsys      fs.FS
	net       *vnet.VirtualNetwork
	srv       *http.Server
	win       *window.Window
	app       *app.App
	peer      *talk.Peer
	loaded    chan bool
	stopped   chan bool
	cdpCancel func()
}

func New(image *Image, config Config) (*VM, error) {
	fsys := namespacefs.New()
	if err := fsys.Mount(assets.Dir, "/"); err != nil {
		return nil, err
	}
	if err := fsys.Mount(image.FS, "image"); err != nil {
		return nil, err
	}

	if config.WasmPath == "" {
		config.WasmPath = "/v86.wasm"
	}
	if config.Filesystem == nil {
		config.Filesystem = &FilesystemConfig{}
	}
	if config.Filesystem.BaseFS == "" {
		config.Filesystem.BaseFS = "/image/fs.json"
		config.Filesystem.BaseURL = "/image/fs/"
	}
	if config.MemorySize == 0 {
		config.MemorySize = 512 * 1024 * 1024 // 512MB
	}
	if config.VGAMemorySize == 0 {
		config.VGAMemorySize = 8 * 1024 * 1024 // 8MB
	}
	// should this be done in Image.Config()?
	if config.InitialState == nil && image.HasInitialState() && !config.ColdBoot {
		config.InitialState = image.InitialStateConfig()
	}
	if config.InitialState == nil && !image.HasInitialState() {
		config.ColdBoot = true
	}
	if config.ColdBoot {
		config.BIOS = &ImageConfig{
			URL: "/seabios.bin",
		}
		config.VGABIOS = &ImageConfig{
			URL: "/vgabios.bin",
		}
		config.BZImage = &ImageConfig{
			URL: "/vmlinuz.bin",
		}
		config.Initrd = &ImageConfig{
			URL: "/initramfs.bin",
		}
	}
	config.Autostart = true

	if config.ConsoleAddr == "" {
		config.ConsoleAddr = ListenAddr()
	}

	vm := &VM{
		image:   image,
		config:  config,
		fsys:    fsys,
		addr:    config.ConsoleAddr,
		stopped: make(chan bool),
	}
	vm.console = &Console{vm: vm}

	if config.EnableNetwork {
		var err error
		vm.net, err = vnet.New(&vnet.Configuration{
			Debug:             false,
			MTU:               1500,
			Subnet:            "192.168.127.0/24",
			GatewayIP:         "192.168.127.1",
			GatewayMacAddress: "5a:94:ef:e4:0c:dd",
			GatewayVirtualIPs: []string{"192.168.127.253"},
		})
		if err != nil {
			return nil, err
		}
		vm.config.NetworkRelayURL = fmt.Sprintf("ws://%s/net", LocalhostAddr(vm.addr))
	}

	return vm, nil
}

func (vm *VM) Start() error {
	if vm.app == nil && !vm.config.ChromeDP {
		launched := make(chan bool)
		vm.app = app.Run(app.Options{
			Accessory: true,
			Agent:     true,
		}, func() {
			launched <- true
		})
		<-launched
	}

	if vm.srv == nil {
		go vm.startHTTP()
	}

	vm.loaded = make(chan bool)
	url := fmt.Sprintf("http://%s/console.html", LocalhostAddr(vm.addr))
	if vm.config.ChromeDP {
		ctx, cancel := chromedp.NewContext(context.Background())
		vm.cdpCancel = cancel
		go func() {
			if err := chromedp.Run(ctx, chromedp.Navigate(url)); err != nil {
				log.Println(err)
			}
		}()
	} else {
		desktop.Dispatch(func() {
			vm.win = window.New(window.Options{
				Center: true,
				Hidden: vm.config.NoConsole,
				Size: window.Size{
					Width:  1004,
					Height: 785,
				},
				Resizable: true,
				URL:       url,
			})
			vm.win.Reload()
		})
	}
	<-vm.loaded
	vm.loaded = nil
	return nil
}

func (vm *VM) Close() error {
	if err := vm.Stop(); err != nil {
		return err
	}
	if vm.srv != nil {
		if err := vm.srv.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (vm *VM) Run() error {
	if err := vm.Start(); err != nil {
		return err
	}
	return vm.Wait()
}

func (vm *VM) Stop() error {
	if vm.win != nil {
		desktop.Dispatch(func() {
			vm.win.Unload()
			vm.win = nil
		})
	}
	if vm.cdpCancel != nil {
		vm.cdpCancel()
		vm.cdpCancel = nil
	}
	select {
	case vm.stopped <- true:
	default:
	}
	return nil
}

func (vm *VM) Restart() error {
	if err := vm.Stop(); err != nil {
		return err
	}
	return vm.Start()
}

func (vm *VM) Wait() error {
	// todo: make work correctly for multiple Waits
	<-vm.stopped
	return nil
}

func (vm *VM) Pause() (err error) {
	if vm.peer == nil {
		return
	}
	_, err = vm.peer.Call(context.TODO(), "pause", nil, nil)
	return
}

func (vm *VM) Unpause() (err error) {
	if vm.peer == nil {
		return
	}
	_, err = vm.peer.Call(context.TODO(), "unpause", nil, nil)
	return
}

// Save saves the state of the VM
func (vm *VM) Save() (io.Reader, error) {
	if vm.peer == nil {
		return nil, fmt.Errorf("not ready")
	}
	var b []byte
	_, err := vm.peer.Call(context.TODO(), "save", nil, &b)
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(b), nil
}

func (vm *VM) SaveInitialState() error {
	r, err := vm.Save()
	if err != nil {
		return err
	}
	return vm.image.SaveInitialState(r)
}

// Restore loads state into the VM
func (vm *VM) Restore(state io.Reader) error {
	if vm.peer == nil {
		return fmt.Errorf("not ready")
	}
	b, err := io.ReadAll(state)
	if err != nil {
		return err
	}
	_, err = vm.peer.Call(context.TODO(), "restore", fn.Args{b}, nil)
	return err
}

// Console is an API to interact with the screen, keyboard, and mouse
func (vm *VM) Console() *Console {
	return vm.console
}

// Serial returns an io.ReadWriter to the serial/COM1 port
func (vm *VM) Serial() (io.ReadWriter, error) {
	return nil, fmt.Errorf("TODO")
}

// NIC returns an io.ReadWriter of Ethernet packets to the virtual NIC
func (vm *VM) NIC() (io.ReadWriter, error) {
	return nil, fmt.Errorf("TODO")
}

func (vm *VM) MacAddress() (string, error) {
	if vm.peer == nil {
		return "", fmt.Errorf("not ready")
	}
	var mac string
	_, err := vm.peer.Call(context.TODO(), "mac", nil, &mac)
	if err != nil {
		return "", err
	}
	return mac, nil
}

func (vm *VM) Network() *vnet.VirtualNetwork {
	return vm.net
}
