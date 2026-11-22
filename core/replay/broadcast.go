package replay

import (
	"bytes"
	"encoding/binary"
)

// BroadcastPayload serializes the extended replay header followed by the
// compressed response stream for STOC_REPLAY.
func (r *Replay) BroadcastPayload() []byte {
	var out bytes.Buffer
	if err := binary.Write(&out, binary.LittleEndian, r.pheader); err != nil {
		return nil
	}
	out.Write(r.compressedStream())
	return out.Bytes()
}
