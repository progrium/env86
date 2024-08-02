package main

import (
	"io"
	"log"
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
		log.Println(err.Error())
		return
	}
	ch, err := r.Continue(nil)
	if err != nil {
		log.Println(err.Error())
		panic(err)
	}
	var wg sync.WaitGroup
	go func() {
		io.Copy(ch, f)
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		io.Copy(f, ch)
		wg.Done()
	}()
	wg.Add(1)
	wg.Wait()
	ch.Close()
}
