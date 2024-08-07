package main

import (
	"io"
	"net"
	"sync"

	"tractor.dev/toolkit-go/duplex/rpc"
)

func (api *API) Dial(r rpc.Responder, c *rpc.Call) {
	var addr string
	c.Receive(&addr)

	conn, err := net.Dial("tcp", addr)
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
		io.Copy(ch, conn)
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		io.Copy(conn, ch)
		wg.Done()
	}()

	wg.Wait()
	ch.Close()
}
