package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func metadataText(value string) ProjectMetadataCell {
	return ProjectMetadataCell{Storage: "text", Text: &value}
}

func metadataInteger(value string) ProjectMetadataCell {
	return ProjectMetadataCell{Storage: "integer", Integer: &value}
}

func TestProjectMetadataFieldPoliciesCoverEveryEditableSourceColumn(t *testing.T) {
	service, state := projectMetadataFixture(t)
	review, err := service.ReviewProjectMetadata(t.Context(), state.ContextID, "META1")
	if err != nil {
		t.Fatal(err)
	}
	if len(projectMetadataFields) != 70 {
		t.Fatal("metadata must cover all70 non-identity/non-stamped columns")
	}
	readOnly := map[string]bool{"ID": true, "ProjectID": true, "AllSpecs": true, "TableOfLists": true, "DateLastEdited": true}
	for _, column := range review.ProjectRecords.Columns {
		field, editable := projectMetadataFields[column.Name]
		if editable == readOnly[column.Name] {
			t.Fatal("metadata source column omitted or identity/stamp enabled:", column.Name)
		}
		if !editable {
			continue
		}
		for _, cell := range []ProjectMetadataCell{{Storage: "null"}, metadataText("literal")} {
			if field.kind == "integer" {
				cell = metadataInteger("1")
			}
			if _, err := validateProjectMetadataAssignment(column.Name, cell); err != nil {
				t.Fatal(column.Name, err)
			}
		}
	}
}

func TestProjectMetadataPhysicalTextBoundsAndLiteralPreservation(t *testing.T) {
	for column, field := range projectMetadataFields {
		if field.kind != "text" {
			continue
		}
		t.Run(column, func(t *testing.T) {
			for _, text := range []string{"", "  Mixed CASE  ", "日本語", "🌲"} {
				value, err := validateProjectMetadataAssignment(column, metadataText(text))
				if err != nil || value != text {
					t.Fatal("metadata text was normalized:", value, err)
				}
			}
			if _, err := validateProjectMetadataAssignment(column, metadataText(string([]byte{0xff}))); err == nil {
				t.Fatal("malformed Unicode was repaired")
			}
			if field.maximum > 0 {
				boundary := strings.Repeat("🌲", field.maximum/2) + strings.Repeat("x", field.maximum%2)
				if _, err := validateProjectMetadataAssignment(column, metadataText(boundary)); err != nil {
					t.Fatal("UTF-16 boundary rejected:", err)
				}
				if _, err := validateProjectMetadataAssignment(column, metadataText(boundary+"x")); err == nil {
					t.Fatal("UTF-16 overlength accepted")
				}
			} else if _, err := validateProjectMetadataAssignment(column, metadataText(strings.Repeat("x", 100000))); err != nil {
				t.Fatal("source memo acquired an invented text bound:", err)
			}
		})
	}
}

func TestProjectMetadataIntegersOptionsAndNoImplicitTemplateConversion(t *testing.T) {
	for column, field := range projectMetadataFields {
		if field.kind != "integer" {
			continue
		}
		t.Run(column, func(t *testing.T) {
			valid := []string{"-32768", "0", "32767"}
			invalid := []string{"-32769", "32768"}
			if field.maximum == 32 {
				valid = []string{"-2147483648", "0", "2147483647"}
				invalid = []string{"-2147483649", "2147483648"}
			}
			if field.options {
				valid, invalid = []string{"1", "2", "3"}, []string{"-1", "0", "4", "32767"}
			}
			for _, number := range valid {
				value, err := validateProjectMetadataAssignment(column, metadataInteger(number))
				if err != nil || strconv.FormatInt(value.(int64), 10) != number {
					t.Fatal("source integer changed:", number, value, err)
				}
			}
			for _, number := range append(invalid, "1.5", "+1", "01", "-0", " 1", "9007199254740993") {
				if _, err := validateProjectMetadataAssignment(column, metadataInteger(number)); err == nil {
					t.Fatal("invalid/noncanonical integer accepted:", number)
				}
			}
			if _, err := validateProjectMetadataAssignment(column, metadataText("2000-01-02 03:04:05")); err == nil {
				t.Fatal("master timestamp/text was implicitly converted to project integer")
			}
			if _, err := validateProjectMetadataAssignment(column, ProjectMetadataCell{Storage: "null"}); err != nil {
				t.Fatal("source nullable integer rejected:", err)
			}
		})
	}
	for _, column := range []string{"GeoRefMethod", "Datum", "CoordinateSystem", "VegCoverMethod", "PlotMethod", "MensurationMethod"} {
		if _, err := validateProjectMetadataAssignment(column, metadataInteger("7")); err == nil {
			t.Fatal("numeric master method code implicitly stringified:", column)
		}
	}
}

func TestProjectMetadataAssignmentsRejectUnavailableAndConflictingTaggedValues(t *testing.T) {
	for _, column := range []string{"ID", "ProjectID", "AllSpecs", "TableOfLists", "DateLastEdited", "Unknown", "projecttitle"} {
		if _, err := validateProjectMetadataAssignment(column, metadataText("literal")); err == nil {
			t.Fatal("unavailable/identity/stamp assignment accepted:", column)
		}
	}
	one := 1.0
	text := "literal"
	for _, cell := range []ProjectMetadataCell{
		{Storage: "null", Text: &text},
		{Storage: "text"},
		{Storage: "integer"},
		{Storage: "real"},
		{Storage: "text", Text: &text, Real: &one},
		{Storage: "blob", BlobHex: &text},
		{Storage: "boolean"},
	} {
		if _, err := validateProjectMetadataAssignment("ProjectTitle", cell); err == nil {
			t.Fatal("conflicting/incomplete tagged value accepted:", cell)
		}
	}
}

func TestProjectMetadataDraftsOmitUnchangedHistoricalValuesAndKeepTypedOriginals(t *testing.T) {
	blob := "00ff"
	columns := []ProjectMetadataColumn{{Name: "CollectedSite"}, {Name: "ProjectTitle"}, {Name: "Notes"}, {Name: "StartDate"}}
	original := ProjectMetadataRow{RowID: "-200", Cells: []ProjectMetadataCell{
		metadataInteger("-1"), metadataText(strings.Repeat("x", 300)),
		{Storage: "blob", BlobHex: &blob}, metadataText("2000-01-02 03:04:05"),
	}}
	changes := []projectMetadataChange{}
	for i, column := range columns {
		changes = append(changes, projectMetadataChange{column.Name, original.Cells[i]})
	}
	assignments, err := prepareProjectMetadataChanges(columns, original, changes)
	if err != nil || len(assignments) != 0 {
		t.Fatal("unchanged historical fields acquired assignments or validation:", assignments, err)
	}
	changes[3].value = metadataInteger("2000")
	assignments, err = prepareProjectMetadataChanges(columns, original, changes)
	if err != nil || len(assignments) != 1 || assignments[0].column != "StartDate" ||
		assignments[0].before.Storage != "text" || *assignments[0].before.Text != "2000-01-02 03:04:05" ||
		assignments[0].value != int64(2000) {
		t.Fatal("explicit year assignment lost its typed original or modified unrelated history:", assignments, err)
	}
	changes[0].value = metadataInteger("0")
	if _, err := prepareProjectMetadataChanges(columns, original, changes); err == nil {
		t.Fatal("new invalid collection option was treated as preserved history")
	}
}

func TestProjectMetadataDraftsRejectMissingRepeatedAndUnavailableProperties(t *testing.T) {
	columns := []ProjectMetadataColumn{{Name: "Notes"}, {Name: "ID"}}
	original := ProjectMetadataRow{RowID: "-200", Cells: []ProjectMetadataCell{metadataText("original"), metadataInteger("-200")}}
	for _, changes := range [][]projectMetadataChange{
		nil,
		{{column: "Notes", value: metadataText("new")}, {column: "Notes", value: metadataText("another")}},
		{{column: "ID", value: metadataInteger("-200")}},
		{{column: "Unknown", value: metadataText("new")}},
		{{column: "Notes", value: metadataText(string([]byte{0xff}))}},
	} {
		if _, err := prepareProjectMetadataChanges(columns, original, changes); err == nil {
			t.Fatal("ambiguous/malformed draft accepted:", changes)
		}
	}
	for _, schema := range [][]ProjectMetadataColumn{nil, {{Name: "Notes"}}, {{Name: "Notes"}, {Name: "Notes"}}, {{Name: "Notes"}, {}}} {
		if _, err := prepareProjectMetadataChanges(schema, original, []projectMetadataChange{{column: "Notes", value: metadataText("new")}}); err == nil {
			t.Fatal("incomplete/ambiguous draft schema accepted:", schema)
		}
	}
}

func TestProjectMetadataReferencesPreserveSourceBindingsAndListPolicies(t *testing.T) {
	references, strict := 0, 0
	for column, field := range projectMetadataFields {
		if field.referenceList == "" {
			continue
		}
		references++
		if strings.HasPrefix(column, "DataQuality") {
			if field.referenceList != "PlotQualitySite" || field.referenceColumn != "Note" ||
				field.limitToList != (column != "DataQualitySite") {
				t.Fatal("quality fields must store source Note, not Item or a numeric code:", column, field)
			}
		} else if field.referenceColumn != "Item" || field.limitToList {
			t.Fatal("ordinary reference combo gained strict membership or changed its binding:", column, field)
		}
		if field.limitToList {
			strict++
		}
	}
	if references != 18 || strict != 8 {
		t.Fatal("source reference batch changed:", references, strict)
	}
	if projectMetadataFields["GeoRefMethod"].referenceList != "GeoreferenceMethod" ||
		projectMetadataFields["CoordinateSystem"].referenceList != "CoordSystem" {
		t.Fatal("source list names replaced with physical/master column names")
	}
}

func TestProjectMetadataPoliciesMatchSealedAccessTableDefinition(t *testing.T) {
	source := filepath.Join("testdata", "projectmetadata", "Sample_Metadata_CreateSQL.txt")
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	columns := regexp.MustCompile(`(?m)^\s*\[([^]]+)\] (TEXT\((\d+)\)|INTEGER|LONG|DATETIME|MEMO)`).FindAllStringSubmatch(string(data), -1)
	if len(columns) != 75 {
		t.Fatal("sealed Access table definition must contain75 columns")
	}
	for _, column := range columns {
		field, editable := projectMetadataFields[column[1]]
		if !editable {
			continue
		}
		switch {
		case strings.HasPrefix(column[2], "TEXT"):
			maximum, err := strconv.Atoi(column[3])
			if err != nil || field.kind != "text" || field.maximum != maximum {
				t.Fatal("source text policy mismatch:", column[1])
			}
		case column[2] == "MEMO":
			if field.kind != "text" || field.maximum != 0 {
				t.Fatal("source memo policy mismatch")
			}
		case column[2] == "INTEGER" || column[2] == "LONG":
			bits := 16
			if column[2] == "LONG" {
				bits = 32
			}
			if field.kind != "integer" || field.maximum != bits {
				t.Fatal("source integer policy mismatch:", column[1])
			}
		default:
			t.Fatal("unsupported editable source type:", column)
		}
	}
}
