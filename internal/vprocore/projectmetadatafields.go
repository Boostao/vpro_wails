package vprocore

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

type projectMetadataField struct {
	Kind            string `json:"-" yaml:"-"`
	Maximum         int    `json:"-" yaml:"-"`
	Options         bool   `json:"-" yaml:"-"`
	ReferenceList   string `json:"-" yaml:"-"`
	ReferenceColumn string `json:"-" yaml:"-"`
	LimitToList     bool   `json:"-" yaml:"-"`
}

type ProjectMetadataChangeInternal struct {
	Column string              `json:"-" yaml:"-"`
	Value  ProjectMetadataCell `json:"-" yaml:"-"`
}

type projectMetadataAssignment struct {
	Column string              `json:"-" yaml:"-"`
	Before ProjectMetadataCell `json:"-" yaml:"-"`
	after  ProjectMetadataCell
	Value  any `json:"-" yaml:"-"`
}

var ProjectMetadataFields = func() map[string]projectMetadataField {
	fields := map[string]projectMetadataField{}
	for _, name := range strings.Fields(`ProjectTitle CoordinatingAgency ProponentFunder FieldCompanyAgency
		FieldLeader FieldDataCollectionTeam ProjectPurpose GeographicStudyArea GeographicStudyRegion
		ProjectType ProjectTypeOther EcosysCollectionStandard VegCoverMethod PlotMethod PlotMethodOther
		MensurationMethod DataQualitySite DataQualityVeg DataQualitySoil DataQualityTerrain DataQualityMens
		DataQualityCWD DataQualityWildTree DataQualitySoilChem DataQualityWildlifeHabitatAssessment
		GeoRefMethod GeoRefMethodOther Datum DatumOther CoordinateSystem CoordinateSystemOther
		CoverA1Description CoverA2Description CoverA3Description CoverADescription CoverB1Description
		CoverB2Description CoverB2aDescription CoverB2bDescription CoverB2cDescription CoverBDescription
		CoverCDescription CoverDDescription Cover8Description Cover9Description Cover10Description`) {
		fields[name] = projectMetadataField{Kind: "text", Maximum: 255}
	}
	for _, name := range strings.Fields(`EcosysCollectionStandardOther VegCoverMethodOther MensurationMethodOther
		CollectedCompleteOther CollectedPartialOther CollectedNoneOther`) {
		fields[name] = projectMetadataField{Kind: "text", Maximum: 50}
	}
	for _, name := range strings.Fields(`ExtraVegFieldDescription DataCustodian StorageLocation`) {
		fields[name] = projectMetadataField{Kind: "text", Maximum: 100}
	}
	fields["Notes"] = projectMetadataField{Kind: "text"}
	for _, name := range strings.Fields(`StartDate EndDate NumberOfSiteVisits`) {
		fields[name] = projectMetadataField{Kind: "integer", Maximum: 16}
	}
	for _, name := range strings.Fields(`NumberOfFS882Plots BAPID`) {
		fields[name] = projectMetadataField{Kind: "integer", Maximum: 32}
	}
	for _, name := range strings.Fields(`CollectedSite CollectedVeg CollectedSoil CollectedTerrain CollectedMens
		CollectedCWD CollectedWildTree CollectedSoilChem CollectedWildlifeHabitatAssessment`) {
		fields[name] = projectMetadataField{Kind: "integer", Maximum: 16, Options: true}
	}
	for name, list := range map[string]string{
		"GeographicStudyRegion": "Region", "GeoRefMethod": "GeoreferenceMethod",
		"CoordinateSystem": "CoordSystem", "EcosysCollectionStandard": "EcosysCollectionStandard",
		"Datum": "Datum", "VegCoverMethod": "VegCoverMethod", "PlotMethod": "PlotMethod",
		"MensurationMethod": "MensurationMethod", "ProjectType": "ProjectType",
	} {
		field := fields[name]
		field.ReferenceList, field.ReferenceColumn = list, "Item"
		fields[name] = field
	}
	for _, name := range strings.Fields(`DataQualitySite DataQualityVeg DataQualitySoil DataQualityTerrain
		DataQualityMens DataQualityCWD DataQualityWildTree DataQualitySoilChem DataQualityWildlifeHabitatAssessment`) {
		field := fields[name]
		field.ReferenceList, field.ReferenceColumn = "PlotQualitySite", "Note"
		field.LimitToList = name != "DataQualitySite"
		fields[name] = field
	}
	return fields
}()

func MetadataCellEqual(left, right ProjectMetadataCell) bool {
	return reflect.DeepEqual(left, right) && (left.Real == nil ||
		math.Float64bits(*left.Real) == math.Float64bits(*right.Real))
}

func MetadataCellValue(cell ProjectMetadataCell) (any, error) {
	var value any
	switch cell.Storage {
	case "null":
	case "text":
		if cell.Text == nil {
			return nil, errors.New("metadata text requires an explicit text value")
		}
		value = []byte(*cell.Text)
	case "integer":
		if cell.Integer == nil {
			return nil, errors.New("metadata integer requires an exact decimal value")
		}
		number, err := strconv.ParseInt(*cell.Integer, 10, 64)
		if err != nil {
			return nil, errors.New("metadata integer requires an exact signed64 decimal value")
		}
		value = number
	case "real":
		if cell.Real == nil {
			return nil, errors.New("metadata real requires an explicit finite value")
		}
		value = *cell.Real
	case "blob":
		if cell.BlobHex == nil {
			return nil, errors.New("metadata blob requires its exact hex bytes")
		}
		raw, err := hex.DecodeString(*cell.BlobHex)
		if err != nil {
			return nil, errors.New("metadata blob requires exact hex bytes")
		}
		value = raw
	default:
		return nil, fmt.Errorf("metadata assignment storage %q is unavailable", cell.Storage)
	}
	normalized, err := ProjectMetadataCellInternal(cell.Storage, value)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(normalized, cell) {
		return nil, errors.New("metadata cell must contain only its canonical tagged value")
	}
	if text, ok := value.([]byte); ok && cell.Storage == "text" {
		return string(text), nil
	}
	return value, nil
}

func PrepareProjectMetadataChanges(columns []ProjectMetadataColumn, original ProjectMetadataRow, changes []ProjectMetadataChangeInternal) ([]projectMetadataAssignment, error) {
	if len(columns) == 0 || len(columns) != len(original.Cells) || len(changes) == 0 {
		return nil, errors.New("metadata drafts require a complete original row and explicit changes")
	}
	index := map[string]int{}
	for i, column := range columns {
		if _, present := index[column.Name]; column.Name == "" || present {
			return nil, errors.New("metadata draft schema has missing or ambiguous column identities")
		}
		index[column.Name] = i
	}
	assignments := []projectMetadataAssignment{}
	seen := map[string]bool{}
	for _, change := range changes {
		i, present := index[change.Column]
		if _, editable := ProjectMetadataFields[change.Column]; !present || !editable || seen[change.Column] {
			return nil, fmt.Errorf("metadata draft property %q is unavailable, repeated or caller-owned identity/stamp", change.Column)
		}
		seen[change.Column] = true
		if _, err := MetadataCellValue(change.Value); err != nil {
			return nil, fmt.Errorf("metadata.%s: %w", change.Column, err)
		}
		if reflect.DeepEqual(original.Cells[i], change.Value) {
			continue
		}
		value, err := ValidateProjectMetadataAssignment(change.Column, change.Value)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, projectMetadataAssignment{change.Column, original.Cells[i], change.Value, value})
	}
	return assignments, nil
}

func ValidateProjectMetadataAssignment(column string, cell ProjectMetadataCell) (any, error) {
	field, available := ProjectMetadataFields[column]
	if !available {
		return nil, fmt.Errorf("metadata property %q is unavailable; identities and stamps are not caller-assigned", column)
	}
	value, err := MetadataCellValue(cell)
	if err != nil {
		return nil, fmt.Errorf("metadata.%s: %w", column, err)
	}
	if value == nil {
		return nil, nil
	}
	if cell.Storage != field.Kind {
		return nil, fmt.Errorf("metadata.%s requires nullable %s storage; no date or code conversion is implicit", column, field.Kind)
	}
	if field.Kind == "text" {
		return value, ValidateChildPhysicalText("Metadata."+column, value, field.Maximum)
	}
	number := value.(int64)
	if field.Maximum == 16 && (number < -32768 || number > 32767) ||
		field.Maximum == 32 && (number < -2147483648 || number > 2147483647) {
		return nil, fmt.Errorf("metadata.%s exceeds the source signed%d integer domain", column, field.Maximum)
	}
	if field.Options && (number < 1 || number > 3) {
		return nil, fmt.Errorf("metadata.%s requires a source collection option (1 complete, 2 partial, 3 none), not a BOOLEAN", column)
	}
	return value, nil
}
