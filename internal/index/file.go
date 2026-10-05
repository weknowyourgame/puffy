package index

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/weknowyourgame/puffer/internal/store"
)

// See docs/index-format.md for the layout of these two files.

const (
	Magic   string = "pufi"
	Version uint16 = 0
)

// magic(4) + version(2) + seq(8) + k(4) + dim(4)
const headerLen = 4 + 2 + 8 + 4 + 4

func centroidsName(seq uint64) string { return fmt.Sprintf("index/%020d/centroids.bin", seq) }
func clustersName(seq uint64) string  { return fmt.Sprintf("index/%020d/clusters.bin", seq) }

func encodeHeader(seq uint64, k, dim int) []byte {
	b := make([]byte, 0, headerLen)
	b = append(b, Magic...)
	b = binary.LittleEndian.AppendUint16(b, Version)
	b = binary.LittleEndian.AppendUint64(b, seq)
	b = binary.LittleEndian.AppendUint32(b, uint32(k))
	b = binary.LittleEndian.AppendUint32(b, uint32(dim))
	return b
}

func decodeHeader(b []byte) (seq uint64, k int, dim int, err error) {
	if len(b) < headerLen {
		return 0, 0, 0, errors.New("index header too short")
	}
	if string(b[0:4]) != Magic {
		return 0, 0, 0, errors.New("bad magic: not an index file")
	}
	if binary.LittleEndian.Uint16(b[4:6]) != Version {
		return 0, 0, 0, errors.New("unsupported index version")
	}
	seq = binary.LittleEndian.Uint64(b[6:14])
	k = int(binary.LittleEndian.Uint32(b[14:18]))
	dim = int(binary.LittleEndian.Uint32(b[18:22]))
	return seq, k, dim, nil
}

/*
Write saves an index built in memory as two files, covering the WAL up to seq.
clusters.bin goes first and centroids.bin last: a crash half way leaves
no centroids.bin, and Open ignores an index without one.
*/
func (idx *Index) Write(s store.Store, seq uint64) error {
	if idx.clusterIDs == nil {
		return errors.New("can only write an index that was built in memory")
	}
	idx.seq = seq

	// Pack every cluster first so we know how long each one is.
	blocks := make([][]byte, idx.k)
	for c := 0; c < idx.k; c++ {
		ids := idx.clusterIDs[c]
		vecs := idx.clusterData[c]

		var b []byte
		b = binary.LittleEndian.AppendUint32(b, uint32(len(ids)))
		for i, id := range ids {
			b = binary.LittleEndian.AppendUint16(b, uint16(len(id)))
			b = append(b, id...)
			for _, v := range vecs[i*idx.dim : (i+1)*idx.dim] {
				b = binary.LittleEndian.AppendUint32(b, math.Float32bits(v))
			}
		}
		blocks[c] = b
	}

	// header, then k+1 offsets (cluster c lives at [offsets[c], offsets[c+1])), then the clusters
	out := encodeHeader(seq, idx.k, idx.dim)
	offset := uint64(headerLen + 8*(idx.k+1))
	for c := 0; c < idx.k; c++ {
		out = binary.LittleEndian.AppendUint64(out, offset)
		offset += uint64(len(blocks[c]))
	}
	out = binary.LittleEndian.AppendUint64(out, offset)
	for _, b := range blocks {
		out = append(out, b...)
	}
	if err := s.Create(clustersName(seq), out); err != nil {
		return err
	}

	cen := encodeHeader(seq, idx.k, idx.dim)
	for _, v := range idx.centroids {
		cen = binary.LittleEndian.AppendUint32(cen, math.Float32bits(v))
	}
	return s.Create(centroidsName(seq), cen)
}

// Open loads the newest index in the store, or returns ErrNoIndex.
// Only the centroids and the offset table are read now; each cluster is
// read with ReadAt when a query needs it.
func Open(s store.Store) (*Index, error) {
	keys, err := s.List("index/")
	if err != nil {
		return nil, err
	}

	// keys are sorted and named by sequence number, so the last centroids file is the newest
	latest := ""
	for _, key := range keys {
		if strings.HasSuffix(key, "/centroids.bin") {
			latest = key
		}
	}
	if latest == "" {
		return nil, ErrNoIndex
	}

	dir := strings.TrimSuffix(strings.TrimPrefix(latest, "index/"), "/centroids.bin")
	seq, err := strconv.ParseUint(dir, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", latest, err)
	}

	cen, err := s.Read(centroidsName(seq))
	if err != nil {
		return nil, err
	}
	hseq, k, dim, err := decodeHeader(cen)
	if err != nil {
		return nil, err
	}
	if hseq != seq {
		return nil, errors.New("centroids.bin: sequence does not match its folder")
	}
	if len(cen) != headerLen+k*dim*4 {
		return nil, errors.New("centroids.bin: wrong size")
	}
	centroids := make([]float32, k*dim)
	for i := range centroids {
		bits := binary.LittleEndian.Uint32(cen[headerLen+i*4:])
		centroids[i] = math.Float32frombits(bits)
	}

	// header + offset table of clusters.bin
	table, err := s.ReadAt(clustersName(seq), 0, headerLen+8*(k+1))
	if err != nil {
		return nil, err
	}
	cseq, ck, cdim, err := decodeHeader(table)
	if err != nil {
		return nil, err
	}
	if cseq != seq || ck != k || cdim != dim {
		return nil, errors.New("clusters.bin does not match centroids.bin")
	}
	offsets := make([]uint64, k+1)
	for i := range offsets {
		offsets[i] = binary.LittleEndian.Uint64(table[headerLen+i*8:])
	}

	idx := &Index{centroids: centroids, dim: dim, k: k, seq: seq}
	idx.loadCluster = func(c int) ([]string, []float32, error) {
		b, err := s.ReadAt(clustersName(seq), int64(offsets[c]), int(offsets[c+1]-offsets[c]))
		if err != nil {
			return nil, nil, err
		}
		return decodeCluster(b, dim)
	}
	return idx, nil
}

func decodeCluster(b []byte, dim int) ([]string, []float32, error) {
	if len(b) < 4 {
		return nil, nil, errors.New("cluster too short")
	}
	count := int(binary.LittleEndian.Uint32(b[0:4]))
	pos := 4

	ids := make([]string, 0, count)
	vecs := make([]float32, 0, count*dim)
	for i := 0; i < count; i++ {
		if pos+2 > len(b) {
			return nil, nil, errors.New("cluster is cut short")
		}
		idLen := int(binary.LittleEndian.Uint16(b[pos:]))
		pos += 2
		if pos+idLen+dim*4 > len(b) {
			return nil, nil, errors.New("cluster is cut short")
		}
		ids = append(ids, string(b[pos:pos+idLen]))
		pos += idLen
		for j := 0; j < dim; j++ {
			vecs = append(vecs, math.Float32frombits(binary.LittleEndian.Uint32(b[pos:])))
			pos += 4
		}
	}
	return ids, vecs, nil
}
