package utp

import (
	"context"

	"v2ray.com/core/common"
	"v2ray.com/core/common/dice"
	"v2ray.com/core/common/serial"
)

type UTP struct {
	header       byte
	extension    byte
	connectionId uint16
}

func (*UTP) Size() int32 {
	return 4
}

// Write implements io.Writer.
func (u *UTP) Write(b []byte) (int, error) {
	serial.Uint16ToBytes(u.connectionId, b[:0])
	b[2] = u.header
	b[3] = u.extension
	return 4, nil
}

// New creates a new UTP header for the given config.
func New(ctx context.Context, config interface{}) (interface{}, error) {
	return &UTP{
		header:       1,
		extension:    0,
		connectionId: dice.RollUint16(),
	}, nil
}

func init() {
	common.Must(common.RegisterConfig((*Config)(nil), New))
}
