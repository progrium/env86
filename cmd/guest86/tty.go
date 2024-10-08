package main

import (
	"io"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"tractor.dev/toolkit-go/duplex/rpc"
)

func (api *API) Terminal(r rpc.Responder, c *rpc.Call) {
	c.Receive(nil)

	cmd := exec.Command("/bin/sh")
	f, err := pty.Start(cmd)
	if err != nil {
		r.Return(err)
		return
	}

	ch, err := r.Continue()
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		io.Copy(ch, f)
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		io.Copy(f, ch)
		wg.Done()
	}()

	wg.Wait()
	ch.Close()
}
