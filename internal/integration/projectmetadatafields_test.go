package integration

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestProjectMetadataFieldPoliciesCoverEveryEditableSourceColumn(t *testing.T) {
	service, state := vprotest.ProjectMetadataFixture(t)
	review, err := service.ReviewProjectMetadata(t.Context(), state.ContextID, "META1")
	if err != nil {
		t.Fatal(err)
	}
	if len(vprocore.ProjectMetadataFields) != 70 {
		t.Fatal("metadata must cover all70 non-identity/non-stamped columns")
	}
	readOnly := map[string]bool{"ID": true, "ProjectID": true, "AllSpecs": true, "TableOfLists": true, "DateLastEdited": true}
	for _, column := range review.ProjectRecords.Columns {
		field, editable := vprocore.ProjectMetadataFields[column.Name]
		if editable == readOnly[column.Name] {
			t.Fatal("metadata source column omitted or identity/stamp enabled:", column.Name)
		}
		if !editable {
			continue
		}
		for _, cell := range []vprocore.ProjectMetadataCell{{Storage: "null"}, vprotest.MetadataText("literal")} {
			if field.Kind == "integer" {
				cell = vprotest.MetadataInteger("1")
			}
			if _, err := vprocore.ValidateProjectMetadataAssignment(column.Name, cell); err != nil {
				t.Fatal(column.Name, err)
			}
		}
	}
}

func TestProjectMetadataPhysicalTextBoundsAndLiteralPreservation(t *testing.T) {
	for column, field := range vprocore.ProjectMetadataFields {
		if field.Kind != "text" {
			continue
		}
		t.Run(column, func(t *testing.T) {
			for _, text := range []string{"", "  Mixed CASE  ", "日本語", "🌲"} {
				value, err := vprocore.ValidateProjectMetadataAssignment(column, vprotest.MetadataText(text))
				if err != nil || value != text {
					t.Fatal("metadata text was normalized:", value, err)
				}
			}
			if _, err := vprocore.ValidateProjectMetadataAssignment(column, vprotest.MetadataText(string([]byte{0xff}))); err == nil {
				t.Fatal("malformed Unicode was repaired")
			}
			if field.Maximum > 0 {
				boundary := strings.Repeat("🌲", field.Maximum/2) + strings.Repeat("x", field.Maximum%2)
				if _, err := vprocore.ValidateProjectMetadataAssignment(column, vprotest.MetadataText(boundary)); err != nil {
					t.Fatal("UTF-16 boundary rejected:", err)
				}
				if _, err := vprocore.ValidateProjectMetadataAssignment(column, vprotest.MetadataText(boundary+"x")); err == nil {
					t.Fatal("UTF-16 overlength accepted")
				}
			} else if _, err := vprocore.ValidateProjectMetadataAssignment(column, vprotest.MetadataText(strings.Repeat("x", 100000))); err != nil {
				t.Fatal("source memo acquired an invented text bound:", err)
			}
		})
	}
}

func TestProjectMetadataIntegersOptionsAndNoImplicitTemplateConversion(t *testing.T) {
	for column, field := range vprocore.ProjectMetadataFields {
		if field.Kind != "integer" {
			continue
		}
		t.Run(column, func(t *testing.T) {
			valid := []string{"-32768", "0", "32767"}
			invalid := []string{"-32769", "32768"}
			if field.Maximum == 32 {
				valid = []string{"-2147483648", "0", "2147483647"}
				invalid = []string{"-2147483649", "2147483648"}
			}
			if field.Options {
				valid, invalid = []string{"1", "2", "3"}, []string{"-1", "0", "4", "32767"}
			}
			for _, number := range valid {
				value, err := vprocore.ValidateProjectMetadataAssignment(column, vprotest.MetadataInteger(number))
				if err != nil || strconv.FormatInt(value.(int64), 10) != number {
					t.Fatal("source integer changed:", number, value, err)
				}
			}
			for _, number := range append(invalid, "1.5", "+1", "01", "-0", " 1", "9007199254740993") {
				if _, err := vprocore.ValidateProjectMetadataAssignment(column, vprotest.MetadataInteger(number)); err == nil {
					t.Fatal("invalid/noncanonical integer accepted:", number)
				}
			}
			if _, err := vprocore.ValidateProjectMetadataAssignment(column, vprotest.MetadataText("2000-01-02 03:04:05")); err == nil {
				t.Fatal("master timestamp/text was implicitly converted to project integer")
			}
			if _, err := vprocore.ValidateProjectMetadataAssignment(column, vprocore.ProjectMetadataCell{Storage: "null"}); err != nil {
				t.Fatal("source nullable integer rejected:", err)
			}
		})
	}
	for _, column := range []string{"GeoRefMethod", "Datum", "CoordinateSystem", "VegCoverMethod", "PlotMethod", "MensurationMethod"} {
		if _, err := vprocore.ValidateProjectMetadataAssignment(column, vprotest.MetadataInteger("7")); err == nil {
			t.Fatal("numeric master method code implicitly stringified:", column)
		}
	}
}

func TestProjectMetadataAssignmentsRejectUnavailableAndConflictingTaggedValues(t *testing.T) {
	for _, column := range []string{"ID", "ProjectID", "AllSpecs", "TableOfLists", "DateLastEdited", "Unknown", "projecttitle"} {
		if _, err := vprocore.ValidateProjectMetadataAssignment(column, vprotest.MetadataText("literal")); err == nil {
			t.Fatal("unavailable/identity/stamp assignment accepted:", column)
		}
	}
	one := 1.0
	text := "literal"
	for _, cell := range []vprocore.ProjectMetadataCell{
		{Storage: "null", Text: &text},
		{Storage: "text"},
		{Storage: "integer"},
		{Storage: "real"},
		{Storage: "text", Text: &text, Real: &one},
		{Storage: "blob", BlobHex: &text},
		{Storage: "boolean"},
	} {
		if _, err := vprocore.ValidateProjectMetadataAssignment("ProjectTitle", cell); err == nil {
			t.Fatal("conflicting/incomplete tagged value accepted:", cell)
		}
	}
}

func TestProjectMetadataDraftsOmitUnchangedHistoricalValuesAndKeepTypedOriginals(t *testing.T) {
	blob := "00ff"
	columns := []vprocore.ProjectMetadataColumn{{Name: "CollectedSite"}, {Name: "ProjectTitle"}, {Name: "Notes"}, {Name: "StartDate"}}
	original := vprocore.ProjectMetadataRow{RowID: "-200", Cells: []vprocore.ProjectMetadataCell{
		vprotest.MetadataInteger("-1"), vprotest.MetadataText(strings.Repeat("x", 300)),
		{Storage: "blob", BlobHex: &blob}, vprotest.MetadataText("2000-01-02 03:04:05"),
	}}
	changes := []vprocore.ProjectMetadataChangeInternal{}
	for i, column := range columns {
		changes = append(changes, vprocore.ProjectMetadataChangeInternal{Column: column.Name, Value: original.Cells[i]})
	}
	assignments, err := vprocore.PrepareProjectMetadataChanges(columns, original, changes)
	if err != nil || len(assignments) != 0 {
		t.Fatal("unchanged historical fields acquired assignments or validation:", assignments, err)
	}
	changes[3].Value = vprotest.MetadataInteger("2000")
	assignments, err = vprocore.PrepareProjectMetadataChanges(columns, original, changes)
	if err != nil || len(assignments) != 1 || assignments[0].Column != "StartDate" ||
		assignments[0].Before.Storage != "text" || *assignments[0].Before.Text != "2000-01-02 03:04:05" ||
		assignments[0].Value != int64(2000) {
		t.Fatal("explicit year assignment lost its typed original or modified unrelated history:", assignments, err)
	}
	changes[0].Value = vprotest.MetadataInteger("0")
	if _, err := vprocore.PrepareProjectMetadataChanges(columns, original, changes); err == nil {
		t.Fatal("new invalid collection option was treated as preserved history")
	}
}

func TestProjectMetadataDraftsRejectMissingRepeatedAndUnavailableProperties(t *testing.T) {
	columns := []vprocore.ProjectMetadataColumn{{Name: "Notes"}, {Name: "ID"}}
	original := vprocore.ProjectMetadataRow{RowID: "-200", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("original"), vprotest.MetadataInteger("-200")}}
	for _, changes := range [][]vprocore.ProjectMetadataChangeInternal{
		nil,
		{{Column: "Notes", Value: vprotest.MetadataText("new")}, {Column: "Notes", Value: vprotest.MetadataText("another")}},
		{{Column: "ID", Value: vprotest.MetadataInteger("-200")}},
		{{Column: "Unknown", Value: vprotest.MetadataText("new")}},
		{{Column: "Notes", Value: vprotest.MetadataText(string([]byte{0xff}))}},
	} {
		if _, err := vprocore.PrepareProjectMetadataChanges(columns, original, changes); err == nil {
			t.Fatal("ambiguous/malformed draft accepted:", changes)
		}
	}
	for _, schema := range [][]vprocore.ProjectMetadataColumn{nil, {{Name: "Notes"}}, {{Name: "Notes"}, {Name: "Notes"}}, {{Name: "Notes"}, {}}} {
		if _, err := vprocore.PrepareProjectMetadataChanges(schema, original, []vprocore.ProjectMetadataChangeInternal{{Column: "Notes", Value: vprotest.MetadataText("new")}}); err == nil {
			t.Fatal("incomplete/ambiguous draft schema accepted:", schema)
		}
	}
}

func TestProjectMetadataReferencesPreserveSourceBindingsAndListPolicies(t *testing.T) {
	references, strict := 0, 0
	for column, field := range vprocore.ProjectMetadataFields {
		if field.ReferenceList == "" {
			continue
		}
		references++
		if strings.HasPrefix(column, "DataQuality") {
			if field.ReferenceList != "PlotQualitySite" || field.ReferenceColumn != "Note" ||
				field.LimitToList != (column != "DataQualitySite") {
				t.Fatal("quality fields must store source Note, not Item or a numeric code:", column, field)
			}
		} else if field.ReferenceColumn != "Item" || field.LimitToList {
			t.Fatal("ordinary reference combo gained strict membership or changed its binding:", column, field)
		}
		if field.LimitToList {
			strict++
		}
	}
	if references != 18 || strict != 8 {
		t.Fatal("source reference batch changed:", references, strict)
	}
	if vprocore.ProjectMetadataFields["GeoRefMethod"].ReferenceList != "GeoreferenceMethod" ||
		vprocore.ProjectMetadataFields["CoordinateSystem"].ReferenceList != "CoordSystem" {
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
		field, editable := vprocore.ProjectMetadataFields[column[1]]
		if !editable {
			continue
		}
		switch {
		case strings.HasPrefix(column[2], "TEXT"):
			maximum, err := strconv.Atoi(column[3])
			if err != nil || field.Kind != "text" || field.Maximum != maximum {
				t.Fatal("source text policy mismatch:", column[1])
			}
		case column[2] == "MEMO":
			if field.Kind != "text" || field.Maximum != 0 {
				t.Fatal("source memo policy mismatch")
			}
		case column[2] == "INTEGER" || column[2] == "LONG":
			bits := 16
			if column[2] == "LONG" {
				bits = 32
			}
			if field.Kind != "integer" || field.Maximum != bits {
				t.Fatal("source integer policy mismatch:", column[1])
			}
		default:
			t.Fatal("unsupported editable source type:", column)
		}
	}
}
