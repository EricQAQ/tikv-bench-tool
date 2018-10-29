package bench

import (
	"encoding/binary"
	"errors"
)

const signMask uint64 = 0x8000000000000000

func EncodeIntToUint(v int64) uint64 {
	return uint64(v) ^ signMask
}

func DecodeUintToInt(u uint64) int64 {
	return int64(u ^ signMask)
}

func EncodeInt(b []byte, v int64) []byte {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], EncodeIntToUint(v))
	return append(b, data[:]...)
}

func DecodeInt(b []byte) ([]byte, int64, error) {
	if len(b) < 8 {
		return nil, 0, errors.New("the length of the bytes is illegal.")
	}
	v := binary.BigEndian.Uint64(b[:8])
	b = b[8:]
	return b, DecodeUintToInt(v), nil
}
