//go:build !linux

package main

import "net"

func enableTCPQuickAck(*net.TCPConn) error {
	return nil
}
