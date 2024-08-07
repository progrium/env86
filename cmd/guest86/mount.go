package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"sync"

	"tractor.dev/toolkit-go/duplex/rpc"
)

func (api *API) Mount9P(r rpc.Responder, c *rpc.Call) {
	var mountPath string
	c.Receive(&mountPath)
	l, err := net.Listen("tcp4", ":0")
	if err != nil {
		log.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(l.Addr().String())
	os.MkdirAll(mountPath, 0755)
	go func() {
		cmd := exec.Command("mount", "-t", "9p", "-o", fmt.Sprintf("trans=tcp,port=%s", port), "127.0.0.1", mountPath)
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	}()
	conn, err := l.Accept()
	if err != nil {
		log.Fatal(err)
	}
	ch, err := r.Continue(nil)
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

func (api *API) MountFuse(path, selector string) error {
	return nil
}
