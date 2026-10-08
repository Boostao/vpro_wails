package main

import (
	"context"
	"encoding/xml"
	"errors"
	"html"
	"math"
	"reflect"
	"strings"
	"testing"
)

func kmlTestText(value string) ProjectMetadataCell {
	return ProjectMetadataCell{Storage: "text", Text: &value}
}

func kmlTestReal(value float64) ProjectMetadataCell {
	return ProjectMetadataCell{Storage: "real", Real: &value}
}

func kmlTestInteger(value string) ProjectMetadataCell {
	return ProjectMetadataCell{Storage: "integer", Integer: &value}
}

func kmlTestRows() []googleEarthLocation {
	return []googleEarthLocation{
		{EnvRowID: "-1", MembershipRowID: "2", PlotNumber: kmlTestText(" A<& "),
			StoredLongitude: kmlTestReal(123.25), Longitude: kmlTestReal(-123.25),
			Latitude: kmlTestReal(54.5), Description: ProjectMetadataCell{Storage: "null"}},
		{EnvRowID: "-1", MembershipRowID: "3", PlotNumber: kmlTestText(" A<& "),
			Longitude: kmlTestReal(-123.25), Latitude: kmlTestReal(54.5), Description: ProjectMetadataCell{Storage: "null"}},
		{EnvRowID: "7", PlotNumber: ProjectMetadataCell{Storage: "null"},
			Longitude: kmlTestReal(math.Copysign(0, -1)), Latitude: kmlTestInteger("-90"),
			Description: kmlTestText(" \r\n]]><img src=\"x\">&amp;\U0001f332 ")},
		{EnvRowID: "-2", MembershipRowID: "4", PlotNumber: kmlTestText(" A<& "),
			Longitude: kmlTestReal(-123.25), Latitude: kmlTestReal(54.5), Description: kmlTestText("")},
	}
}

func TestGoogleEarthKMLPointsTextNullFanoutAndExactXML(t *testing.T) {
	rows := kmlTestRows()
	points, err := planGoogleEarthKMLPoints(context.Background(), rows)
	if err != nil || len(points) != 4 || points[0].Name != " A<& " || points[1].Name != points[0].Name ||
		points[0].Description != "" || points[1].Description != "" || points[3].Description != "" || points[2].Name != "" ||
		points[0].Longitude != -123.25 || !math.Signbit(points[2].Longitude) || points[2].Latitude != -90 {
		t.Fatal("literal text/NULL, order/fanout or coordinate direction lost", points, err)
	}
	if rows[0].Description.Storage != "null" || rows[3].Description.Storage != "text" ||
		*rows[0].StoredLongitude.Real != 123.25 {
		t.Fatal("distinct raw metadata/source coordinates changed")
	}
	data, err := prepareGoogleEarthKML(context.Background(), "explicit title", points)
	var observed observedGoogleEarthKML
	if err != nil || xml.Unmarshal(data, &observed) != nil || len(observed.Document.Marks) != 4 {
		t.Fatal("complete adapter-to-XML output missing", err)
	}
	for i, mark := range observed.Document.Marks {
		if mark.Name != points[i].Name || html.UnescapeString(mark.Description) != points[i].Description+"." {
			t.Fatal("source NULL period or literal text differs", i, mark)
		}
	}
	*rows[0].PlotNumber.Text = "changed"
	*rows[2].Description.Text = "changed"
	if points[0].Name != " A<& " || points[2].Description == "changed" {
		t.Fatal("finalized points alias raw metadata")
	}
}

func TestGoogleEarthKMLPointsRejectsUnavailableAndMalformedCells(t *testing.T) {
	blob := "00ff"
	invalid := []ProjectMetadataCell{
		kmlTestInteger("7"), kmlTestReal(1.5), {Storage: "blob", BlobHex: &blob},
		{Storage: "text"}, {Storage: "null", Text: kmlTestText("phantom").Text},
		kmlTestText("\x00"), kmlTestText(string([]byte{0xff})), kmlTestInteger("01"),
	}
	for _, cell := range invalid {
		for _, field := range []string{"PlotNumber", "description"} {
			rows := kmlTestRows()
			if field == "PlotNumber" {
				rows[1].PlotNumber = cell
			} else {
				rows[1].Description = cell
			}
			points, err := planGoogleEarthKMLPoints(context.Background(), rows)
			if err == nil || points != nil || !strings.Contains(err.Error(), `Env row "-1" membership "3" `+field) {
				t.Fatal("unsupported/malformed text produced partial points or lost physical error context", field, cell, err)
			}
		}
	}
	for _, cell := range []ProjectMetadataCell{
		{Storage: "null"}, kmlTestText("123"), {Storage: "blob", BlobHex: &blob},
		kmlTestInteger("9223372036854775807"), kmlTestInteger("-9223372036854775808"),
		kmlTestReal(math.NaN()), kmlTestReal(math.Inf(1)), kmlTestReal(math.Inf(-1)),
		kmlTestReal(math.Nextafter(180, 181)), kmlTestReal(math.Nextafter(-180, -181)),
	} {
		rows := kmlTestRows()
		rows[1].Longitude = cell
		if points, err := planGoogleEarthKMLPoints(context.Background(), rows); err == nil || points != nil {
			t.Fatal("invalid longitude produced rounded/clamped/partial success", cell, err)
		}
	}
	for _, value := range []float64{math.Nextafter(90, 91), math.Nextafter(-90, -91)} {
		rows := kmlTestRows()
		rows[1].Latitude = kmlTestReal(value)
		if points, err := planGoogleEarthKMLPoints(context.Background(), rows); err == nil || points != nil {
			t.Fatal("invalid latitude produced partial success", value, err)
		}
	}
	rows := kmlTestRows()
	rows[0].Longitude, rows[0].Latitude = kmlTestInteger("-180"), kmlTestInteger("90")
	points, err := planGoogleEarthKMLPoints(context.Background(), rows)
	if err != nil || points[0].Longitude != -180 || points[0].Latitude != 90 {
		t.Fatal("exact integer bounds lost", err)
	}
}

func TestGoogleEarthKMLPointsSnapshotsBeforeCallbacksAndCancellation(t *testing.T) {
	rows := kmlTestRows()
	expected, err := planGoogleEarthKMLPoints(context.Background(), rows)
	if err != nil {
		t.Fatal(err)
	}
	ctx := &googleEarthKMLCallbackContext{Context: context.Background(), onCheck: func(int) {
		*rows[0].PlotNumber.Text = "\x00"
		*rows[1].Longitude.Real = math.NaN()
		*rows[2].Description.Text = "\x00"
		rows[2].EnvRowID = "changed"
	}}
	points, err := planGoogleEarthKMLPoints(ctx, rows)
	if err != nil || !reflect.DeepEqual(points, expected) || !math.Signbit(points[2].Longitude) {
		t.Fatal("callback changed detached cells/rows", points, err)
	}
	for _, when := range []int{1, 2, 3, 4, 5, 6} {
		ctx := &googleEarthKMLCallbackContext{Context: context.Background(), cancelAt: when}
		if points, err := planGoogleEarthKMLPoints(ctx, kmlTestRows()); !errors.Is(err, context.Canceled) || points != nil {
			t.Fatal("cancellation returned partial points", when, err)
		}
	}

	if points, err := planGoogleEarthKMLPoints(nil, kmlTestRows()); err == nil || points != nil {
		t.Fatal("nil context replaced silently")
	}
	points, err = planGoogleEarthKMLPoints(context.Background(), nil)
	if err != nil || points == nil || len(points) != 0 {
		t.Fatal("explicit empty dataset differs", points, err)
	}
	points, err = planGoogleEarthKMLPoints(context.Background(), kmlTestRows())
	if err != nil || !reflect.DeepEqual(points, expected) {
		t.Fatal("clean retry differs", points, err)
	}
}

func TestGoogleEarthKMLPointsFromDirectEnvAndSelectedSUProjection(t *testing.T) {
	env, _, su := locationTestTables()
	project, err := planGoogleEarthLocations(context.Background(), "Project", "None", "Zone", env, ProjectMetadataTable{})
	if err != nil {
		t.Fatal(err)
	}
	points, err := planGoogleEarthKMLPoints(context.Background(), project.Rows)
	if err != nil || len(points) != 3 || points[2].Name != "NOADMIN" ||
		points[1].Description != "" || project.Rows[1].Description.Storage != "null" ||
		points[1].Longitude != 123.75 || !math.Signbit(points[1].Latitude) {
		t.Fatal("direct Env orphan/NULL text/coordinate direction lost across adapter", points, err)
	}
	env.Rows = append(env.Rows, ProjectMetadataRow{RowID: "9007199254740993", Cells: append([]ProjectMetadataCell(nil), env.Rows[0].Cells...)})
	selected, err := planGoogleEarthLocations(context.Background(), "Project", "Subset", "Zone", env, su)
	if err != nil {
		t.Fatal(err)
	}
	points, err = planGoogleEarthKMLPoints(context.Background(), selected.Rows)
	if err != nil || len(points) != 6 {
		t.Fatal("physical Env/SU fanout changed across adapter", points, err)
	}
	for _, point := range points {
		if point.Name != "000001" || point.Description != "  Zone  " || point.Longitude != -123.75 || point.Latitude != 49.25 {
			t.Fatal("source-selected literal cell or second longitude negation", point)
		}
	}
	data, err := prepareGoogleEarthKML(context.Background(), "literal title", points)
	var observed observedGoogleEarthKML
	if err != nil || xml.Unmarshal(data, &observed) != nil || len(observed.Document.Marks) != 6 {
		t.Fatal("source projection-to-KML complete shape/fanout missing", err)
	}
}
