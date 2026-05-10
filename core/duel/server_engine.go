package duel

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/panjf2000/gnet/v2"
	"github.com/panjf2000/gnet/v2/pkg/logging"
)

// Server is the gnet event engine for YGOPro duel server.
// Extracted from cmd/server/server.go to allow embedded server usage.
type Server struct {
	gnet.BuiltinEventEngine
	eng          gnet.Engine
	network      string
	addr         string
	multicore    bool
	connected    int32
	disconnected int32
}

func (s *Server) OnBoot(eng gnet.Engine) (action gnet.Action) {
	logging.Infof("running server on %s with multi-core=%t",
		fmt.Sprintf("%s://%s", s.network, s.addr), s.multicore)
	s.eng = eng
	NetServerEngine = &eng
	return
}

func (s *Server) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	if !AcceptingConnections {
		return nil, gnet.Close
	}
	atomic.AddInt32(&s.connected, 1)
	codec := new(SimpleCodec)
	codec.Player = &DuelPlayer{
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
	atomic.AddInt32(&s.disconnected, 1)
	atomic.AddInt32(&s.connected, -1)
	// Note: we do NOT shut down the server when all connections close,
	// because this is an embedded server that should stay alive until
	// explicitly stopped (e.g. when the client application exits).
	return
}

func (s *Server) OnTraffic(c gnet.Conn) (action gnet.Action) {
	codec := c.Context().(*SimpleCodec)
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

// StartDuelServer starts the gnet TCP server.
func StartDuelServer(port int, multicore bool) error {
	ss := &Server{
		network:   "tcp",
		addr:      fmt.Sprintf(":%d", port),
		multicore: multicore,
	}
	return gnet.Run(ss, ss.network+"://"+ss.addr, gnet.WithMulticore(multicore))
}
