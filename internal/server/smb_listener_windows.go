//go:build windows

package server

import (
	"fmt"
	"net"
	"os"

	"github.com/Microsoft/go-winio"
)

func init() {
	newSMBListener = windowsSMBListen
}

func windowsSMBListen(pipeName string) (net.Listener, error) {
	pipePath := fmt.Sprintf(`\\.\pipe\%s`, pipeName)
	os.Remove(pipePath)
	ln, err := winio.ListenPipe(pipePath, nil)
	if err != nil {
		return nil, err
	}
	return ln, nil
}
