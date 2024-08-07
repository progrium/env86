package main

import (
	"flag"
	"log"
	"os/exec"

	"github.com/tarm/serial"
	"tractor.dev/toolkit-go/duplex/codec"
	"tractor.dev/toolkit-go/duplex/fn"
	"tractor.dev/toolkit-go/duplex/mux"
	"tractor.dev/toolkit-go/duplex/talk"
	"tractor.dev/toolkit-go/engine/fs"
	"tractor.dev/toolkit-go/engine/fs/osfs"
)

var Version = "dev"

func main() {
	flag.Parse()
	serialPort := flag.Arg(0)
	if serialPort == "" {
		serialPort = "/dev/ttyS1"
	}

	port, err := serial.OpenPort(&serial.Config{
		Name: serialPort,
		Baud: 115200,
	})
	if err != nil {
		log.Fatal(err)
	}

	peer := talk.NewPeer(mux.New(port), codec.CBORCodec{})
	peer.Handle("vm", fn.HandlerFrom(&API{
		FS: osfs.New(),
	}))
	log.Println("guest service running on", serialPort)
	peer.Respond()
}

type API struct {
	FS fs.FS
}

func (api *API) Version() string {
	return Version
}

// this is somewhat specific to Alpine...
func (api *API) ResetNetwork() error {
	cmd := exec.Command("sh", "-c", "rmmod ne2k-pci && modprobe ne2k-pci && hwclock -s && ifconfig lo up && hostname localhost && setup-interfaces -a -r")
	_, err := cmd.CombinedOutput()
	return err

}
