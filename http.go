package env86

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/progrium/env86/assets"
	"github.com/progrium/env86/network"

	"golang.org/x/net/websocket"
	"golang.org/x/term"
	"tractor.dev/toolkit-go/duplex/codec"
	"tractor.dev/toolkit-go/duplex/fn"
	"tractor.dev/toolkit-go/duplex/mux"
	"tractor.dev/toolkit-go/duplex/rpc"
	"tractor.dev/toolkit-go/duplex/talk"
)

func (vm *VM) startHTTP() {
	bundle, err := assets.BundleJavaScript()
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.Handle("/net", network.Handler(vm.net))
	mux.Handle("/ctl", websocket.Handler(vm.handleControl))
	mux.Handle("/env86.min.js", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("content-type", "text/javascript")
		io.Copy(w, bytes.NewBuffer(bundle))
	}))
	mux.Handle("/", http.FileServerFS(vm.fsys))

	vm.srv = &http.Server{
		Addr:    vm.addr,
		Handler: mux,
	}
	if err := vm.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Println(err)
	}
}

func (vm *VM) handleControl(conn *websocket.Conn) {
	conn.PayloadType = websocket.BinaryFrame
	sess := mux.New(conn)
	defer sess.Close()

	vm.peer = talk.NewPeer(sess, codec.CBORCodec{})
	vm.peer.Handle("loaded", fn.HandlerFrom(func() {
		if vm.loaded != nil {
			vm.loaded <- true
		}
	}))
	vm.peer.Handle("log", rpc.HandlerFunc(func(r rpc.Responder, c *rpc.Call) {
		var args []any
		c.Receive(&args)
		log.Println(args...)
	}))
	vm.peer.Handle("config", fn.HandlerFrom(func() Config {
		return vm.config
	}))
	vm.peer.Handle("tty", rpc.HandlerFunc(func(r rpc.Responder, c *rpc.Call) {
		c.Receive(nil)
		if !vm.config.EnableTTY {
			r.Return(fmt.Errorf("tty is not enabled"))
			return
		}
		ch, err := r.Continue(nil)
		if err != nil {
			log.Println(err)
			return
		}

		go io.Copy(os.Stdout, ch)

		oldstate, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			log.Fatal(err)
		}
		defer term.Restore(int(os.Stdin.Fd()), oldstate)

		// send newline to trigger new prompt
		// since most saves will be at prompt
		if vm.image.HasInitialState() {
			io.WriteString(ch, "\n")
		}

		buffer := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buffer)
			if err != nil {
				log.Fatal("Error reading from stdin:", err)
			}

			for i := 0; i < n; i++ {
				// Check for Ctrl-D (ASCII 4)
				if buffer[i] == 4 {
					term.Restore(int(os.Stdin.Fd()), oldstate)
					if vm.config.SaveOnExit {
						fmt.Println("\r\nCtrl-D detected. Saving...")
						r, err := vm.Save()
						if err != nil {
							log.Fatal(err)
						}
						if err := vm.image.SaveInitialState(r); err != nil {
							log.Fatal(err)
						}
					} else {
						fmt.Println("\r\nCtrl-D detected. Exiting...")
					}
					ch.Close()
					vm.Stop()
					return
				}
			}

			_, err = ch.Write(buffer[:n])
			if err != nil {
				log.Println(err)
			}
		}
	}))

	vm.peer.Respond()

	// websocket closed, so we assume window was closed
	vm.win = nil
	vm.Stop()
}
