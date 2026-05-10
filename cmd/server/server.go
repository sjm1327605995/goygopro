package main

import (
	"flag"
	_ "github.com/mattn/go-sqlite3"
	"github.com/panjf2000/gnet/v2/pkg/logging"
	"github.com/sjm1327605995/goygopro/core/duel"
)

func main() {
	var (
		port      int
		multicore bool
	)
	// Example command: go run server.go --port 9000 --multicore=true
	flag.IntVar(&port, "port", 9000, "--port 9000")
	flag.BoolVar(&multicore, "multicore", false, "--multicore=true")
	flag.Parse()

	err := duel.InitServerData("E:\\ygopro\\cards.cdb", "E:\\ygo", "E:\\Go\\gopath\\goygopro")
	if err != nil {
		panic(err)
	}

	bs, err := duel.StartBroadcast(uint16(port))
	if err != nil {
		logging.Infof("failed to start broadcast: %v", err)
	} else {
		duel.BroadcastInstance = bs
		defer bs.Stop()
	}

	err = duel.StartDuelServer(port, multicore)
	logging.Infof("server exits with error: %v", err)
}
