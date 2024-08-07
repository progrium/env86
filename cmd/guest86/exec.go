package main

import (
	"io"
	"log"
	"os/exec"

	"github.com/creack/pty"
	"tractor.dev/toolkit-go/duplex/rpc"
)

type RunInput struct {
	Name string
	Args []string
	Env  []string
	Dir  string
	PTY  bool
}

type RunOutput struct {
	Stdout []byte
	Stderr []byte
	Status *int
}

func (api *API) Run(r rpc.Responder, c *rpc.Call) {
	var in RunInput
	c.Receive(&in)

	cmd := exec.Command(in.Name, in.Args...)
	cmd.Dir = in.Dir
	cmd.Env = in.Env

	var ch io.Closer
	var err error
	if in.PTY {
		ch, err = api.runPty(r, cmd)
	} else {
		ch, err = api.runNoPty(r, cmd)
	}
	if err != nil {
		r.Return(err)
		return
	}
	defer ch.Close()

	status := 0
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			status = exitErr.ExitCode()
		} else {
			log.Println(err)
		}
	}
	r.Send(RunOutput{Status: &status})
}

func (api *API) runPty(r rpc.Responder, cmd *exec.Cmd) (io.Closer, error) {
	tty, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	ch, err := r.Continue(cmd.Process.Pid)
	if err != nil {
		panic(err)
	}

	go func() {
		io.Copy(tty, ch)
	}()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := tty.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Println(err)
				}
				return
			}
			r.Send(RunOutput{Stdout: buf[:n]})
		}
	}()

	return ch, nil
}

func (api *API) runNoPty(r rpc.Responder, cmd *exec.Cmd) (io.Closer, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		panic(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}

	// todo: set group id for subprocs

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	ch, err := r.Continue(cmd.Process.Pid)
	if err != nil {
		panic(err)
	}

	go func() {
		io.Copy(stdin, ch)
		stdin.Close()
	}()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Println(err)
				}
				return
			}
			r.Send(RunOutput{Stdout: buf[:n]})
		}
	}()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				if err != io.EOF {
					log.Println(err)
				}
				return
			}
			r.Send(RunOutput{Stderr: buf[:n]})
		}
	}()

	return ch, nil
}

func (api *API) Signal(pid, sig int) error {
	// TODO
	return nil
}

func (api *API) Terminate(pid int) error {
	// TODO
	return nil
}
