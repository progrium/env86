package env86

import (
	"bytes"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

func (vm *VM) handleTTY(ch io.ReadWriteCloser) {
	if vm.serialPipe != nil {
		vm.joinSerialPipe(ch)
		return
	}

	oldstate, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatal(err)
	}

	var outReader io.Reader = ch
	if vm.config.ExitPattern != "" {
		r, w := io.Pipe()
		outReader = io.TeeReader(ch, w)
		go func() {
			exitPattern := []byte(vm.config.ExitPattern)
			buffer := make([]byte, 1024)
			temp := bytes.NewBuffer(nil)
			for {
				n, err := r.Read(buffer)
				if n > 0 {
					temp.Write(buffer[:n])
					if bytes.Contains(temp.Bytes(), exitPattern) {
						<-time.After(200 * time.Millisecond) // give moment for stdout to flush
						term.Restore(int(os.Stdin.Fd()), oldstate)
						ch.Close()
						vm.Exit("Exit pattern detected")
						return
					}
					// Keep only the last len(pattern)-1 bytes in temp to handle patterns spanning chunks
					if temp.Len() > len(exitPattern) {
						temp.Next(temp.Len() - len(exitPattern) + 1)
					}
				}
				if err != nil {
					if err != io.EOF {
						log.Println(err)
					}
					break
				}
			}
		}()
	}
	go io.Copy(os.Stdout, outReader)

	// send newline to trigger new prompt
	// since most saves will be at prompt
	if vm.image.HasInitialState() {
		io.WriteString(ch, "\n")
	}

	buffer := make([]byte, 1024)
	for {
		n, err := os.Stdin.Read(buffer)
		if err != nil {
			term.Restore(int(os.Stdin.Fd()), oldstate)
			log.Fatal("Error reading from stdin:", err)
		}

		for i := 0; i < n; i++ {
			// Check for Ctrl-D (ASCII 4)
			if buffer[i] == 4 {
				term.Restore(int(os.Stdin.Fd()), oldstate)
				ch.Close()
				vm.Exit("Ctrl-D detected")
				return
			}
		}

		_, err = ch.Write(buffer[:n])
		if err != nil {
			log.Println(err)
		}
	}
}

func (vm *VM) joinSerialPipe(ch io.ReadWriteCloser) {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		io.Copy(ch, vm.serialPipe)
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		io.Copy(vm.serialPipe, ch)
		wg.Done()
	}()

	wg.Wait()
}
