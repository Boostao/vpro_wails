package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTwoPageGoogleEarthProductionWireFixture(t *testing.T) {
	service, state, db := twoPageGoogleEarthFixture(t, false)
	if _, err := db.Exec(`ALTER TABLE Sample_Env RENAME TO Archived_Env;
		CREATE TABLE Sample_Env(PlotNumber TEXT,Longitude REAL,Latitude REAL,PlotRepresenting TEXT,Unmapped BLOB);
		INSERT INTO Sample_Env VALUES('108050',123.25,53.5,'Saved café & <b>raw</b>',X'00FF')`); err != nil {
		t.Fatal(err)
	}
	options, err := service.GetOptions(context.Background(), state.ContextID, "108050", "FS882-8x6XL")
	if err != nil {
		t.Fatal(err)
	}
	request, err := json.Marshal(GoogleEarthKMLRequest{DescriptionField: "PlotRepresenting", Title: "Fixture & café"})
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", string(request))
	if err != nil {
		t.Fatal(err)
	}
	options.ContextID, review.ContextID = "wire-owned", "wire-owned"
	options.ProjectPath, review.ProjectPath = `C:\fixture\Sample.db`, `C:\fixture\Sample.db`
	cases := []*TwoPageGoogleEarthReview{}
	for _, description := range []string{"\nSaved text", "Saved\r\ntext", "Saved\rtext", "\r\nSaved\r\ntext", `"quoted" & 'literal' <b>café</b>`} {
		if _, err := db.Exec(`UPDATE Sample_Env SET PlotRepresenting=? WHERE PlotNumber='108050'`, description); err != nil {
			t.Fatal(err)
		}
		item, err := service.GetReview(context.Background(), state.ContextID, "108050", "FS882-8x6XL", string(request))
		if err != nil {
			t.Fatal(err)
		}
		item.ContextID, item.ProjectPath = "wire-owned", `C:\fixture\Sample.db`
		cases = append(cases, item)
	}
	wire := struct {
		Options          *TwoPageGoogleEarthOptions  `json:"options"`
		Review           *TwoPageGoogleEarthReview   `json:"review"`
		DescriptionCases []*TwoPageGoogleEarthReview `json:"descriptionCases"`
	}{options, review, cases}
	encoded, err := json.MarshalIndent(wire, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("frontend", "src", "twoPageGoogleEarthWire.json")
	if os.Getenv("VPRO_UPDATE_TWO_PAGE_GOOGLE_EARTH_WIRE") == "true" {
		if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fixture, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var expected, actual any
	if err := json.Unmarshal(fixture, &expected); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &actual); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("actual Go source KML wire differs from shared fixture:\n%s", encoded)
	}
}
