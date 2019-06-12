package net

import (
"fmt"
_net "net"
)

// IsPortAvailable checks if a TCP port is available or not
func IsPortAvailable(p int) bool {
	ln, err := _net.Listen("tcp", fmt.Sprintf(":%v", p))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}