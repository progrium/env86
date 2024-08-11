package env86

import (
	"context"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/hugelgupf/p9/fsimpl/localfs"
	"github.com/hugelgupf/p9/p9"
	"golang.org/x/net/websocket"
	"tractor.dev/toolkit-go/duplex/codec"
	"tractor.dev/toolkit-go/duplex/mux"
	"tractor.dev/toolkit-go/duplex/talk"
)

func (vm *VM) handleGuest(conn *websocket.Conn) {
	conn.PayloadType = websocket.BinaryFrame
	sess := mux.New(conn)
	defer sess.Close()

	vm.guest = &Guest{
		vm:   vm,
		peer: talk.NewPeer(sess, codec.CBORCodec{}),
	}
	vm.guest.cond = sync.NewCond(&vm.guest.mu)

	var v string
	_, err := vm.guest.peer.Call(context.TODO(), "vm.Version", nil, &v)
	if err != nil {
		log.Println("guest:", err)
		return
	}
	vm.guest.ver = v
	vm.guest.ready = true
	vm.guest.cond.Broadcast()
	vm.guest.peer.Respond()
}

type Guest struct {
	vm    *VM
	peer  *talk.Peer
	ver   string
	ready bool
	mu    sync.Mutex
	cond  *sync.Cond
}

func (g *Guest) Ready() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	for !g.ready {
		g.cond.Wait()
	}
	return g.ready
}

func (g *Guest) Version() string {
	return g.ver
}

type GuestCmd struct {
	guestRunInput
	guest  *Guest
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

func (gc *GuestCmd) Run() (status int, err error) {
	// todo: change to start, get pid and return
	resp, err := gc.guest.peer.Call(context.Background(), "vm.Run", gc.guestRunInput, nil)
	if err != nil {
		return -1, err
	}
	defer resp.Channel.Close()
	if gc.Stdin != nil {
		go func() {
			io.Copy(resp.Channel, os.Stdin)
		}()
	}
	for {
		var out guestRunOutput
		err := resp.Receive(&out)
		if err != nil {
			return -1, err
		}
		if len(out.Stdout) > 0 {
			gc.Stdout.Write(out.Stdout)
			continue
		}
		if len(out.Stderr) > 0 {
			gc.Stderr.Write(out.Stderr)
			continue
		}
		if out.Status != nil {
			return *(out.Status), nil
		}
	}
}

type guestRunInput struct {
	Name string
	Args []string
	Dir  string
	Env  []string
	PTY  bool
}

type guestRunOutput struct {
	Stdout []byte
	Stderr []byte
	Status *int
}

func (g *Guest) Command(name string, args ...string) *GuestCmd {
	return &GuestCmd{
		guestRunInput: guestRunInput{
			Name: name,
			Args: args,
			PTY:  true,
		},
		guest: g,
	}
}

func (g *Guest) ResetNetwork() error {
	_, err := g.peer.Call(context.Background(), "vm.ResetNetwork", nil, nil)
	return err
}

func (g *Guest) Mount(src, dst string) error {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return err
	}
	_, port, _ := net.SplitHostPort(l.Addr().String())
	path, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	go func() {
		srv := p9.NewServer(localfs.Attacher(path))
		if err := srv.Serve(l); err != nil {
			log.Fatal(err)
		}
	}()
	resp, err := g.peer.Call(context.Background(), "vm.Mount9P", dst, nil)
	if err != nil {
		return err
	}
	ch := resp.Channel
	conn, err := net.Dial("tcp", net.JoinHostPort("localhost", port))
	if err != nil {
		return err
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
	return ch.Close()
}

// FS(path) fs.FS
// Dial(addr) Conn, error
