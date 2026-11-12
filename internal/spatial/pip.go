package spatial

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/resources"
)

const (
	DefaultSpatialAssetsDir = "/home/bruno/Work/spatial_assets"
	CurrentBECGISVersion    = 13
)

type pmtilesHeaderV3 struct {
	rootDirOffset       uint64
	rootDirBytes        uint64
	jsonMetadataOffset  uint64
	jsonMetadataBytes   uint64
	leafDirsOffset      uint64
	leafDirsBytes       uint64
	tileDataOffset      uint64
	tileDataBytes       uint64
	addressedTilesCount uint64
	tileEntriesCount    uint64
	tileContentsCount   uint64
	clustered           uint8
	internalCompression uint8 // 1 = none, 2 = gzip
	tileCompression     uint8 // 1 = none, 2 = gzip
	tileType            uint8 // 1 = mvt
	minZoom             uint8
	maxZoom             uint8
}

type pmtilesDirEntry struct {
	tileID    uint64
	offset    uint64
	length    uint32
	runLength uint32
}

type mvtDecodedFeature struct {
	id         uint64
	geomType   uint32 // 1=Point, 2=LineString, 3=Polygon
	rings      [][][2]float64
	bbox       [4]float64 // minX, minY, maxX, maxY in tile coordinates
	properties map[string]any
}

type mvtDecodedLayer struct {
	name     string
	extent   uint32
	features []mvtDecodedFeature
}

type pmtilesReader struct {
	sourceLabel string
	readerAt    io.ReaderAt
	closer      io.Closer
	header      pmtilesHeaderV3
	rootEntries []pmtilesDirEntry

	mu        sync.Mutex
	leafCache map[uint64][]pmtilesDirEntry
	tileCache map[uint64]map[string]*mvtDecodedLayer
}

func parsePMTilesHeaderV3(buf []byte) (pmtilesHeaderV3, error) {
	if len(buf) < 127 || string(buf[:7]) != "PMTiles" || buf[7] != 3 {
		return pmtilesHeaderV3{}, errors.New("invalid PMTiles v3 header magic")
	}
	return pmtilesHeaderV3{
		rootDirOffset:       binary.LittleEndian.Uint64(buf[8:16]),
		rootDirBytes:        binary.LittleEndian.Uint64(buf[16:24]),
		jsonMetadataOffset:  binary.LittleEndian.Uint64(buf[24:32]),
		jsonMetadataBytes:   binary.LittleEndian.Uint64(buf[32:40]),
		leafDirsOffset:      binary.LittleEndian.Uint64(buf[40:48]),
		leafDirsBytes:       binary.LittleEndian.Uint64(buf[48:56]),
		tileDataOffset:      binary.LittleEndian.Uint64(buf[56:64]),
		tileDataBytes:       binary.LittleEndian.Uint64(buf[64:72]),
		addressedTilesCount: binary.LittleEndian.Uint64(buf[72:80]),
		tileEntriesCount:    binary.LittleEndian.Uint64(buf[80:88]),
		tileContentsCount:   binary.LittleEndian.Uint64(buf[88:96]),
		clustered:           buf[96],
		internalCompression: buf[97],
		tileCompression:     buf[98],
		tileType:            buf[99],
		minZoom:             buf[100],
		maxZoom:             buf[101],
	}, nil
}

// Wait — let's double-check header byte offsets [8..127]!
// Earlier we unpacked `hdr[8:127]` with `<11QBBBBBB4iB2i`:
// 11 * 8 = 88 bytes -> [8 .. 96)!
// So:
// [8..16]: root_off (1)
// [16..24]: root_len (2)
// [24..32]: meta_off (3)
// [32..40]: meta_len (4)
// [40..48]: leaf_off (5)
// [48..56]: leaf_len (6)
// [56..64]: data_off (7)
// [64..72]: data_len (8)
// [72..80]: addr_cnt (9)
// [80..88]: entry_cnt (10)
// [88..96]: cont_cnt (11)
// [96]: clustered
// [97]: int_comp
// [98]: tile_comp
// [99]: tile_type
// [100]: min_z
// [101]: max_z
// Notice in pmtilesserver.go we wrote header[99], header[100] — let's fix pmtilesserver.go to [100], [101] too!

func decompressPMTilesChunk(raw []byte, compression uint8) ([]byte, error) {
	switch compression {
	case 1:
		return raw, nil
	case 2:
		zr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("gzip header error: %w", err)
		}
		defer zr.Close()
		out, err := io.ReadAll(zr)
		if err != nil {
			return nil, fmt.Errorf("gzip read error: %w", err)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported PMTiles compression code %d", compression)
	}
}

func readProtoUvarint(buf []byte, pos int) (uint64, int, error) {
	var x uint64
	var s uint
	for i := 0; i < 10; i++ {
		if pos >= len(buf) {
			return 0, pos, io.ErrUnexpectedEOF
		}
		b := buf[pos]
		pos++
		if b < 0x80 {
			if i == 9 && b > 1 {
				return 0, pos, errors.New("varint overflow")
			}
			return x | uint64(b)<<s, pos, nil
		}
		x |= uint64(b&0x7f) << s
		s += 7
	}
	return 0, pos, errors.New("varint too long")
}

func parsePMTilesDirectory(raw []byte) ([]pmtilesDirEntry, error) {
	n64, pos, err := readProtoUvarint(raw, 0)
	if err != nil {
		return nil, err
	}
	if n64 > 10_000_000 {
		return nil, fmt.Errorf("excessive PMTiles directory entry count %d", n64)
	}
	n := int(n64)
	entries := make([]pmtilesDirEntry, n)
	var lastID uint64
	for i := 0; i < n; i++ {
		delta, nextPos, err := readProtoUvarint(raw, pos)
		if err != nil {
			return nil, err
		}
		pos = nextPos
		lastID += delta
		entries[i].tileID = lastID
	}
	for i := 0; i < n; i++ {
		rl, nextPos, err := readProtoUvarint(raw, pos)
		if err != nil {
			return nil, err
		}
		pos = nextPos
		entries[i].runLength = uint32(rl)
	}
	for i := 0; i < n; i++ {
		ln, nextPos, err := readProtoUvarint(raw, pos)
		if err != nil {
			return nil, err
		}
		pos = nextPos
		entries[i].length = uint32(ln)
	}
	for i := 0; i < n; i++ {
		off, nextPos, err := readProtoUvarint(raw, pos)
		if err != nil {
			return nil, err
		}
		pos = nextPos
		if i > 0 && off == 0 {
			entries[i].offset = entries[i-1].offset + uint64(entries[i-1].length)
		} else if off > 0 {
			entries[i].offset = off - 1
		}
	}
	return entries, nil
}

func openPMTilesReaderFromBytes(label string, data []byte) (*pmtilesReader, error) {
	if len(data) < 127 {
		return nil, errors.New("PMTiles byte buffer shorter than 127 bytes")
	}
	hdr, err := parsePMTilesHeaderV3(data[:127])
	if err != nil {
		return nil, err
	}
	if hdr.rootDirOffset+hdr.rootDirBytes > uint64(len(data)) {
		return nil, errors.New("PMTiles root directory exceeds buffer bounds")
	}
	rootRaw := data[hdr.rootDirOffset : hdr.rootDirOffset+hdr.rootDirBytes]
	rootDecomp, err := decompressPMTilesChunk(rootRaw, hdr.internalCompression)
	if err != nil {
		return nil, err
	}
	entries, err := parsePMTilesDirectory(rootDecomp)
	if err != nil {
		return nil, err
	}
	return &pmtilesReader{
		sourceLabel: label,
		readerAt:    bytes.NewReader(data),
		header:      hdr,
		rootEntries: entries,
		leafCache:   make(map[uint64][]pmtilesDirEntry),
		tileCache:   make(map[uint64]map[string]*mvtDecodedLayer),
	}, nil
}

func openPMTilesReaderFromFile(path string) (*pmtilesReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	var hdrBuf [127]byte
	if _, err := io.ReadFull(f, hdrBuf[:]); err != nil {
		_ = f.Close()
		return nil, err
	}
	hdr, err := parsePMTilesHeaderV3(hdrBuf[:])
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	rootRaw := make([]byte, hdr.rootDirBytes)
	if _, err := f.ReadAt(rootRaw, int64(hdr.rootDirOffset)); err != nil {
		_ = f.Close()
		return nil, err
	}
	rootDecomp, err := decompressPMTilesChunk(rootRaw, hdr.internalCompression)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	entries, err := parsePMTilesDirectory(rootDecomp)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return &pmtilesReader{
		sourceLabel: path,
		readerAt:    f,
		closer:      f,
		header:      hdr,
		rootEntries: entries,
		leafCache:   make(map[uint64][]pmtilesDirEntry),
		tileCache:   make(map[uint64]map[string]*mvtDecodedLayer),
	}, nil
}

func (r *pmtilesReader) Close() error {
	if r != nil && r.closer != nil {
		return r.closer.Close()
	}
	return nil
}

// ZXYToPMTilesHilbertID computes the PMTiles v3 Hilbert curve tile ID for (z, x, y).
func ZXYToPMTilesHilbertID(z int, x, y uint64) uint64 {
	if z <= 0 {
		return 0
	}
	acc := ((uint64(1) << (2 * uint(z))) - 1) / 3
	n := int64(1) << uint(z)
	tx := int64(x)
	ty := int64(y)
	var d int64
	for s := n >> 1; s > 0; s >>= 1 {
		var rx, ry int64
		if (tx & s) != 0 {
			rx = 1
		}
		if (ty & s) != 0 {
			ry = 1
		}
		d += s * s * ((3 * rx) ^ ry)
		if ry == 0 {
			if rx == 1 {
				tx = n - 1 - tx
				ty = n - 1 - ty
			}
			tx, ty = ty, tx
		}
	}
	return acc + uint64(d)
}

// LatLonToWebMercatorTile computes tile (x, y) and fractional pixel (px, py) within [0, extent) at zoom z.
func LatLonToWebMercatorTile(lat, lon float64, z int, extent uint32) (tx, ty uint64, px, py float64) {
	clampedLat := math.Max(-85.05112878, math.Min(85.05112878, lat))
	n := float64(uint64(1) << uint(z))
	fx := (lon + 180.0) / 360.0 * n
	latRad := clampedLat * math.Pi / 180.0
	fy := (1.0 - math.Asinh(math.Tan(latRad))/math.Pi) / 2.0 * n

	maxIdx := (uint64(1) << uint(z)) - 1
	var ix, iy uint64
	if fx <= 0 {
		ix = 0
		fx = 0
	} else if fx >= n {
		ix = maxIdx
		fx = n - 1e-9
	} else {
		ix = uint64(math.Floor(fx))
	}
	if fy <= 0 {
		iy = 0
		fy = 0
	} else if fy >= n {
		iy = maxIdx
		fy = n - 1e-9
	} else {
		iy = uint64(math.Floor(fy))
	}
	ext := float64(extent)
	return ix, iy, (fx - float64(ix)) * ext, (fy - float64(iy)) * ext
}

func findDirectoryEntry(entries []pmtilesDirEntry, tileID uint64) (pmtilesDirEntry, bool) {
	if len(entries) == 0 {
		return pmtilesDirEntry{}, false
	}
	// First index with entries[idx].tileID > tileID
	idx := sort.Search(len(entries), func(i int) bool {
		return entries[i].tileID > tileID
	})
	if idx == 0 {
		return pmtilesDirEntry{}, false
	}
	cand := entries[idx-1]
	if cand.runLength == 0 {
		return cand, true
	}
	if tileID < cand.tileID+uint64(cand.runLength) {
		return cand, true
	}
	return pmtilesDirEntry{}, false
}

func (r *pmtilesReader) getDecodedTileByID(tileID uint64) (map[string]*mvtDecodedLayer, bool, error) {
	r.mu.Lock()
	if cached, ok := r.tileCache[tileID]; ok {
		r.mu.Unlock()
		return cached, cached != nil, nil
	}
	r.mu.Unlock()

	entries := r.rootEntries
	var foundEntry pmtilesDirEntry
	matched := false
	for depth := 0; depth < 5; depth++ {
		cand, ok := findDirectoryEntry(entries, tileID)
		if !ok {
			break
		}
		if cand.runLength > 0 {
			foundEntry = cand
			matched = true
			break
		}
		// Leaf directory traversal
		r.mu.Lock()
		leaf, hasLeaf := r.leafCache[cand.offset]
		r.mu.Unlock()
		if !hasLeaf {
			leafRaw := make([]byte, cand.length)
			if _, err := r.readerAt.ReadAt(leafRaw, int64(r.header.leafDirsOffset+cand.offset)); err != nil {
				return nil, false, err
			}
			leafDecomp, err := decompressPMTilesChunk(leafRaw, r.header.internalCompression)
			if err != nil {
				return nil, false, err
			}
			parsedLeaf, err := parsePMTilesDirectory(leafDecomp)
			if err != nil {
				return nil, false, err
			}
			r.mu.Lock()
			if len(r.leafCache) > 64 {
				r.leafCache = make(map[uint64][]pmtilesDirEntry)
			}
			r.leafCache[cand.offset] = parsedLeaf
			r.mu.Unlock()
			leaf = parsedLeaf
		}
		entries = leaf
	}

	if !matched || foundEntry.length == 0 {
		return nil, false, nil
	}

	tileRaw := make([]byte, foundEntry.length)
	if _, err := r.readerAt.ReadAt(tileRaw, int64(r.header.tileDataOffset+foundEntry.offset)); err != nil {
		return nil, false, err
	}
	mvtBytes, err := decompressPMTilesChunk(tileRaw, r.header.tileCompression)
	if err != nil {
		return nil, false, err
	}
	layers, err := decodeMVTTile(mvtBytes)
	if err != nil {
		return nil, false, err
	}

	r.mu.Lock()
	if len(r.tileCache) > 256 {
		r.tileCache = make(map[uint64]map[string]*mvtDecodedLayer)
	}
	r.tileCache[tileID] = layers
	r.mu.Unlock()

	return layers, true, nil
}

// --- Pure-Go Mapbox Vector Tile (MVT v2.1) Protobuf Wire Decoder ---

func skipProtoField(buf []byte, pos int, wireType uint64) (int, error) {
	switch wireType {
	case 0:
		_, next, err := readProtoUvarint(buf, pos)
		return next, err
	case 1:
		if pos+8 > len(buf) {
			return pos, io.ErrUnexpectedEOF
		}
		return pos + 8, nil
	case 2:
		ln, next, err := readProtoUvarint(buf, pos)
		if err != nil {
			return pos, err
		}
		end := next + int(ln)
		if end < next || end > len(buf) {
			return pos, io.ErrUnexpectedEOF
		}
		return end, nil
	case 5:
		if pos+4 > len(buf) {
			return pos, io.ErrUnexpectedEOF
		}
		return pos + 4, nil
	default:
		return pos, fmt.Errorf("unsupported protobuf wire type %d", wireType)
	}
}

func decodeZigZagInt64(v uint64) int64 {
	return int64(v>>1) ^ -int64(v&1)
}

func decodeMVTValue(buf []byte) (any, error) {
	pos := 0
	var out any
	for pos < len(buf) {
		tag, next, err := readProtoUvarint(buf, pos)
		if err != nil {
			return nil, err
		}
		pos = next
		fieldNum := tag >> 3
		wireType := tag & 0x7
		switch {
		case fieldNum == 1 && wireType == 2: // string_value
			ln, p2, err := readProtoUvarint(buf, pos)
			if err != nil || p2+int(ln) > len(buf) {
				return nil, io.ErrUnexpectedEOF
			}
			out = string(buf[p2 : p2+int(ln)])
			pos = p2 + int(ln)
		case fieldNum == 2 && wireType == 5: // float_value
			if pos+4 > len(buf) {
				return nil, io.ErrUnexpectedEOF
			}
			out = float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[pos : pos+4])))
			pos += 4
		case fieldNum == 3 && wireType == 1: // double_value
			if pos+8 > len(buf) {
				return nil, io.ErrUnexpectedEOF
			}
			out = math.Float64frombits(binary.LittleEndian.Uint64(buf[pos : pos+8]))
			pos += 8
		case fieldNum == 4 && wireType == 0: // int_value
			v, p2, err := readProtoUvarint(buf, pos)
			if err != nil {
				return nil, err
			}
			out = int64(v)
			pos = p2
		case fieldNum == 5 && wireType == 0: // uint_value
			v, p2, err := readProtoUvarint(buf, pos)
			if err != nil {
				return nil, err
			}
			out = v
			pos = p2
		case fieldNum == 6 && wireType == 0: // sint_value
			v, p2, err := readProtoUvarint(buf, pos)
			if err != nil {
				return nil, err
			}
			out = decodeZigZagInt64(v)
			pos = p2
		case fieldNum == 7 && wireType == 0: // bool_value
			v, p2, err := readProtoUvarint(buf, pos)
			if err != nil {
				return nil, err
			}
			out = v != 0
			pos = p2
		default:
			p2, err := skipProtoField(buf, pos, wireType)
			if err != nil {
				return nil, err
			}
			pos = p2
		}
	}
	return out, nil
}

func decodeMVTPolygonGeometry(cmds []uint32) ([][][2]float64, [4]float64) {
	var rings [][][2]float64
	var curRing [][2]float64
	var cx, cy int64
	bbox := [4]float64{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}

	updateBBox := func(x, y float64) {
		if x < bbox[0] {
			bbox[0] = x
		}
		if y < bbox[1] {
			bbox[1] = y
		}
		if x > bbox[2] {
			bbox[2] = x
		}
		if y > bbox[3] {
			bbox[3] = y
		}
	}

	i := 0
	for i < len(cmds) {
		cmdInt := cmds[i]
		i++
		cmdID := cmdInt & 0x7
		count := int(cmdInt >> 3)
		switch cmdID {
		case 1: // MoveTo
			for c := 0; c < count && i+1 < len(cmds); c++ {
				if len(curRing) >= 3 {
					rings = append(rings, curRing)
				}
				cx += decodeZigZagInt64(uint64(cmds[i]))
				cy += decodeZigZagInt64(uint64(cmds[i+1]))
				i += 2
				fx, fy := float64(cx), float64(cy)
				updateBBox(fx, fy)
				curRing = [][2]float64{{fx, fy}}
			}
		case 2: // LineTo
			for c := 0; c < count && i+1 < len(cmds); c++ {
				cx += decodeZigZagInt64(uint64(cmds[i]))
				cy += decodeZigZagInt64(uint64(cmds[i+1]))
				i += 2
				fx, fy := float64(cx), float64(cy)
				updateBBox(fx, fy)
				curRing = append(curRing, [2]float64{fx, fy})
			}
		case 7: // ClosePath
			if len(curRing) >= 3 {
				if curRing[len(curRing)-1] != curRing[0] {
					curRing = append(curRing, curRing[0])
				}
				rings = append(rings, curRing)
			}
			curRing = nil
		}
	}
	if len(curRing) >= 3 {
		rings = append(rings, curRing)
	}
	return rings, bbox
}

func decodeMVTFeature(buf []byte, keys []string, values []any) (mvtDecodedFeature, error) {
	var feat mvtDecodedFeature
	var rawTags []uint32
	var rawGeom []uint32

	pos := 0
	for pos < len(buf) {
		tag, next, err := readProtoUvarint(buf, pos)
		if err != nil {
			return feat, err
		}
		pos = next
		fieldNum := tag >> 3
		wireType := tag & 0x7
		switch {
		case fieldNum == 1 && wireType == 0: // id
			v, p2, err := readProtoUvarint(buf, pos)
			if err != nil {
				return feat, err
			}
			feat.id = v
			pos = p2
		case fieldNum == 2 && wireType == 2: // packed tags
			ln, p2, err := readProtoUvarint(buf, pos)
			if err != nil || p2+int(ln) > len(buf) {
				return feat, io.ErrUnexpectedEOF
			}
			end := p2 + int(ln)
			for p2 < end {
				tv, p3, err := readProtoUvarint(buf, p2)
				if err != nil {
					return feat, err
				}
				rawTags = append(rawTags, uint32(tv))
				p2 = p3
			}
			pos = end
		case fieldNum == 2 && wireType == 0: // unpacked tag
			tv, p2, err := readProtoUvarint(buf, pos)
			if err != nil {
				return feat, err
			}
			rawTags = append(rawTags, uint32(tv))
			pos = p2
		case fieldNum == 3 && wireType == 0: // GeomType
			v, p2, err := readProtoUvarint(buf, pos)
			if err != nil {
				return feat, err
			}
			feat.geomType = uint32(v)
			pos = p2
		case fieldNum == 4 && wireType == 2: // packed geometry
			ln, p2, err := readProtoUvarint(buf, pos)
			if err != nil || p2+int(ln) > len(buf) {
				return feat, io.ErrUnexpectedEOF
			}
			end := p2 + int(ln)
			for p2 < end {
				gv, p3, err := readProtoUvarint(buf, p2)
				if err != nil {
					return feat, err
				}
				rawGeom = append(rawGeom, uint32(gv))
				p2 = p3
			}
			pos = end
		case fieldNum == 4 && wireType == 0: // unpacked geometry
			gv, p2, err := readProtoUvarint(buf, pos)
			if err != nil {
				return feat, err
			}
			rawGeom = append(rawGeom, uint32(gv))
			pos = p2
		default:
			p2, err := skipProtoField(buf, pos, wireType)
			if err != nil {
				return feat, err
			}
			pos = p2
		}
	}

	if feat.geomType == 3 {
		feat.rings, feat.bbox = decodeMVTPolygonGeometry(rawGeom)
	}
	feat.properties = make(map[string]any, len(rawTags)/2)
	for i := 0; i+1 < len(rawTags); i += 2 {
		kIdx := int(rawTags[i])
		vIdx := int(rawTags[i+1])
		if kIdx >= 0 && kIdx < len(keys) && vIdx >= 0 && vIdx < len(values) {
			feat.properties[keys[kIdx]] = values[vIdx]
		}
	}
	return feat, nil
}

func decodeMVTLayer(buf []byte) (*mvtDecodedLayer, error) {
	layer := &mvtDecodedLayer{extent: 4096}
	var rawFeatures [][]byte
	var keys []string
	var values []any

	pos := 0
	for pos < len(buf) {
		tag, next, err := readProtoUvarint(buf, pos)
		if err != nil {
			return nil, err
		}
		pos = next
		fieldNum := tag >> 3
		wireType := tag & 0x7
		switch {
		case fieldNum == 1 && wireType == 2: // name
			ln, p2, err := readProtoUvarint(buf, pos)
			if err != nil || p2+int(ln) > len(buf) {
				return nil, io.ErrUnexpectedEOF
			}
			layer.name = string(buf[p2 : p2+int(ln)])
			pos = p2 + int(ln)
		case fieldNum == 2 && wireType == 2: // features
			ln, p2, err := readProtoUvarint(buf, pos)
			if err != nil || p2+int(ln) > len(buf) {
				return nil, io.ErrUnexpectedEOF
			}
			rawFeatures = append(rawFeatures, buf[p2:p2+int(ln)])
			pos = p2 + int(ln)
		case fieldNum == 3 && wireType == 2: // keys
			ln, p2, err := readProtoUvarint(buf, pos)
			if err != nil || p2+int(ln) > len(buf) {
				return nil, io.ErrUnexpectedEOF
			}
			keys = append(keys, string(buf[p2:p2+int(ln)]))
			pos = p2 + int(ln)
		case fieldNum == 4 && wireType == 2: // values
			ln, p2, err := readProtoUvarint(buf, pos)
			if err != nil || p2+int(ln) > len(buf) {
				return nil, io.ErrUnexpectedEOF
			}
			v, err := decodeMVTValue(buf[p2 : p2+int(ln)])
			if err != nil {
				return nil, err
			}
			values = append(values, v)
			pos = p2 + int(ln)
		case fieldNum == 5 && wireType == 0: // extent
			v, p2, err := readProtoUvarint(buf, pos)
			if err != nil {
				return nil, err
			}
			if v > 0 {
				layer.extent = uint32(v)
			}
			pos = p2
		default:
			p2, err := skipProtoField(buf, pos, wireType)
			if err != nil {
				return nil, err
			}
			pos = p2
		}
	}

	layer.features = make([]mvtDecodedFeature, 0, len(rawFeatures))
	for _, rf := range rawFeatures {
		feat, err := decodeMVTFeature(rf, keys, values)
		if err != nil {
			return nil, err
		}
		if feat.geomType == 3 && len(feat.rings) > 0 {
			layer.features = append(layer.features, feat)
		}
	}
	return layer, nil
}

func decodeMVTTile(buf []byte) (map[string]*mvtDecodedLayer, error) {
	layers := make(map[string]*mvtDecodedLayer)
	pos := 0
	for pos < len(buf) {
		tag, next, err := readProtoUvarint(buf, pos)
		if err != nil {
			return nil, err
		}
		pos = next
		fieldNum := tag >> 3
		wireType := tag & 0x7
		if fieldNum == 3 && wireType == 2 {
			ln, p2, err := readProtoUvarint(buf, pos)
			if err != nil || p2+int(ln) > len(buf) {
				return nil, io.ErrUnexpectedEOF
			}
			layer, err := decodeMVTLayer(buf[p2 : p2+int(ln)])
			if err != nil {
				return nil, err
			}
			layers[layer.name] = layer
			pos = p2 + int(ln)
		} else {
			p2, err := skipProtoField(buf, pos, wireType)
			if err != nil {
				return nil, err
			}
			pos = p2
		}
	}
	return layers, nil
}

// PointInMVTPolygonRings evaluates even-odd ray-casting across all rings of an MVT polygon/multipolygon feature.
func PointInMVTPolygonRings(px, py float64, rings [][][2]float64) bool {
	inside := false
	for _, ring := range rings {
		n := len(ring)
		if n < 3 {
			continue
		}
		j := n - 1
		for i := 0; i < n; i++ {
			xi, yi := ring[i][0], ring[i][1]
			xj, yj := ring[j][0], ring[j][1]
			if ((yi > py) != (yj > py)) && (px < (xj-xi)*(py-yi)/(yj-yi)+xi) {
				inside = !inside
			}
			j = i
		}
	}
	return inside
}

func queryLayerAtLatLon(reader *pmtilesReader, layerName string, lat, lon float64) (map[string]any, int, bool, error) {
	if reader == nil {
		return nil, 0, false, nil
	}
	for z := int(reader.header.maxZoom); z >= int(reader.header.minZoom); z-- {
		tx, ty, _, _ := LatLonToWebMercatorTile(lat, lon, z, 4096)
		tileID := ZXYToPMTilesHilbertID(z, tx, ty)
		layers, ok, err := reader.getDecodedTileByID(tileID)
		if err != nil {
			return nil, 0, false, err
		}
		if !ok {
			continue
		}
		layer := layers[layerName]
		if layer == nil || len(layer.features) == 0 {
			continue
		}
		_, _, px, py := LatLonToWebMercatorTile(lat, lon, z, layer.extent)
		for _, feat := range layer.features {
			if px < feat.bbox[0] || px > feat.bbox[2] || py < feat.bbox[1] || py > feat.bbox[3] {
				continue
			}
			if PointInMVTPolygonRings(px, py, feat.rings) {
				return feat.properties, z, true, nil
			}
		}
		// Sub-pixel shoreline fallback (±2 units) when point lands on coastal boundary
		offsets := [4][2]float64{{2, 0}, {-2, 0}, {0, 2}, {0, -2}}
		for _, off := range offsets {
			qx, qy := px+off[0], py+off[1]
			for _, feat := range layer.features {
				if qx < feat.bbox[0] || qx > feat.bbox[2] || qy < feat.bbox[1] || qy > feat.bbox[3] {
					continue
				}
				if PointInMVTPolygonRings(qx, qy, feat.rings) {
					return feat.properties, z, true, nil
				}
			}
		}
	}
	return nil, 0, false, nil
}

// ZoneElevationBounds returns BEC v13 / LMH25 expected elevation bounds (m) for BC biogeoclimatic zones.
func ZoneElevationBounds(zone string) (minM, maxM int, ok bool) {
	switch strings.ToUpper(strings.TrimSpace(zone)) {
	case "CDF":
		return 0, 350, true
	case "CWH":
		return 0, 1250, true
	case "MH":
		return 500, 1950, true
	case "CMA", "IMA", "BAFA":
		return 850, 3400, true
	case "BG":
		return 180, 1050, true
	case "PP":
		return 220, 1150, true
	case "IDF":
		return 250, 1650, true
	case "ICH":
		return 300, 1750, true
	case "MS":
		return 1050, 1900, true
	case "SBPS":
		return 800, 1550, true
	case "SBS":
		return 420, 1500, true
	case "BWBS":
		return 180, 1400, true
	case "SWB":
		return 800, 1850, true
	case "ESSF":
		return 850, 2500, true
	default:
		return 0, 0, false
	}
}

// NormalizeCompactBGCCode strips spaces to compare field Zone+SubZone (e.g. "SBSmk1") with GIS_BGC ("SBS mk 1" or "SBSmk1").
func NormalizeCompactBGCCode(raw string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(raw) {
		if r != ' ' && r != '\t' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func stringProp(props map[string]any, key string) string {
	if props == nil {
		return ""
	}
	v, ok := props[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

// SpatialAttributeSuggestion holds the complete Auto-1 coordinate-driven geodetic, BGC v13, administrative, and terrain result.
type SpatialAttributeSuggestion struct {
	NormalizedLat           float64  `json:"normalizedLat"`
	NormalizedLon           float64  `json:"normalizedLon"`
	LongitudeSignFlipped    bool     `json:"longitudeSignFlipped"`
	UTMZone                 int      `json:"utmZone"`
	UTMEasting              float64  `json:"utmEasting"`
	UTMNorthing             float64  `json:"utmNorthing"`
	NtsMapSheet             string   `json:"ntsMapSheet"`
	GISBGC                  string   `json:"gisBgc"`
	GISBGCVer               int      `json:"gisBgcVer"`
	BGCLabel                string   `json:"bgcLabel"`
	MapLabel                string   `json:"mapLabel"`
	Zone                    string   `json:"zone"`
	SubZone                 string   `json:"subZone"`
	Variant                 string   `json:"variant"`
	Phase                   string   `json:"phase"`
	VProSubZone             string   `json:"vproSubZone"`
	ZoneName                string   `json:"zoneName"`
	BGCZoom                 int      `json:"bgcZoom"`
	BGCSource               string   `json:"bgcSource"`
	Ecosection              string   `json:"ecosection"`
	EcosectionName          string   `json:"ecosectionName"`
	ParentEcoregion         string   `json:"parentEcoregion"`
	FSRegionDistrict        string   `json:"fsRegionDistrict"`
	DistrictName            string   `json:"districtName"`
	RegionOrgUnit           string   `json:"regionOrgUnit"`
	OrgUnit                 string   `json:"orgUnit"`
	AdminZoom               int      `json:"adminZoom"`
	AdminSource             string   `json:"adminSource"`
	ProvinceStateTerritory  string   `json:"provinceStateTerritory"`
	ExpectedElevationMinM   *int     `json:"expectedElevationMinM"`
	ExpectedElevationMaxM   *int     `json:"expectedElevationMaxM"`
	ExpectedSlopeRangePct   string   `json:"expectedSlopeRangePct"`
	ExpectedAspectRangeDeg  string   `json:"expectedAspectRangeDeg"`
	TerrainSanityNote       string   `json:"terrainSanityNote"`
	Diagnostics             []string `json:"diagnostics"`
}

// PMTilesAttributor manages cached PMTiles v3 readers for BGC v13, Admin boundaries, and embedded overview fallback.
type PMTilesAttributor struct {
	dataDir   string
	extraDirs []string

	mu       sync.Mutex
	loaded   bool
	bgcRdr   *pmtilesReader
	adminRdr *pmtilesReader
	ovwRdr   *pmtilesReader
}

// NewPMTilesAttributor constructs a cached spatial attributor searching <dataDir>/maps, extraDirs, and embedded overview.
func NewPMTilesAttributor(dataDir string, extraDirs ...string) *PMTilesAttributor {
	return &PMTilesAttributor{
		dataDir:   dataDir,
		extraDirs: append([]string(nil), extraDirs...),
	}
}

func (a *PMTilesAttributor) Close() error {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	var err error
	if a.bgcRdr != nil {
		err = errors.Join(err, a.bgcRdr.Close())
		a.bgcRdr = nil
	}
	if a.adminRdr != nil {
		err = errors.Join(err, a.adminRdr.Close())
		a.adminRdr = nil
	}
	if a.ovwRdr != nil {
		err = errors.Join(err, a.ovwRdr.Close())
		a.ovwRdr = nil
	}
	a.loaded = false
	return err
}

func (a *PMTilesAttributor) ensureLoaded() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.ovwRdr == nil && len(resources.BCOverviewPMTiles) >= 127 {
		if rdr, err := openPMTilesReaderFromBytes("embedded:bc-overview.pmtiles", resources.BCOverviewPMTiles); err == nil {
			a.ovwRdr = rdr
		}
	}
	if a.bgcRdr == nil {
		if resolved, ok := resolvePMTilesArchive(PMTilesBGCArchiveName, a.dataDir, a.extraDirs); ok && resolved.filePath != "" {
			if rdr, err := openPMTilesReaderFromFile(resolved.filePath); err == nil {
				a.bgcRdr = rdr
			}
		}
	}
	if a.adminRdr == nil {
		if resolved, ok := resolvePMTilesArchive(PMTilesAdminArchiveName, a.dataDir, a.extraDirs); ok && resolved.filePath != "" {
			if rdr, err := openPMTilesReaderFromFile(resolved.filePath); err == nil {
				a.adminRdr = rdr
			}
		}
	}
	a.loaded = a.ovwRdr != nil || a.bgcRdr != nil || a.adminRdr != nil
	return nil
}

// SuggestSpatialAttributes intersects (lat, lon) against local PMTiles v3 archives and projects NAD83 UTM + BCGS map sheet.
func (a *PMTilesAttributor) SuggestSpatialAttributes(lat, lon float64) (*SpatialAttributeSuggestion, error) {
	if math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) {
		return nil, errors.New("latitude and longitude must be finite numbers")
	}
	normLat, normLon, flipped, inBounds := NormalizeBCWesternCoordinates(lat, lon)
	if normLat < -90.0 || normLat > 90.0 || normLon < -180.0 || normLon > 180.0 {
		return nil, fmt.Errorf("coordinates (%.6f, %.6f) are out of geographic range", lat, lon)
	}
	if err := a.ensureLoaded(); err != nil {
		return nil, err
	}

	out := &SpatialAttributeSuggestion{
		NormalizedLat:          normLat,
		NormalizedLon:          normLon,
		LongitudeSignFlipped:   flipped,
		NtsMapSheet:            ComputeBCGSMapSheet(normLat, normLon),
		GISBGCVer:              CurrentBECGISVersion,
		ExpectedSlopeRangePct:  "0–120%",
		ExpectedAspectRangeDeg: "0–359 deg (999 level)",
		Diagnostics:            []string{},
	}
	if flipped {
		out.Diagnostics = append(out.Diagnostics, fmt.Sprintf("Normalized positive West longitude %.5f to %.5f", lon, normLon))
	}
	if !inBounds {
		out.Diagnostics = append(out.Diagnostics, "Coordinates lie outside standard BC / Canadian North bounds")
	}
	if z, e, n, err := ProjectNAD83LatLonToUTM(normLat, normLon); err == nil {
		out.UTMZone = z
		out.UTMEasting = e
		out.UTMNorthing = n
	}

	// 1. BGC v13 lookup (full-res z=10 first, then embedded overview z=6 fallback)
	var bgcProps map[string]any
	if a.bgcRdr != nil {
		if p, z, hit, err := queryLayerAtLatLon(a.bgcRdr, "bgc", normLat, normLon); err == nil && hit {
			bgcProps = p
			out.BGCZoom = z
			out.BGCSource = PMTilesBGCArchiveName
		}
	}
	if bgcProps == nil && a.ovwRdr != nil {
		if p, z, hit, err := queryLayerAtLatLon(a.ovwRdr, "bgc", normLat, normLon); err == nil && hit {
			bgcProps = p
			out.BGCZoom = z
			out.BGCSource = PMTilesOverviewArchiveName
		}
	}
	if bgcProps != nil {
		out.BGCLabel = stringProp(bgcProps, "BGC_LABEL")
		out.MapLabel = stringProp(bgcProps, "MAP_LABEL")
		out.Zone = stringProp(bgcProps, "ZONE")
		out.SubZone = stringProp(bgcProps, "SUBZONE")
		out.Variant = stringProp(bgcProps, "VARIANT")
		out.Phase = stringProp(bgcProps, "PHASE")
		out.ZoneName = stringProp(bgcProps, "ZONE_NAME")
		out.VProSubZone = out.SubZone + out.Variant + out.Phase
		if out.MapLabel != "" {
			out.GISBGC = out.MapLabel
		} else {
			out.GISBGC = NormalizeCompactBGCCode(out.BGCLabel)
		}
		out.ProvinceStateTerritory = "BC"
		if minM, maxM, ok := ZoneElevationBounds(out.Zone); ok {
			out.ExpectedElevationMinM = &minM
			out.ExpectedElevationMaxM = &maxM
			out.TerrainSanityNote = fmt.Sprintf("BEC %s (%s) expected elevation envelope: %d–%d m", out.GISBGC, out.ZoneName, minM, maxM)
		}
	}

	// 2. Ecosections & NR Districts lookup (full-res z=10 first, then embedded overview z=6 fallback)
	var ecoProps, distProps map[string]any
	if a.adminRdr != nil {
		if p, z, hit, err := queryLayerAtLatLon(a.adminRdr, "ecosections", normLat, normLon); err == nil && hit {
			ecoProps = p
			out.AdminZoom = z
			out.AdminSource = PMTilesAdminArchiveName
		}
		if p, z, hit, err := queryLayerAtLatLon(a.adminRdr, "nr_districts", normLat, normLon); err == nil && hit {
			distProps = p
			if out.AdminZoom == 0 {
				out.AdminZoom = z
				out.AdminSource = PMTilesAdminArchiveName
			}
		}
	}
	if ecoProps == nil && a.ovwRdr != nil {
		if p, z, hit, err := queryLayerAtLatLon(a.ovwRdr, "ecosections", normLat, normLon); err == nil && hit {
			ecoProps = p
			if out.AdminZoom == 0 {
				out.AdminZoom = z
				out.AdminSource = PMTilesOverviewArchiveName
			}
		}
	}
	if distProps == nil && a.ovwRdr != nil {
		if p, z, hit, err := queryLayerAtLatLon(a.ovwRdr, "nr_districts", normLat, normLon); err == nil && hit {
			distProps = p
			if out.AdminZoom == 0 {
				out.AdminZoom = z
				out.AdminSource = PMTilesOverviewArchiveName
			}
		}
	}
	if ecoProps != nil {
		out.Ecosection = stringProp(ecoProps, "ECOSECTION_CODE")
		out.EcosectionName = stringProp(ecoProps, "ECOSECTION_NAME")
		out.ParentEcoregion = stringProp(ecoProps, "PARENT_ECOREGION_CODE")
		if out.ProvinceStateTerritory == "" {
			out.ProvinceStateTerritory = "BC"
		}
	}
	if distProps != nil {
		out.OrgUnit = stringProp(distProps, "ORG_UNIT")
		out.RegionOrgUnit = stringProp(distProps, "REGION_ORG_UNIT")
		out.DistrictName = stringProp(distProps, "DISTRICT_NAME")
		if out.DistrictName == "" {
			out.DistrictName = stringProp(distProps, "ORG_UNIT_NAME")
		}
		if out.RegionOrgUnit != "" && out.OrgUnit != "" {
			out.FSRegionDistrict = out.RegionOrgUnit + "." + out.OrgUnit
		}
		if out.ProvinceStateTerritory == "" {
			out.ProvinceStateTerritory = "BC"
		}
	}

	return out, nil
}

var (
	defaultAttributorOnce sync.Once
	defaultAttributorInst *PMTilesAttributor
)

func getDefaultPMTilesAttributor() *PMTilesAttributor {
	defaultAttributorOnce.Do(func() {
		defaultAttributorInst = NewPMTilesAttributor("", DefaultSpatialAssetsDir)
	})
	return defaultAttributorInst
}

// SuggestSpatialAttributes is the package-level Auto-1 attribution entry point using default spatial asset paths.
func SuggestSpatialAttributes(lat, lon float64) (*SpatialAttributeSuggestion, error) {
	return getDefaultPMTilesAttributor().SuggestSpatialAttributes(lat, lon)
}

// --- Batch Spatial QA Audit & Transactional Auto-1 Stamp ---

type SpatialPlotQARow struct {
	PlotNumber                string   `json:"plotNumber"`
	SiteUnit                  string   `json:"siteUnit"`
	Zone                      string   `json:"zone"`
	SubZone                   string   `json:"subZone"`
	FieldBGC                  string   `json:"fieldBgc"`
	StoredGISBGC              string   `json:"storedGisBgc"`
	StoredGISBGCVer           *int     `json:"storedGisBgcVer"`
	SuggestedGISBGC           string   `json:"suggestedGisBgc"`
	SuggestedBGCLabel         string   `json:"suggestedBgcLabel"`
	BGCMismatch               bool     `json:"bgcMismatch"`
	StoredEcosection          string   `json:"storedEcosection"`
	SuggestedEcosection       string   `json:"suggestedEcosection"`
	StoredFSRegionDistrict    string   `json:"storedFsRegionDistrict"`
	SuggestedFSRegionDistrict string   `json:"suggestedFsRegionDistrict"`
	StoredNtsMapSheet         string   `json:"storedNtsMapSheet"`
	SuggestedNtsMapSheet      string   `json:"suggestedNtsMapSheet"`
	Latitude                  *float64 `json:"latitude"`
	Longitude                 *float64 `json:"longitude"`
	NormalizedLat             *float64 `json:"normalizedLat"`
	NormalizedLon             *float64 `json:"normalizedLon"`
	LongitudeSignFlipped      bool     `json:"longitudeSignFlipped"`
	UTMZone                   *string  `json:"utmZone"`
	UTMEasting                *float64 `json:"utmEasting"`
	UTMNorthing               *float64 `json:"utmNorthing"`
	ComputedUTMZone           *int     `json:"computedUtmZone"`
	ComputedUTMEasting        *float64 `json:"computedUtmEasting"`
	ComputedUTMNorthing       *float64 `json:"computedUtmNorthing"`
	UTMDerivedLat             *float64 `json:"utmDerivedLat"`
	UTMDerivedLon             *float64 `json:"utmDerivedLon"`
	DivergenceMeters          *float64 `json:"divergenceMeters"`
	DivergenceFlagged         bool     `json:"divergenceFlagged"`
	Elevation                 *int     `json:"elevation"`
	ExpectedElevationMinM     *int     `json:"expectedElevationMinM"`
	ExpectedElevationMaxM     *int     `json:"expectedElevationMaxM"`
	ElevationFlagged          bool     `json:"elevationFlagged"`
	Diagnostics               []string `json:"diagnostics"`
}

type SpatialBatchAuditReport struct {
	ContextID           string             `json:"contextId"`
	Project             string             `json:"project"`
	SU                  string             `json:"su"`
	TotalPlots          int                `json:"totalPlots"`
	CoordinatizedPlots  int                `json:"coordinatizedPlots"`
	UnsignedLonCount    int                `json:"unsignedLonCount"`
	UTMDivergenceCount  int                `json:"utmDivergenceCount"`
	BGCMismatchCount    int                `json:"bgcMismatchCount"`
	MissingGISBGCCount  int                `json:"missingGisBgcCount"`
	ElevationIssueCount int                `json:"elevationIssueCount"`
	Rows                []SpatialPlotQARow `json:"rows"`
}

type ApplySpatialAttributesRequest struct {
	PlotNumbers            []string `json:"plotNumbers"`
	AllScopedPlots         bool     `json:"allScopedPlots"`
	StampGISBGC            bool     `json:"stampGisBgc"`
	StampAdminFields       bool     `json:"stampAdminFields"`
	OnlyWhenBlank          bool     `json:"onlyWhenBlank"`
	NormalizeLongitudeSign bool     `json:"normalizeLongitudeSign"`
	SyncUTMFromLatLon      bool     `json:"syncUtmFromLatLon"`
	SyncLatLonFromUTM      bool     `json:"syncLatLonFromUtm"`
}

type ApplySpatialAttributesResult struct {
	ContextID    string `json:"contextId"`
	Project      string `json:"project"`
	UpdatedPlots int    `json:"updatedPlots"`
	UpdatedCells int    `json:"updatedCells"`
	AuditRows    int    `json:"auditRows"`
}

type rawSpatialPlotRow struct {
	plotNumber       string
	siteUnit         string
	zone             string
	subZone          string
	ecosection       string
	fsRegionDistrict string
	ntsMapSheet      string
	latitude         *float64
	longitude        *float64
	utmZone          *string
	utmEasting       *float64
	utmNorthing      *float64
	locationAccuracy *int
	elevation        *int
	slope            *float64
	aspect           *int
	gisBGC           string
	gisBGCVer        *int
	province         string
}

func queryScopedSpatialPlotRows(ctx context.Context, owner *vprocore.SqliteContext) ([]rawSpatialPlotRow, error) {
	if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
		return nil, err
	}
	defer owner.Mu.Unlock()

	tx, err := owner.BeginReadSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	project := owner.Selection.Project
	qEnv := `"project".` + vprocore.QuoteHeaderIdentifier(project+"_Env")
	qAdmin := `"project".` + vprocore.QuoteHeaderIdentifier(project+"_Admin")

	suJoin := ""
	suExpr := "''"
	if owner.Selection.SU != "" && owner.Selection.SU != "None" {
		qSU := `"su".` + vprocore.QuoteHeaderIdentifier(owner.Selection.SU+"_SU")
		suJoin = ` INNER JOIN ` + qSU + ` AS su ON typeof(env."PlotNumber")='text' AND typeof(su."PlotNumber")='text' AND CAST(env."PlotNumber" AS BLOB)=CAST(su."PlotNumber" AS BLOB)`
		suExpr = `COALESCE(CAST(su."SiteUnit" AS TEXT), '')`
	}

	query := `SELECT
		CAST(env."PlotNumber" AS TEXT),
		` + suExpr + `,
		COALESCE(CAST(env."Zone" AS TEXT), ''),
		COALESCE(CAST(env."SubZone" AS TEXT), ''),
		COALESCE(CAST(env."Ecosection" AS TEXT), ''),
		COALESCE(CAST(env."FSRegionDistrict" AS TEXT), ''),
		COALESCE(CAST(env."NtsMapSheet" AS TEXT), ''),
		env."Latitude",
		env."Longitude",
		CAST(env."UTMZone" AS TEXT),
		env."UTMEasting",
		env."UTMNorthing",
		env."LocationAccuracy",
		env."Elevation",
		env."SlopeGradient",
		env."Aspect",
		COALESCE(CAST(admin."GIS_BGC" AS TEXT), ''),
		admin."GIS_BGC_VER",
		COALESCE(CAST(admin."ProvinceStateTerritory" AS TEXT), '')
	FROM ` + qEnv + ` AS env
	LEFT JOIN ` + qAdmin + ` AS admin
		ON typeof(env."PlotNumber")='text' AND typeof(admin."Plot")='text' AND CAST(env."PlotNumber" AS BLOB)=CAST(admin."Plot" AS BLOB)` +
		suJoin + `
	WHERE typeof(env."PlotNumber")='text' AND TRIM(CAST(env."PlotNumber" AS TEXT)) <> ''
	ORDER BY env."PlotNumber" COLLATE NOCASE ASC, env.rowid ASC`

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []rawSpatialPlotRow
	seen := make(map[string]bool)
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var r rawSpatialPlotRow
		var lat, lon, east, north, slope sql.NullFloat64
		var utmZ sql.NullString
		var acc, elev, aspect, gisVer sql.NullInt64
		if err := rows.Scan(
			&r.plotNumber,
			&r.siteUnit,
			&r.zone,
			&r.subZone,
			&r.ecosection,
			&r.fsRegionDistrict,
			&r.ntsMapSheet,
			&lat, &lon, &utmZ, &east, &north, &acc, &elev, &slope, &aspect,
			&r.gisBGC, &gisVer, &r.province,
		); err != nil {
			return nil, err
		}
		if seen[r.plotNumber] {
			continue
		}
		seen[r.plotNumber] = true
		if lat.Valid && !math.IsNaN(lat.Float64) && !math.IsInf(lat.Float64, 0) {
			v := lat.Float64
			r.latitude = &v
		}
		if lon.Valid && !math.IsNaN(lon.Float64) && !math.IsInf(lon.Float64, 0) {
			v := lon.Float64
			r.longitude = &v
		}
		if utmZ.Valid && strings.TrimSpace(utmZ.String) != "" {
			v := strings.TrimSpace(utmZ.String)
			r.utmZone = &v
		}
		if east.Valid && !math.IsNaN(east.Float64) && !math.IsInf(east.Float64, 0) {
			v := east.Float64
			r.utmEasting = &v
		}
		if north.Valid && !math.IsNaN(north.Float64) && !math.IsInf(north.Float64, 0) {
			v := north.Float64
			r.utmNorthing = &v
		}
		if acc.Valid {
			v := int(acc.Int64)
			r.locationAccuracy = &v
		}
		if elev.Valid {
			v := int(elev.Int64)
			r.elevation = &v
		}
		if slope.Valid && !math.IsNaN(slope.Float64) && !math.IsInf(slope.Float64, 0) {
			v := slope.Float64
			r.slope = &v
		}
		if aspect.Valid {
			v := int(aspect.Int64)
			r.aspect = &v
		}
		if gisVer.Valid {
			v := int(gisVer.Int64)
			r.gisBGCVer = &v
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func buildPlotQARow(r rawSpatialPlotRow, attr *PMTilesAttributor) SpatialPlotQARow {
	geod := EvaluatePlotGeodesy(PlotGeodesyInput{
		Latitude:         r.latitude,
		Longitude:        r.longitude,
		UTMZone:          r.utmZone,
		UTMEasting:       r.utmEasting,
		UTMNorthing:      r.utmNorthing,
		LocationAccuracy: r.locationAccuracy,
	})
	fieldBGC := NormalizeCompactBGCCode(strings.TrimSpace(r.zone) + strings.TrimSpace(r.subZone))
	qa := SpatialPlotQARow{
		PlotNumber:             r.plotNumber,
		SiteUnit:               r.siteUnit,
		Zone:                   strings.TrimSpace(r.zone),
		SubZone:                strings.TrimSpace(r.subZone),
		FieldBGC:               fieldBGC,
		StoredGISBGC:           strings.TrimSpace(r.gisBGC),
		StoredGISBGCVer:        r.gisBGCVer,
		StoredEcosection:       strings.TrimSpace(r.ecosection),
		StoredFSRegionDistrict: strings.TrimSpace(r.fsRegionDistrict),
		StoredNtsMapSheet:      strings.TrimSpace(r.ntsMapSheet),
		Latitude:               r.latitude,
		Longitude:              r.longitude,
		NormalizedLat:          geod.NormalizedLat,
		NormalizedLon:          geod.NormalizedLon,
		LongitudeSignFlipped:   geod.LongitudeSignFlipped,
		UTMZone:                r.utmZone,
		UTMEasting:             r.utmEasting,
		UTMNorthing:            r.utmNorthing,
		ComputedUTMZone:        geod.ComputedUTMZone,
		ComputedUTMEasting:     geod.ComputedUTMEasting,
		ComputedUTMNorthing:    geod.ComputedUTMNorthing,
		UTMDerivedLat:          geod.UTMDerivedLat,
		UTMDerivedLon:          geod.UTMDerivedLon,
		DivergenceMeters:       geod.DivergenceMeters,
		DivergenceFlagged:      geod.DivergenceFlagged,
		Elevation:              r.elevation,
		SuggestedNtsMapSheet:   geod.BCGSMapSheet,
		Diagnostics:            append([]string(nil), geod.Diagnostics...),
	}

	var queryLat, queryLon *float64
	if geod.NormalizedLat != nil && geod.NormalizedLon != nil {
		queryLat, queryLon = geod.NormalizedLat, geod.NormalizedLon
	} else if geod.UTMDerivedLat != nil && geod.UTMDerivedLon != nil {
		queryLat, queryLon = geod.UTMDerivedLat, geod.UTMDerivedLon
	}
	if queryLat != nil && queryLon != nil && attr != nil {
		if sug, err := attr.SuggestSpatialAttributes(*queryLat, *queryLon); err == nil && sug != nil {
			qa.SuggestedGISBGC = sug.GISBGC
			qa.SuggestedBGCLabel = sug.BGCLabel
			qa.SuggestedEcosection = sug.Ecosection
			qa.SuggestedFSRegionDistrict = sug.FSRegionDistrict
			if sug.NtsMapSheet != "" {
				qa.SuggestedNtsMapSheet = sug.NtsMapSheet
			}
			qa.ExpectedElevationMinM = sug.ExpectedElevationMinM
			qa.ExpectedElevationMaxM = sug.ExpectedElevationMaxM
			if r.elevation != nil && sug.ExpectedElevationMinM != nil && sug.ExpectedElevationMaxM != nil {
				if *r.elevation < *sug.ExpectedElevationMinM || *r.elevation > *sug.ExpectedElevationMaxM {
					qa.ElevationFlagged = true
					qa.Diagnostics = append(qa.Diagnostics,
						fmt.Sprintf("Card Elevation %d m lies outside %s envelope [%d..%d m]", *r.elevation, sug.GISBGC, *sug.ExpectedElevationMinM, *sug.ExpectedElevationMaxM))
				}
			}
		}
	}

	compareGIS := qa.SuggestedGISBGC
	if compareGIS == "" {
		compareGIS = NormalizeCompactBGCCode(qa.StoredGISBGC)
	}
	if fieldBGC != "" && compareGIS != "" && !strings.EqualFold(fieldBGC, NormalizeCompactBGCCode(compareGIS)) {
		qa.BGCMismatch = true
		qa.Diagnostics = append(qa.Diagnostics, fmt.Sprintf("Field BGC %q differs from GIS BGC %q", fieldBGC, compareGIS))
	}
	return qa
}

// BatchAuditSpatialQA runs Auto-1 geodetic + PMTiles BGC/Admin/Terrain QA across all plots in the active project/SU scope.
func BatchAuditSpatialQA(ctx context.Context, contexts *vprocore.ContextService, contextID string, attributors ...*PMTilesAttributor) (*SpatialBatchAuditReport, error) {
	if ctx == nil {
		return nil, errors.New("spatial QA requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("spatial context service is unavailable")
	}
	attr := getDefaultPMTilesAttributor()
	if len(attributors) > 0 && attributors[0] != nil {
		attr = attributors[0]
	}

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (*SpatialBatchAuditReport, error) {
		owner := plots.Projects.Sqlite
		rawRows, err := queryScopedSpatialPlotRows(ctx, owner)
		if err != nil {
			return nil, err
		}
		report := &SpatialBatchAuditReport{
			ContextID:  contextID,
			Project:    owner.Selection.Project,
			SU:         owner.Selection.SU,
			TotalPlots: len(rawRows),
			Rows:       make([]SpatialPlotQARow, 0, len(rawRows)),
		}
		for _, r := range rawRows {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			qa := buildPlotQARow(r, attr)
			if qa.NormalizedLat != nil || qa.UTMDerivedLat != nil {
				report.CoordinatizedPlots++
				if strings.TrimSpace(qa.StoredGISBGC) == "" && qa.SuggestedGISBGC != "" {
					report.MissingGISBGCCount++
				}
			}
			if qa.LongitudeSignFlipped {
				report.UnsignedLonCount++
			}
			if qa.DivergenceFlagged {
				report.UTMDivergenceCount++
			}
			if qa.BGCMismatch {
				report.BGCMismatchCount++
			}
			if qa.ElevationFlagged {
				report.ElevationIssueCount++
			}
			report.Rows = append(report.Rows, qa)
		}
		return report, nil
	})
}

func validateUTF16FieldWidth(field, val string, maxUnits int) error {
	if !utf8.ValidString(val) || strings.ContainsRune(val, 0) {
		return fmt.Errorf("%s contains invalid UTF-8 or NUL bytes", field)
	}
	if maxUnits > 0 && len(utf16.Encode([]rune(val))) > maxUnits {
		return fmt.Errorf("%s value %q exceeds %d UTF-16 code units", field, val, maxUnits)
	}
	return nil
}

func formatAuditBeforeAfter(val any) any {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case string:
		return v
	case *string:
		if v == nil {
			return nil
		}
		return *v
	case int:
		return strconv.Itoa(v)
	case *int:
		if v == nil {
			return nil
		}
		return strconv.Itoa(*v)
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64)
	case *float64:
		if v == nil {
			return nil
		}
		return strconv.FormatFloat(*v, 'g', -1, 64)
	default:
		return fmt.Sprint(v)
	}
}

// ApplySpatialAttributes transactionally stamps GIS_BGC (v13), Ecosection, FSRegionDistrict, NtsMapSheet,
// and/or NAD83 UTM <-> LatLon coordinates inside BEGIN IMMEDIATE with mandatory <Project>_Audit rows.
func ApplySpatialAttributes(ctx context.Context, contexts *vprocore.ContextService, contextID string, req ApplySpatialAttributesRequest, attributors ...*PMTilesAttributor) (*ApplySpatialAttributesResult, error) {
	if ctx == nil {
		return nil, errors.New("spatial stamp requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if contexts == nil {
		return nil, errors.New("spatial context service is unavailable")
	}
	if !req.StampGISBGC && !req.StampAdminFields && !req.NormalizeLongitudeSign && !req.SyncUTMFromLatLon && !req.SyncLatLonFromUTM {
		return nil, errors.New("at least one spatial attribution action must be enabled")
	}
	attr := getDefaultPMTilesAttributor()
	if len(attributors) > 0 && attributors[0] != nil {
		attr = attributors[0]
	}

	return vprocore.WithContextPlotRequest(ctx, contexts, contextID, func(plots *vprocore.PlotService) (*ApplySpatialAttributesResult, error) {
		owner := plots.Projects.Sqlite
		project := owner.Selection.Project
		plots.Mu.RLock()
		currentUser := strings.TrimSpace(plots.CurrentUser)
		plots.Mu.RUnlock()
		if currentUser == "" {
			currentUser = "VProUser"
		}

		rawRows, err := queryScopedSpatialPlotRows(ctx, owner)
		if err != nil {
			return nil, err
		}
		selectedSet := make(map[string]bool, len(req.PlotNumbers))
		for _, p := range req.PlotNumbers {
			if p == "" || !utf8.ValidString(p) || strings.ContainsRune(p, 0) {
				return nil, fmt.Errorf("invalid plot number %q", p)
			}
			selectedSet[p] = true
		}
		if len(selectedSet) == 0 && !req.AllScopedPlots {
			return nil, errors.New("specify plotNumbers or set allScopedPlots=true")
		}

		var targets []rawSpatialPlotRow
		for _, r := range rawRows {
			if req.AllScopedPlots || selectedSet[r.plotNumber] {
				targets = append(targets, r)
			}
		}
		if len(targets) == 0 {
			return &ApplySpatialAttributesResult{ContextID: contextID, Project: project}, nil
		}

		type cellMutation struct {
			tableSuffix string // "Env" | "Admin"
			column      string
			beforeVal   any
			afterVal    any
		}
		type plotPlan struct {
			plotNumber string
			mutations  []cellMutation
		}
		var plans []plotPlan

		for _, r := range targets {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			geod := EvaluatePlotGeodesy(PlotGeodesyInput{
				Latitude:         r.latitude,
				Longitude:        r.longitude,
				UTMZone:          r.utmZone,
				UTMEasting:       r.utmEasting,
				UTMNorthing:      r.utmNorthing,
				LocationAccuracy: r.locationAccuracy,
			})
			var sug *SpatialAttributeSuggestion
			if geod.NormalizedLat != nil && geod.NormalizedLon != nil {
				sug, _ = attr.SuggestSpatialAttributes(*geod.NormalizedLat, *geod.NormalizedLon)
			} else if geod.UTMDerivedLat != nil && geod.UTMDerivedLon != nil {
				sug, _ = attr.SuggestSpatialAttributes(*geod.UTMDerivedLat, *geod.UTMDerivedLon)
			}

			var muts []cellMutation

			if req.NormalizeLongitudeSign && geod.LongitudeSignFlipped && geod.NormalizedLon != nil && r.longitude != nil {
				muts = append(muts, cellMutation{
					tableSuffix: "Env",
					column:      "Longitude",
					beforeVal:   *r.longitude,
					afterVal:    *geod.NormalizedLon,
				})
			}

			if req.SyncUTMFromLatLon && geod.ComputedUTMZone != nil && geod.ComputedUTMEasting != nil && geod.ComputedUTMNorthing != nil {
				zStr := strconv.Itoa(*geod.ComputedUTMZone)
				if err := validateUTF16FieldWidth("UTMZone", zStr, 2); err != nil {
					return nil, err
				}
				eastRounded := math.Round(*geod.ComputedUTMEasting*10.0) / 10.0
				northRounded := math.Round(*geod.ComputedUTMNorthing*10.0) / 10.0
				allUTMBlank := (r.utmZone == nil || strings.TrimSpace(*r.utmZone) == "") && r.utmEasting == nil && r.utmNorthing == nil
				if !req.OnlyWhenBlank || allUTMBlank {
					var beforeZ, beforeE, beforeN any
					if r.utmZone != nil {
						beforeZ = *r.utmZone
					}
					if r.utmEasting != nil {
						beforeE = *r.utmEasting
					}
					if r.utmNorthing != nil {
						beforeN = *r.utmNorthing
					}
					if beforeZ != zStr {
						muts = append(muts, cellMutation{"Env", "UTMZone", beforeZ, zStr})
					}
					if r.utmEasting == nil || math.Abs(*r.utmEasting-eastRounded) > 0.05 {
						muts = append(muts, cellMutation{"Env", "UTMEasting", beforeE, eastRounded})
					}
					if r.utmNorthing == nil || math.Abs(*r.utmNorthing-northRounded) > 0.05 {
						muts = append(muts, cellMutation{"Env", "UTMNorthing", beforeN, northRounded})
					}
				}
			}

			if req.SyncLatLonFromUTM && geod.UTMDerivedLat != nil && geod.UTMDerivedLon != nil {
				latRounded := math.Round(*geod.UTMDerivedLat*1e6) / 1e6
				lonRounded := math.Round(*geod.UTMDerivedLon*1e6) / 1e6
				bothDDBlank := r.latitude == nil && r.longitude == nil
				if !req.OnlyWhenBlank || bothDDBlank {
					var beforeLat, beforeLon any
					if r.latitude != nil {
						beforeLat = *r.latitude
					}
					if r.longitude != nil {
						beforeLon = *r.longitude
					}
					if r.latitude == nil || math.Abs(*r.latitude-latRounded) > 1e-6 {
						muts = append(muts, cellMutation{"Env", "Latitude", beforeLat, latRounded})
					}
					if r.longitude == nil || math.Abs(*r.longitude-lonRounded) > 1e-6 {
						muts = append(muts, cellMutation{"Env", "Longitude", beforeLon, lonRounded})
					}
				}
			}

			if req.StampGISBGC && sug != nil && sug.GISBGC != "" {
				if err := validateUTF16FieldWidth("GIS_BGC", sug.GISBGC, 255); err != nil {
					return nil, err
				}
				if !req.OnlyWhenBlank || strings.TrimSpace(r.gisBGC) == "" {
					var beforeGIS any
					if r.gisBGC != "" {
						beforeGIS = r.gisBGC
					}
					if r.gisBGC != sug.GISBGC {
						muts = append(muts, cellMutation{"Admin", "GIS_BGC", beforeGIS, sug.GISBGC})
					}
					var beforeVer any
					if r.gisBGCVer != nil {
						beforeVer = *r.gisBGCVer
					}
					if r.gisBGCVer == nil || *r.gisBGCVer != CurrentBECGISVersion {
						muts = append(muts, cellMutation{"Admin", "GIS_BGC_VER", beforeVer, CurrentBECGISVersion})
					}
				}
			}

			if req.StampAdminFields && sug != nil {
				if sug.Ecosection != "" {
					if err := validateUTF16FieldWidth("Ecosection", sug.Ecosection, 3); err != nil {
						return nil, err
					}
					if !req.OnlyWhenBlank || strings.TrimSpace(r.ecosection) == "" {
						var before any
						if r.ecosection != "" {
							before = r.ecosection
						}
						if r.ecosection != sug.Ecosection {
							muts = append(muts, cellMutation{"Env", "Ecosection", before, sug.Ecosection})
						}
					}
				}
				if sug.FSRegionDistrict != "" {
					if err := validateUTF16FieldWidth("FSRegionDistrict", sug.FSRegionDistrict, 7); err != nil {
						return nil, err
					}
					if !req.OnlyWhenBlank || strings.TrimSpace(r.fsRegionDistrict) == "" {
						var before any
						if r.fsRegionDistrict != "" {
							before = r.fsRegionDistrict
						}
						if r.fsRegionDistrict != sug.FSRegionDistrict {
							muts = append(muts, cellMutation{"Env", "FSRegionDistrict", before, sug.FSRegionDistrict})
						}
					}
				}
				if sug.NtsMapSheet != "" {
					if err := validateUTF16FieldWidth("NtsMapSheet", sug.NtsMapSheet, 8); err != nil {
						return nil, err
					}
					if !req.OnlyWhenBlank || strings.TrimSpace(r.ntsMapSheet) == "" {
						var before any
						if r.ntsMapSheet != "" {
							before = r.ntsMapSheet
						}
						if r.ntsMapSheet != sug.NtsMapSheet {
							muts = append(muts, cellMutation{"Env", "NtsMapSheet", before, sug.NtsMapSheet})
						}
					}
				}
				if sug.ProvinceStateTerritory != "" {
					if err := validateUTF16FieldWidth("ProvinceStateTerritory", sug.ProvinceStateTerritory, 255); err != nil {
						return nil, err
					}
					if !req.OnlyWhenBlank || strings.TrimSpace(r.province) == "" {
						var before any
						if r.province != "" {
							before = r.province
						}
						if r.province != sug.ProvinceStateTerritory {
							muts = append(muts, cellMutation{"Admin", "ProvinceStateTerritory", before, sug.ProvinceStateTerritory})
						}
					}
				}
			}

			if len(muts) > 0 {
				plans = append(plans, plotPlan{plotNumber: r.plotNumber, mutations: muts})
			}
		}

		if len(plans) == 0 {
			return &ApplySpatialAttributesResult{ContextID: contextID, Project: project}, nil
		}

		res := &ApplySpatialAttributesResult{ContextID: contextID, Project: project}
		err = owner.WithMetadataWriter(ctx, func(conn *sql.Conn) (txErr error) {
			tx, bErr := conn.BeginTx(ctx, nil)
			if bErr != nil {
				return bErr
			}
			defer func() {
				if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
					txErr = errors.Join(txErr, rb)
				}
			}()

			qEnv := vprocore.QuoteHeaderIdentifier(project + "_Env")
			qAdmin := vprocore.QuoteHeaderIdentifier(project + "_Admin")
			qAudit := vprocore.QuoteHeaderIdentifier(project + "_Audit")
			now := time.Now().Format("2006-01-02 15:04:05")

			auditStmt, aErr := tx.PrepareContext(ctx,
				`INSERT INTO main.`+qAudit+` ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag") VALUES (?,?,?,?,?,?,?,?,0,0)`)
			if aErr != nil {
				return aErr
			}
			defer auditStmt.Close()

			for _, plan := range plans {
				if err := ctx.Err(); err != nil {
					return err
				}
				// Ensure 1:1 Admin row exists for this literal plot before updating Admin columns
				var hasAdminMuts bool
				for _, m := range plan.mutations {
					if m.tableSuffix == "Admin" {
						hasAdminMuts = true
						break
					}
				}
				if hasAdminMuts {
					var adminCnt int
					if err := tx.QueryRowContext(ctx,
						`SELECT COUNT(*) FROM main.`+qAdmin+` WHERE typeof("Plot")='text' AND CAST("Plot" AS BLOB)=CAST(? AS BLOB)`,
						plan.plotNumber).Scan(&adminCnt); err != nil {
						return err
					}
					if adminCnt == 0 {
						if _, err := tx.ExecContext(ctx, `INSERT INTO main.`+qAdmin+` ("Plot") VALUES (?)`, plan.plotNumber); err != nil {
							return err
						}
					} else if adminCnt > 1 {
						return fmt.Errorf("plot %q has duplicate Admin rows", plan.plotNumber)
					}
				}

				for _, m := range plan.mutations {
					qTarget := qEnv
					pkCol := `"PlotNumber"`
					if m.tableSuffix == "Admin" {
						qTarget = qAdmin
						pkCol = `"Plot"`
					}
					qCol := vprocore.QuoteHeaderIdentifier(m.column)
					execRes, err := tx.ExecContext(ctx,
						`UPDATE main.`+qTarget+` SET `+qCol+` = ? WHERE typeof(`+pkCol+`)='text' AND CAST(`+pkCol+` AS BLOB)=CAST(? AS BLOB)`,
						m.afterVal, plan.plotNumber)
					if err != nil {
						return fmt.Errorf("update %s.%s for plot %s: %w", m.tableSuffix, m.column, plan.plotNumber, err)
					}
					affected, _ := execRes.RowsAffected()
					if affected != 1 {
						return fmt.Errorf("expected 1 updated row in %s for plot %q, got %d", m.tableSuffix, plan.plotNumber, affected)
					}
					if _, err := auditStmt.ExecContext(ctx,
						project,
						currentUser,
						plan.plotNumber,
						"_"+m.tableSuffix,
						m.column,
						now,
						formatAuditBeforeAfter(m.beforeVal),
						formatAuditBeforeAfter(m.afterVal),
					); err != nil {
						return fmt.Errorf("insert audit row for plot %s: %w", plan.plotNumber, err)
					}
					res.UpdatedCells++
					res.AuditRows++
				}
				res.UpdatedPlots++
			}

			if err := ctx.Err(); err != nil {
				return err
			}
			return tx.Commit()
		})
		if err != nil {
			return nil, err
		}
		return res, nil
	})
}
