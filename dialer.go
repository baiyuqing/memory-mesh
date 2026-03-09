package main

import (
	"context"
	"database/sql"
	"net"
	"sync"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

const (
	optimizedTCPNetworkName  = "mysqlbench-tcp"
	optimizedTCP4NetworkName = "mysqlbench-tcp4"
	optimizedTCP6NetworkName = "mysqlbench-tcp6"
)

var optimizedTCPDialOnce = map[string]*sync.Once{
	"tcp":  new(sync.Once),
	"tcp4": new(sync.Once),
	"tcp6": new(sync.Once),
}

func openDB(cfg config) (*sql.DB, error) {
	driverCfg, err := buildMySQLDriverConfig(cfg.dsn)
	if err != nil {
		return nil, err
	}
	connector, err := mysqlDriver.NewConnector(driverCfg)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(connector), nil
}

func buildMySQLDriverConfig(dsn string) (*mysqlDriver.Config, error) {
	driverCfg, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	if !isTCPNetwork(driverCfg.Net) {
		return driverCfg, nil
	}
	driverCfg.Net = ensureOptimizedTCPDialContext(driverCfg.Net)
	return driverCfg, nil
}

func isTCPNetwork(network string) bool {
	return network == "tcp" || network == "tcp4" || network == "tcp6"
}

func ensureOptimizedTCPDialContext(network string) string {
	customNetwork, ok := optimizedTCPNetworkNameFor(network)
	if !ok {
		return network
	}
	optimizedTCPDialOnce[network].Do(func() {
		mysqlDriver.RegisterDialContext(customNetwork, func(ctx context.Context, addr string) (net.Conn, error) {
			return dialOptimizedTCP(ctx, network, addr)
		})
	})
	return customNetwork
}

func optimizedTCPNetworkNameFor(network string) (string, bool) {
	switch network {
	case "tcp":
		return optimizedTCPNetworkName, true
	case "tcp4":
		return optimizedTCP4NetworkName, true
	case "tcp6":
		return optimizedTCP6NetworkName, true
	default:
		return "", false
	}
}

func dialOptimizedTCP(ctx context.Context, network, addr string) (net.Conn, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return conn, nil
	}
	if err := tcpConn.SetNoDelay(true); err != nil {
		_ = conn.Close()
		return nil, err
	}
	_ = enableTCPQuickAck(tcpConn)
	return conn, nil
}
