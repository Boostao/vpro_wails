package accessreader

// Column describes a field in an Access table.
type Column struct {
	Name string
	Type string
}

// NativeColumnObservation is ordered by the native source column number, not
// normalized by name. TypeLabel is MDBTools' Access backend label, not DDL.
// RawSize and RawFlags retain table-definition bytes; size units and flag
// semantics (including identity) are not established by this API.
// Required, nullability, defaults, identity and decimal precision/scale are
// unavailable. A zero raw size or flag byte is not evidence for those facts.
type NativeColumnObservation struct {
	Name               string
	SourceColumnNumber int
	TypeCode           uint8
	TypeLabel          string
	RawSize            uint16
	RawFlags           uint8
	Source             string
}

// Table describes a local or linked Access table.
type Table struct {
	Name        string
	Linked      bool
	Description string
}

// DescriptionEntry is one table-scope Description record from MSysObjects.LvProp.
// Block and Entry are zero-based positions in the native ordered property blocks.
// NameIndex references the property-name list active for this block.
// TypeCode and Flags are source bytes, not evidence of DAO type or NULL semantics.
// TypeState is "unknown" because DAO type mapping is not established.
// RawValue is an owned copy; zero length does not establish NULL versus empty text.
type DescriptionEntry struct {
	Block      int
	Entry      int
	RecordType uint16
	NameIndex  uint16
	TypeCode   uint8
	TypeState  string
	Flags      uint8
	RawValue   []byte
}

// TableDescriptionObservation does not follow links or inspect referenced tables.
// MetadataState is "parsed" or "unavailable": the catalog reader does not
// distinguish absent, NULL or zero-length LvProp here. PropertyState is "recorded", "missing"
// (only for a completely parsed sequence), or "unavailable".
// NullState is always "unknown"; no proven native NULL discriminator is available.
// Source identifies the catalog field; DefinitionProvenance is "unknown" because
// authorship, inheritance and default origins are not established.
type TableDescriptionObservation struct {
	Name                 string
	Linked               bool
	MetadataState        string
	PropertyState        string
	NullState            string
	Source               string
	DefinitionProvenance string
	Entries              []DescriptionEntry
}

// Row contains values in the same order as a reader's Columns.
// NULL is nil; binary fields are []byte.
type Row []any

// QuarantinedRow records a 1-based row index skipped when EnableRowQuarantine is active.
type QuarantinedRow struct {
	RowNumber int
	Reason    string
}
