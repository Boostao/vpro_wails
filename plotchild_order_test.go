package main

import (
	"reflect"
	"testing"
)

func TestSoilChildSourceDepthOrderingAndNoReadMutations(t *testing.T) {
	for _, kind := range []string{"Humus", "Mineral"} {
		t.Run(kind, func(t *testing.T) {
			s, db := childFixture(t)
			for _, entry := range []struct {
				id    int
				depth any
			}{{7, 0.0}, {3, -10.25}, {9, nil}, {2, -10.25}, {4, 1.125}} {
				if _, err := db.Exec(`INSERT INTO "Sample_`+kind+`" (PlotNumber,ID,UpperDepth) VALUES ('CHILD1',?,?)`,
					entry.id, entry.depth); err != nil {
					t.Fatal(err)
				}
			}
			before := substrateSnapshot(t, db)
			ids := loadedSoilChildIDs(t, s, kind)
			want := []int64{4, 7, 2, 3, 9}
			if kind == "Mineral" {
				want = []int64{9, 2, 3, 7, 4}
			}
			if !reflect.DeepEqual(ids, want) {
				t.Fatalf("source UpperDepth order = %v, want %v", ids, want)
			}
			assertSubstrateSnapshot(t, db, before)
			if _, err := db.Exec(`ALTER TABLE "Sample_` + kind + `" DROP COLUMN UpperDepth`); err != nil {
				t.Fatal(err)
			}
			caps, err := s.GetChildCapabilities(kind)
			if err != nil || caps["upperDepth"] {
				t.Fatalf("missing source depth was advertised: %v %v", caps, err)
			}
			loaded := loadedSoilChildIDs(t, s, kind)
			if !reflect.DeepEqual(loaded, []int64{2, 3, 4, 7, 9}) {
				t.Fatalf("unavailable depth must retain deterministic ID order: %v", loaded)
			}
		})
	}
}

func loadedSoilChildIDs(t *testing.T, s *PlotService, kind string) []int64 {
	t.Helper()
	var ids []int64
	if kind == "Humus" {
		rows, err := s.ListHumusRecords("CHILD1")
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
	} else {
		rows, err := s.ListMineralRecords("CHILD1")
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
	}
	return ids
}
