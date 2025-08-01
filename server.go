package main

import (
	"flag"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/panjf2000/gnet/v2"
	"github.com/panjf2000/gnet/v2/pkg/logging"
	"github.com/sjm1327605995/goygopro/core/duel"
	"github.com/sjm1327605995/goygopro/ocgcore"
	"math"
	"sync/atomic"
	"time"
)

type Server struct {
	gnet.BuiltinEventEngine
	eng          gnet.Engine
	network      string
	addr         string
	multicore    bool
	connected    int32
	disconnected int32
	batchRead    int // maximum number of packet to read per event-loop iteration
}

func (s *Server) OnBoot(eng gnet.Engine) (action gnet.Action) {
	logging.Infof("running server on %s with multi-core=%t",
		fmt.Sprintf("%s://%s", s.network, s.addr), s.multicore)
	s.eng = eng
	return
}

func (s *Server) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {

	atomic.AddInt32(&s.connected, 1)
	codec := new(duel.SimpleCodec)
	codec.Player = &duel.DuelPlayer{
		ID:    time.Now().Format(time.RFC3339Nano),
		Game:  nil,
		Conn:  c,
		State: 0,
	}
	c.SetContext(codec)
	return
}

func (s *Server) OnClose(c gnet.Conn, err error) (action gnet.Action) {
	if err != nil {
		logging.Infof("error occurred on connection=%s, %v\n", c.RemoteAddr().String(), err)
	}
	disconnected := atomic.AddInt32(&s.disconnected, 1)
	connected := atomic.AddInt32(&s.connected, -1)
	if connected == 0 {
		logging.Infof("all %d connections are closed, shut it down", disconnected)
		action = gnet.Shutdown
	}
	return
}

func (s *Server) OnTraffic(c gnet.Conn) (action gnet.Action) {
	codec := c.Context().(*duel.SimpleCodec)
	for {
		data, finish, err := codec.Decode(c)
		if err != nil {
			return gnet.Close
		}
		if finish {
			break
		}
		if len(data) == 0 {
			return gnet.None
		}
		codec.Player.HandleCTOSPacket(data)
	}
	return
}

func main() {
	var (
		port      int
		multicore bool
		batchRead int
	)

	// Example command: go run server.go --port 9000 --multicore=true --batchread 10
	flag.IntVar(&port, "port", 9000, "--port 9000")
	flag.BoolVar(&multicore, "multicore", false, "--multicore=true")
	flag.IntVar(&batchRead, "batchread", 100, "--batch-read 100")
	flag.Parse()
	if batchRead <= 0 {
		batchRead = math.MaxInt32 // unlimited batch read
	}

	err := duel.DefaultDataManager.LoadDB("E:\\YGOPro2\\cdb\\cards.cdb")
	if err != nil {
		panic(err)
	}
	duel.DeckManger.LoadLFList()
	ss := &Server{
		network:   "tcp",
		addr:      fmt.Sprintf(":%d", port),
		multicore: multicore,
		batchRead: batchRead,
	}
	err = ocgcore.Init(ocgcore.WithRootPath("E:\\Go\\gopath\\goygopro"),
		ocgcore.WithScriptDirectory("E:\\ygopro"),
		ocgcore.WithCardReader(func(cardId uint32, card *ocgcore.CardData) uint {
			cardData := duel.DefaultDataManager.GetData(cardId)
			if cardData != nil {
				*card = *cardData
			} else {
				return 0
			}
			return uint(cardId)
		}),
	)
	if err != nil {
		panic(err)
	}
	err = gnet.Run(ss, ss.network+"://"+ss.addr, gnet.WithMulticore(multicore))
	logging.Infof("server exits with error: %v", err)
}
