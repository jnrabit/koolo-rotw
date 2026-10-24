package packet

import (
	"encoding/binary"

	"github.com/hectorgimenez/d2go/pkg/data"
)

type Walk struct {
	PacketID byte
	X        uint16
	Y        uint16
}

func NewWalk(position data.Position) *Walk {
	return &Walk{
		PacketID: 0x03,
		X:        uint16(position.X),
		Y:        uint16(position.Y),
	}
}

func (p *Walk) GetPayload() []byte {
	buf := make([]byte, 5)
	buf[0] = p.PacketID
	binary.LittleEndian.PutUint16(buf[1:], p.X)
	binary.LittleEndian.PutUint16(buf[3:], p.Y)
	return buf
}
