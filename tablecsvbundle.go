package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"slices"
	"sort"
	"unicode/utf8"
)

// This private container publishes neither files nor databases. Its byte budget
// bounds both the whole input archive and the aggregate uncompressed members.
func encodeTableCSVBundle(ctx context.Context, document tableCSVDocument) ([]byte, error) {
	document = snapshotTableCSVBundleDocument(document)
	if _, err := decodeTableCSV(ctx, document); err != nil {
		return nil, fmt.Errorf("table CSV bundle document: %w", err)
	}
	manifest, err := json.Marshal(document.Manifest)
	if err != nil {
		return nil, fmt.Errorf("table CSV bundle manifest: %w", err)
	}
	if err := validateTableCSVBundleManifest(ctx, manifest); err != nil {
		return nil, fmt.Errorf("table CSV bundle manifest: %w", err)
	}
	return encodeTableCSVBundleMembers(ctx, document.Data, manifest)
}

func encodeTableCSVBundleMembers(ctx context.Context, csv, manifest []byte) ([]byte, error) {
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, member := range []struct {
		name string
		data []byte
	}{{"table.csv", csv}, {"manifest.json", manifest}} {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("table CSV bundle encode: %w", err)
		}
		header := &zip.FileHeader{Name: member.name, Method: zip.Store,
			CreatorVersion: 20, ReaderVersion: 20, ModifiedDate: 33,
			CRC32:            crc32.ChecksumIEEE(member.data),
			CompressedSize64: uint64(len(member.data)), UncompressedSize64: uint64(len(member.data))}
		header.SetMode(0600)
		destination, err := writer.CreateRaw(header)
		if err != nil {
			return nil, fmt.Errorf("table CSV bundle member %s: %w", member.name, err)
		}
		if _, err := io.Copy(destination, tableCSVBundleReader{ctx, bytes.NewReader(member.data)}); err != nil {
			return nil, fmt.Errorf("table CSV bundle member %s: %w", member.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("table CSV bundle close: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("table CSV bundle encode: %w", err)
	}
	return append([]byte(nil), output.Bytes()...), nil
}

func decodeTableCSVBundle(ctx context.Context, encoded []byte, byteBudget int64) (tableCSVDocument, error) {
	var zero tableCSVDocument
	contents, err := decodeTableCSVBundleMembers(ctx, encoded, byteBudget)
	if err != nil {
		return zero, err
	}
	if err := validateTableCSVBundleManifest(ctx, contents["manifest.json"]); err != nil {
		return zero, fmt.Errorf("table CSV bundle manifest: %w", err)
	}
	var manifest TableCSVManifest
	if err := json.Unmarshal(contents["manifest.json"], &manifest); err != nil {
		return zero, fmt.Errorf("table CSV bundle manifest decode: %w", err)
	}
	document := tableCSVDocument{Manifest: manifest, Data: contents["table.csv"]}
	if _, err := decodeTableCSV(ctx, document); err != nil {
		return zero, fmt.Errorf("table CSV bundle document: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return zero, fmt.Errorf("table CSV bundle decode: %w", err)
	}
	return document, nil
}

func decodeTableCSVBundleMembers(ctx context.Context, encoded []byte, byteBudget int64) (map[string][]byte, error) {
	var zero map[string][]byte
	if byteBudget <= 0 || uint64(len(encoded)) > uint64(byteBudget) {
		return zero, errors.New("table CSV bundle requires a positive byte budget covering the raw archive")
	}
	encoded = slices.Clone(encoded)
	if err := ctx.Err(); err != nil {
		return zero, fmt.Errorf("table CSV bundle decode: %w", err)
	}
	reader, err := zip.NewReader(tableCSVBundleReaderAt{ctx, bytes.NewReader(encoded)}, int64(len(encoded)))
	if err != nil {
		return zero, fmt.Errorf("table CSV bundle ZIP: %w", err)
	}
	if len(reader.File) != 2 {
		return zero, errors.New("table CSV bundle requires exactly table.csv and manifest.json")
	}
	remaining := uint64(byteBudget)
	members := make(map[string]*zip.File, 2)
	for _, member := range reader.File {
		if err := ctx.Err(); err != nil {
			return zero, fmt.Errorf("table CSV bundle member: %w", err)
		}
		if (member.Name != "table.csv" && member.Name != "manifest.json") || members[member.Name] != nil {
			return zero, fmt.Errorf("table CSV bundle unknown or duplicate member %q", member.Name)
		}
		if !member.Mode().IsRegular() || member.Flags & ^uint16(0x808) != 0 || member.Method != zip.Store {
			return zero, fmt.Errorf("table CSV bundle member %s is not a regular unencrypted ZIP Store member", member.Name)
		}
		// Subtraction avoids overflow even for malicious ZIP64 size claims.
		if member.CompressedSize64 != member.UncompressedSize64 || member.UncompressedSize64 > remaining {
			return zero, fmt.Errorf("table CSV bundle member %s exceeds the aggregate byte budget or Store size", member.Name)
		}
		remaining -= member.UncompressedSize64
		members[member.Name] = member
	}
	if err := validateTableCSVBundleHeaders(ctx, encoded, reader.File); err != nil {
		return zero, fmt.Errorf("table CSV bundle headers: %w", err)
	}
	contents := make(map[string][]byte, 2)
	for _, name := range []string{"table.csv", "manifest.json"} {
		member := members[name]
		source, err := member.Open()
		if err != nil {
			return zero, fmt.Errorf("table CSV bundle member %s open: %w", name, err)
		}
		// File.Open bounds Store reads to the declared size and verifies CRC at EOF.
		data, readErr := io.ReadAll(tableCSVBundleReader{ctx, source})
		closeErr := source.Close()
		if readErr != nil {
			return zero, fmt.Errorf("table CSV bundle member %s read: %w", name, readErr)
		}
		if closeErr != nil {
			return zero, fmt.Errorf("table CSV bundle member %s close: %w", name, closeErr)
		}
		if uint64(len(data)) != member.UncompressedSize64 || crc32.ChecksumIEEE(data) != member.CRC32 {
			return zero, fmt.Errorf("table CSV bundle member %s size or CRC mismatch", name)
		}
		contents[name] = data
	}
	if err := ctx.Err(); err != nil {
		return zero, fmt.Errorf("table CSV bundle decode: %w", err)
	}
	return contents, nil
}

// Snapshot before context callbacks or validation. A JSON roundtrip here could
// repair malformed caller text before its original bytes have been rejected.
func snapshotTableCSVBundleDocument(document tableCSVDocument) tableCSVDocument {
	document.Data = slices.Clone(document.Data)
	manifest := &document.Manifest
	manifest.Columns = slices.Clone(manifest.Columns)
	manifest.RowIDs = slices.Clone(manifest.RowIDs)
	manifest.Storage = slices.Clone(manifest.Storage)
	for index := range manifest.Storage {
		manifest.Storage[index] = slices.Clone(manifest.Storage[index])
	}
	manifest.Descriptions = slices.Clone(manifest.Descriptions)
	for index := range manifest.Descriptions {
		cell := &manifest.Descriptions[index].Value
		if cell.Text != nil {
			value := *cell.Text
			cell.Text = &value
		}
		if cell.Integer != nil {
			value := *cell.Integer
			cell.Integer = &value
		}
		if cell.Real != nil {
			value := *cell.Real
			cell.Real = &value
		}
		if cell.BlobHex != nil {
			value := *cell.BlobHex
			cell.BlobHex = &value
		}
	}
	return document
}

type tableCSVBundleReader = artifactPublicationReader

type tableCSVBundleReaderAt struct {
	ctx    context.Context
	source io.ReaderAt
}

func (reader tableCSVBundleReaderAt) ReadAt(data []byte, offset int64) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.source.ReadAt(data, offset)
}

// archive/zip primarily trusts central headers. Compare their local counterparts
// too, so a changed name, method, encryption flag or descriptor is not ignored.
func validateTableCSVBundleHeaders(ctx context.Context, data []byte, members []*zip.File) error {
	ordered := append([]*zip.File(nil), members...)
	offsets := make(map[*zip.File]uint64, 2)
	for _, member := range ordered {
		offset, err := member.DataOffset()
		if err != nil {
			return fmt.Errorf("member %s offset: %w", member.Name, err)
		}
		if offset < 0 || uint64(offset) > uint64(len(data)) {
			return fmt.Errorf("member %s offset is outside the archive", member.Name)
		}
		offsets[member] = uint64(offset)
	}
	sort.Slice(ordered, func(i, j int) bool { return offsets[ordered[i]] < offsets[ordered[j]] })
	cursor := uint64(0)
	for _, member := range ordered {
		if err := ctx.Err(); err != nil {
			return err
		}
		if cursor > uint64(len(data)) || uint64(len(data))-cursor < 30 {
			return errors.New("truncated local ZIP header")
		}
		header := data[cursor : cursor+30]
		if binary.LittleEndian.Uint32(header) != 0x04034b50 {
			return errors.New("missing local ZIP header")
		}
		flags := binary.LittleEndian.Uint16(header[6:])
		nameSize := uint64(binary.LittleEndian.Uint16(header[26:]))
		extraSize := uint64(binary.LittleEndian.Uint16(header[28:]))
		start := cursor + 30
		if nameSize+extraSize > uint64(len(data))-start {
			return errors.New("truncated local ZIP name or extra data")
		}
		if string(data[start:start+nameSize]) != member.Name || flags != member.Flags ||
			binary.LittleEndian.Uint16(header[8:]) != member.Method ||
			start+nameSize+extraSize != offsets[member] {
			return fmt.Errorf("member %s local and central headers differ", member.Name)
		}
		if flags&8 == 0 {
			compressed := uint64(binary.LittleEndian.Uint32(header[18:]))
			uncompressed := uint64(binary.LittleEndian.Uint32(header[22:]))
			if compressed == 0xffffffff || uncompressed == 0xffffffff {
				// ZIP64 local sizes occur first in the corresponding extra field.
				extra := data[start+nameSize : start+nameSize+extraSize]
				found := false
				for len(extra) >= 4 {
					size := int(binary.LittleEndian.Uint16(extra[2:]))
					if size > len(extra)-4 {
						return errors.New("truncated local ZIP64 extra field")
					}
					if binary.LittleEndian.Uint16(extra) == 1 {
						values := extra[4 : 4+size]
						for _, target := range []*uint64{&uncompressed, &compressed} {
							if *target == 0xffffffff {
								if len(values) < 8 {
									return errors.New("missing local ZIP64 size")
								}
								*target = binary.LittleEndian.Uint64(values)
								values = values[8:]
							}
						}
						found = true
						break
					}
					extra = extra[4+size:]
				}
				if !found {
					return errors.New("missing local ZIP64 extra field")
				}
			}
			if binary.LittleEndian.Uint32(header[14:]) != member.CRC32 ||
				compressed != member.CompressedSize64 || uncompressed != member.UncompressedSize64 {
				return fmt.Errorf("member %s local CRC or sizes differ", member.Name)
			}
		}
		cursor = offsets[member]
		if member.CompressedSize64 > uint64(len(data))-cursor {
			return fmt.Errorf("member %s data is truncated", member.Name)
		}
		cursor += member.CompressedSize64
		if flags&8 != 0 {
			if uint64(len(data))-cursor < 12 {
				return errors.New("truncated ZIP data descriptor")
			}
			if binary.LittleEndian.Uint32(data[cursor:]) == 0x08074b50 {
				cursor += 4
			}
			size := uint64(12)
			if member.CompressedSize64 >= 0xffffffff || member.UncompressedSize64 >= 0xffffffff {
				size = 20
			}
			if size > uint64(len(data))-cursor {
				return errors.New("truncated ZIP data descriptor sizes")
			}
			descriptor := data[cursor : cursor+size]
			compressed := uint64(binary.LittleEndian.Uint32(descriptor[4:]))
			uncompressed := uint64(binary.LittleEndian.Uint32(descriptor[8:]))
			if size == 20 {
				compressed = binary.LittleEndian.Uint64(descriptor[4:])
				uncompressed = binary.LittleEndian.Uint64(descriptor[12:])
			}
			if binary.LittleEndian.Uint32(descriptor) != member.CRC32 ||
				compressed != member.CompressedSize64 || uncompressed != member.UncompressedSize64 {
				return fmt.Errorf("member %s data descriptor differs", member.Name)
			}
			cursor += size
		}
	}
	if uint64(len(data))-cursor < 4 || binary.LittleEndian.Uint32(data[cursor:]) != 0x02014b50 {
		return errors.New("unexpected data between ZIP members and central directory")
	}
	return nil
}

// Bounded to the four object shapes in this manifest, not an application-wide
// JSON policy. Inspect every raw string before encoding/json can repair Unicode.
func validateTableCSVBundleManifest(ctx context.Context, raw []byte) error {
	return validateTableCSVBundleJSON(ctx, raw, "manifest")
}

func validateTableCSVBundleJSON(ctx context.Context, raw []byte, shape string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return errors.New("manifest must be one syntax-valid UTF-8 JSON value")
	}
	for index := 0; index < len(raw); index++ {
		if index%4096 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		if raw[index] != '"' {
			continue
		}
		start := index
		for index++; index < len(raw); index++ {
			if index%4096 == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			if raw[index] == '\\' {
				index++
			} else if raw[index] == '"' {
				break
			}
		}
		if err := validateQualityJSONToken(raw[start : index+1]); err != nil {
			return fmt.Errorf("manifest string Unicode: %w", err)
		}
	}
	return validateTableCSVBundleObject(ctx, raw, shape)
}

func validateTableCSVBundleObject(ctx context.Context, raw []byte, shape string) error {
	var fields []string
	switch shape {
	case "manifest":
		fields = []string{"version", "table", "columns", "rowIds", "storage", "descriptions", "sha256"}
	case "owned-manifest":
		fields = []string{"format", "version", "descriptionMetadataPresent", "document"}
	case "column":
		fields = []string{"name", "declaredType"}
	case "description":
		fields = []string{"rowId", "value"}
	case "cell":
		fields = []string{"storage", "text", "integer", "real", "blobHex"}
	default:
		return errors.New("unsupported manifest object shape")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token != json.Delim('{') {
		return fmt.Errorf("%s must be a JSON object", shape)
	}
	seen := make(map[string]bool, len(fields))
	for decoder.More() {
		if err := ctx.Err(); err != nil {
			return err
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok {
			return fmt.Errorf("%s property is not a string", shape)
		}
		known := false
		for _, field := range fields {
			known = known || field == key
		}
		if !known || seen[key] {
			return fmt.Errorf("%s unknown or duplicate property %q", shape, key)
		}
		seen[key] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
		if shape == "column" && key == "declaredType" {
			if rawType := bytes.TrimSpace(value); len(rawType) == 0 || rawType[0] != '"' {
				return errors.New("column declaredType must be a JSON string, including explicit empty text")
			}
		}
		switch {
		case shape == "owned-manifest" && key == "descriptionMetadataPresent":
			literal := bytes.TrimSpace(value)
			if !bytes.Equal(literal, []byte("true")) && !bytes.Equal(literal, []byte("false")) {
				return errors.New("descriptionMetadataPresent must be an explicit JSON boolean")
			}
		case shape == "owned-manifest" && key == "document":
			if err := validateTableCSVBundleObject(ctx, value, "manifest"); err != nil {
				return fmt.Errorf("owned document: %w", err)
			}
		case shape == "manifest" && (key == "columns" || key == "descriptions"):
			child := "column"
			if key == "descriptions" {
				child = "description"
			}
			array := json.NewDecoder(bytes.NewReader(value))
			token, err := array.Token()
			if err != nil {
				return err
			}
			if token != json.Delim('[') {
				return fmt.Errorf("%s must be a JSON array", key)
			}
			for array.More() {
				if err := ctx.Err(); err != nil {
					return err
				}
				var item json.RawMessage
				if err := array.Decode(&item); err != nil {
					return err
				}
				if err := validateTableCSVBundleObject(ctx, item, child); err != nil {
					return fmt.Errorf("%s: %w", key, err)
				}
			}
			if _, err := array.Token(); err != nil {
				return err
			}
		case shape == "description" && key == "value":
			if err := validateTableCSVBundleObject(ctx, value, "cell"); err != nil {
				return fmt.Errorf("description value: %w", err)
			}
		}
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	for _, field := range fields {
		if !seen[field] {
			return fmt.Errorf("%s missing property %q", shape, field)
		}
	}
	return ctx.Err()
}
