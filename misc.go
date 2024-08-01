package env86

import (
	"net"
	"strings"
)

func ListenAddr() string {
	ln, err := net.Listen("tcp4", ":0")
	if err != nil {
		panic(err)
	}
	ln.Close()
	return ln.Addr().String()
}

func LocalhostAddr(addr string) string {
	return strings.ReplaceAll(addr, "0.0.0.0", "localhost")
}
