// Command api starts the ottoplus control plane API server.
//
// The server exposes the block catalog, composition validation,
// auto-wiring, and topology endpoints. It registers the minimum
// block set (storage.local-pv, datastore.postgresql, gateway.pgbouncer)
// needed for a functional demo.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/baiyuqing/ottoplus/src/api"
	"github.com/baiyuqing/ottoplus/src/core/block"
	"github.com/baiyuqing/ottoplus/src/operator/blocks/datastore/postgresql"
	"github.com/baiyuqing/ottoplus/src/operator/blocks/gateway/pgbouncer"
	localpv "github.com/baiyuqing/ottoplus/src/operator/blocks/storage/local-pv"
)

func main() {
	addr := flag.String("addr", ":8080", "Listen address for the API server.")
	flag.Parse()

	registry := block.NewRegistry()

	// Register the minimum block set for Phase 1.
	for _, b := range []block.Block{
		&localpv.Block{},
		&postgresql.Block{},
		&pgbouncer.Block{},
	} {
		if err := registry.Register(b); err != nil {
			fmt.Fprintf(os.Stderr, "register block: %v\n", err)
			os.Exit(1)
		}
	}

	srv := api.NewServer(registry)

	log.Printf("ottoplus API server listening on %s", *addr)
	if err := http.ListenAndServe(*addr, srv.Handler()); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
