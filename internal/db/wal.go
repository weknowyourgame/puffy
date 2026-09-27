package db

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

type Wal interface {
	Encode(seq uint64, ops []Operation) ([]byte, error)
	Decode(data []byte) (uint64, []Operation, error)
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
	Upsert OpType = iota
	Delete
)

// magic(4) + version(2) + seq(8) + numOps(4) + checksum(4)
const minRecordLen = 4 + 2 + 8 + 4 + 4

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

	// Encode operations
	for _, op := range ops {
		// Encode op types & values
		err := EncodeOperation(&buf, op)
		if err != nil {
			return nil, err
		}
	}

	// Encode checksum
	checksum := crc32.ChecksumIEEE(buf.Bytes())

	var b4 [4]byte
	binary.LittleEndian.PutUint32(b4[:], checksum)
	buf.Write(b4[:])

	return buf.Bytes(), nil
}

func Decode(data []byte) (uint64, []Operation, error) {
	if len(data) < minRecordLen {
		return 0, nil, errors.New("record too short")
	}

	r := bytes.NewReader(data)

	// Magic
	magic := make([]byte, 4)
	if _, err := io.ReadFull(r, magic); err != nil {
		return 0, nil, err
	}
	if string(magic) != Magic {
		return 0, nil, errors.New("bad magic: not a wal record")
	}

	// Version
	var b2 [2]byte
	if _, err := io.ReadFull(r, b2[:]); err != nil {
		return 0, nil, err
	}
	version := binary.LittleEndian.Uint16(b2[:])
	if version != Version {
		return 0, nil, errors.New("unsupported wal version")
	}

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
		// Decode op types & values
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

	if int64(idLen) > int64(r.Len()) {
		return op, errors.New("id length exceeds remaining data")
	}

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

		if int64(vectorLen) > int64(r.Len())/4 {
			return op, errors.New("vector length exceeds remaining data")
		}

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

	// allocating the sequence number isn't the encoder's job
	// seq++
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
