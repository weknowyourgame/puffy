package db

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"math"
	"os"
	"strconv"
	"strings"
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
	Version uint16 = 0
)

const Path string = "wal"

const (
	Upsert OpType = 0
	Delete OpType = 1
)

type Entry struct {
	name string
}

func Encode(seq uint64, ops []Operation) ([]byte, error) {
	header, err := EncodeHeader(seq, ops)
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

func Decode(data []byte) (uint64, []Operation, error) {
	r := bytes.NewReader(data)

	// Magic
	magic := make([]byte, 4)
	r.Read(magic)

	// Version
	// version, _ := r.ReadByte()

	// Sequence
	var b8 [8]byte
	r.Read(b8[:])
	seq := binary.LittleEndian.Uint64(b8[:])

	// Number of operations
	var b4 [4]byte
	r.Read(b4[:])
	numOps := binary.LittleEndian.Uint32(b4[:])

	// Operations
	ops := make([]Operation, 0, numOps)

	for i := uint32(0); i < numOps; i++ {
		op, err := DecodeOperation(r)
		if err != nil {
			return 0, nil, err
		}
		ops = append(ops, op)
	}

	// Checksum
	r.Read(b4[:])
	stored := binary.LittleEndian.Uint32(b4[:])

	actual := crc32.ChecksumIEEE(data[:len(data)-4])

	if stored != actual {
		return 0, nil, errors.New("checksum mismatch")
	}

	return seq, ops, nil
}

// func AddCheckSum() {

// }

func DecodeOperation(r *bytes.Reader) (Operation, error) {
	var op Operation

	// Type
	t, err := r.ReadByte()
	if err != nil {
		return op, err
	}
	op.Type = OpType(t)

	// ID length
	var b2 [2]byte
	_, err = r.Read(b2[:])
	if err != nil {
		return op, err
	}

	idLen := binary.LittleEndian.Uint16(b2[:])

	// ID
	id := make([]byte, idLen)
	_, err = r.Read(id)
	if err != nil {
		return op, err
	}
	op.ID = string(id)

	// Values — only for Upsert
	if op.Type == Upsert {
		var b4 [4]byte

		_, err = r.Read(b4[:])
		if err != nil {
			return op, err
		}

		vectorLen := binary.LittleEndian.Uint32(b4[:])
		op.Values = make([]float32, vectorLen)

		for i := uint32(0); i < vectorLen; i++ {
			_, err = r.Read(b4[:])
			if err != nil {
				return op, err
			}

			bits := binary.LittleEndian.Uint32(b4[:])
			op.Values[i] = math.Float32frombits(bits)
		}
	}

	return op, nil
}

// magic + ver. + sequence + numops
func EncodeHeader(seq uint64, ops []Operation) ([]byte, error) {
	var buf bytes.Buffer

	// Cant rely on I/O
	// last, err := LastEntry()
	// if err != nil {
	// 	return nil, err
	// }

	seq++
	numOps := len(ops)

	buf.WriteString(Magic)

	var b2 [2]byte
	binary.LittleEndian.PutUint16(b2[:], Version)
	buf.Write(b2[:])

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
	entries, err := os.ReadDir(Path)
	if err != nil {
		return 0, err
	}

	if len(entries) == 0 {
		// no entries

		return 0, nil
	}

	entry := strings.TrimSuffix(entries[len(entries)-1].Name(), ".wal")
	n, err := strconv.ParseUint(entry, 10, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}
