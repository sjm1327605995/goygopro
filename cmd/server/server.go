package main

import (
	"flag"
	_ "github.com/mattn/go-sqlite3"
	"github.com/panjf2000/gnet/v2/pkg/logging"
	"github.com/sjm1327605995/goygopro/core/duel"
)

func main() {
	var (
		port       int
		multicore  bool
		dbPath     string
		scriptPath string
		rootPath   string
	)
	// Example command: go run server.go --port 9000 --multicore=true --db=cards.cdb --script=script --root=.
	flag.IntVar(&port, "port", 9000, "server listening port")
	flag.BoolVar(&multicore, "multicore", false, "enable gnet multi-core event loops")
	flag.StringVar(&dbPath, "db", "cards.cdb", "path to cards.cdb SQLite database")
	flag.StringVar(&scriptPath, "script", "script", "path to lua scripts directory")
	flag.StringVar(&rootPath, "root", ".", "root directory containing ocgcore library")
	flag.Parse()

	err := duel.InitServerData(dbPath, scriptPath, rootPath)
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
