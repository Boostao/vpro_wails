package main

import (
	"reflect"
	"testing"
)

func TestPlotCoordinate_DerivedSavePreservesUntouchedAxisAndAuditsOnlyChangedField(t *testing.T) {
	double := func(value float64) *float64 { return &value }
	for _, test := range []struct {
		name      string
		latitude  *float64
		longitude *float64
		axis      string
		mode      CoordinateMode
		parts     CoordinateParts
		want      float64
		field     string
	}{
		{"signed-dms", double(49.123456789), double(123.987654321), "latitude", CoordinateModeDMS,
			CoordinateParts{Degrees: double(49), Minutes: double(30), Seconds: double(0), Negative: true}, -49.5, "Latitude"},
		{"untouched-null", nil, double(-123.123456789), "longitude", CoordinateModeDM,
			CoordinateParts{Degrees: double(123), Minutes: double(30), Negative: true}, -123.5, "Longitude"},
		{"negative-subdegree", double(-0.5), double(123.987654321), "latitude", CoordinateModeDMS,
			CoordinateParts{Degrees: double(0), Minutes: double(45), Seconds: double(0), Negative: true}, -0.75, "Latitude"},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, db := headerFixture(t)
			service.SetAuditStrength(0)
			original := FS882Header{PlotNumber: "COORDSAVE", Latitude: test.latitude, Longitude: test.longitude}
			if err := service.CreatePlot(original); err != nil {
				t.Fatal(err)
			}
			service.SetAuditStrength(1)
			value, err := ConvertCoordinate(test.axis, test.mode, test.parts)
			if err != nil || value == nil || *value != test.want {
				t.Fatalf("conversion: %v, %v", value, err)
			}
			draft := original
			if test.axis == "latitude" {
				draft.Latitude = value
			} else {
				draft.Longitude = value
			}
			if err := service.UpdatePlot(draft); err != nil {
				t.Fatal(err)
			}
			got, err := service.GetPlot(original.PlotNumber)
			if err != nil || got == nil || !reflect.DeepEqual(*got, draft) {
				t.Fatalf("stored coordinates/partner precision: %#v, %v", got, err)
			}
			if count := auditCount(t, db, original.PlotNumber); count != 1 {
				t.Fatalf("coordinate edit added %d audits, want exactly one", count)
			}
			var field string
			if err := db.QueryRow(`SELECT EditField FROM Sample_Audit WHERE PlotNumber=?`, original.PlotNumber).Scan(&field); err != nil {
				t.Fatal(err)
			}
			if field != test.field {
				t.Fatalf("untouched or computed field audited: %s", field)
			}
		})
	}
}

func TestPlotCoordinate_HeaderFailureRollsBackCoordinateAndAudit(t *testing.T) {
	service, db := headerFixture(t)
	service.SetAuditStrength(0)
	lat, lon := 49.123456789, -123.987654321
	original := FS882Header{PlotNumber: "COORDROLLBACK", Latitude: &lat, Longitude: &lon}
	if err := service.CreatePlot(original); err != nil {
		t.Fatal(err)
	}
	service.SetAuditStrength(2)
	if _, err := db.Exec(`CREATE TRIGGER fail_coordinate_admin BEFORE UPDATE ON Sample_Admin
		WHEN NEW.Plot='COORDROLLBACK' BEGIN SELECT RAISE(ABORT,'coordinate rollback'); END`); err != nil {
		t.Fatal(err)
	}
	next := -49.5
	draft := original
	draft.Latitude = &next
	if err := service.UpdatePlot(draft); err == nil {
		t.Fatal("failed Admin update authorized coordinate save")
	}
	got, err := service.GetPlot(original.PlotNumber)
	if err != nil || got == nil || !reflect.DeepEqual(*got, original) || auditCount(t, db, original.PlotNumber) != 0 {
		t.Fatalf("coordinate or audit partially committed: %#v, %v", got, err)
	}
}
