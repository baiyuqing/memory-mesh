//go:build linux

package main

import (
	"net"
	"syscall"
)

func enableTCPQuickAck(conn *net.TCPConn) error {
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return err
	}

	var controlErr error
	if err := rawConn.Control(func(fd uintptr) {
		controlErr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_QUICKACK, 1)
	}); err != nil {
		return err
	}
	if controlErr != nil && controlErr != syscall.ENOPROTOOPT {
		return controlErr
	}
	return nil
}
