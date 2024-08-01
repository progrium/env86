package env86

import (
	"context"
	"fmt"

	"tractor.dev/toolkit-go/duplex/fn"
)

type Console struct {
	vm *VM
}

// func (c *Console) Show() {}
// func (c *Console) Hide() {}

func (c *Console) Screenshot() ([]byte, error) {
	if c.vm.peer == nil {
		return nil, fmt.Errorf("not ready")
	}
	var data []byte
	_, err := c.vm.peer.Call(context.TODO(), "screenshot", nil, &data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (c *Console) SetScale(x, y float64) error {
	if c.vm.peer == nil {
		return fmt.Errorf("not ready")
	}
	_, err := c.vm.peer.Call(context.TODO(), "setScale", fn.Args{x, y}, nil)
	return err
}

// TODO: not working, but webview window might not allow this
func (c *Console) SetFullscreen() error {
	if c.vm.peer == nil {
		return fmt.Errorf("not ready")
	}
	_, err := c.vm.peer.Call(context.TODO(), "setFullscreen", nil, nil)
	return err
}

// func (c *Console) EnableKeyboard()  {}
// func (c *Console) KeyboardEnabled() {}
func (c *Console) SendText(text string) error {
	if c.vm.peer == nil {
		return fmt.Errorf("not ready")
	}
	_, err := c.vm.peer.Call(context.TODO(), "sendText", fn.Args{text}, nil)
	return err
}

// func (c *Console) EnableMouse()  {}
// func (c *Console) MouseEnabled() {}
