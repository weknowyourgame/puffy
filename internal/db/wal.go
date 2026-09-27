package db

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"math"
	"os"
	"strconv"
)

type Wal interface {
	Encode(name string, data []byte) (e error)
	Decode(name string) (data []byte, e error)
}

type OpType int

type Operation struct {
	Type   OpType
	ID     string
	Values []float32
}

const (
	Magic   string = "puff"
	Version uint8  = 0
)

const (
	Upsert OpType = 0
	Delete OpType = 1
)

type Entry struct {
	name string
}

func Encode(ops []Operation) ([]byte, error) {
	header, err := EncodeHeader(ops)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(header)

	for _, op := range ops {
		err := EncodeOperation(&buf, op)
		if err != nil {
			return nil, err
		}
	}

	checksum := crc32.ChecksumIEEE(buf.Bytes())

	var b4 [4]byte
	binary.LittleEndian.PutUint32(b4[:], checksum)
	buf.Write(b4[:])

	return buf.Bytes(), nil
}

func Decode(name string) {

}

func AddCheckSum() {

}

// magic + ver. + sequence + numops
func EncodeHeader(ops []Operation) ([]byte, error) {
	var buf bytes.Buffer

	last, err := LastEntry()
	if err != nil {
		return nil, err
	}

	seq := last + 1
	numOps := len(ops)

	buf.WriteString(Magic)
	buf.WriteByte(Version)

	var b8 [8]byte
	binary.LittleEndian.PutUint64(b8[:], seq)
	buf.Write(b8[:])

	var b4 [4]byte
	binary.LittleEndian.PutUint32(b4[:], uint32(numOps))
	buf.Write(b4[:])

	return buf.Bytes(), nil
}

func EncodeOperation(buf *bytes.Buffer, op Operation) error {
	buf.WriteByte(byte(op.Type))

	var b2 [2]byte
	binary.LittleEndian.PutUint16(b2[:], uint16(len(op.ID)))
	buf.Write(b2[:])

	buf.WriteString(op.ID)

	if op.Type == Upsert {
		var b4 [4]byte

		binary.LittleEndian.PutUint32(b4[:], uint32(len(op.Values)))
		buf.Write(b4[:])

		for _, value := range op.Values {
			binary.LittleEndian.PutUint32(b4[:], math.Float32bits(value))
			buf.Write(b4[:])
		}
	}

	return nil
}

// dont count all the enteries instead use the last entry
func LastEntry() (uint64, error) {
	entries, err := os.ReadDir("wal")
	if err != nil {
		return 0, err
	}

	if len(entries) == 0 {
		// no entries

		return 0, nil
	}

	n, err := strconv.ParseUint(entries[len(entries)-1].Name(), 10, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}
