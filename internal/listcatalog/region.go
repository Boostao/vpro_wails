package listcatalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func RegionSnapshotRows(data []byte, p Provenance) ([]Choice, error) {
	if err := ValidateProvenanceFor(p, RegionProfile()); err != nil {
		return nil, err
	}
	if checksum(data) != p.SnapshotSHA256 {
		return nil, errors.New("region snapshot checksum mismatch")
	}
	var snapshot struct {
		Kind   string `json:"snapshotKind"`
		Source struct {
			Before         string `json:"BeforeSHA256"`
			CanonicalAfter string `json:"CanonicalAfterSHA256"`
			BaselineAfter  string `json:"BaselineAfterSHA256"`
			Restored       string `json:"RestoredOwnedSHA256"`
		} `json:"source"`
		Schema struct {
			Name            string
			SourceTableName string
			Fields          []struct {
				Name string
				Type int
				Size int
			}
		} `json:"schema"`
		ReadOnly    bool   `json:"sourceReadOnly"`
		SourceTable string `json:"sourceTable"`
		Rows        int    `json:"rowCount"`
		Cells       int    `json:"cellCount"`
		Lists       map[string]struct {
			SQL       string               `json:"sourceSQL"`
			Filter    string               `json:"sourceFilterCase"`
			Ordered   bool                 `json:"hasOrderBy"`
			Rows      int                  `json:"rowCount"`
			Cells     int                  `json:"cellCount"`
			Unchanged struct{ Equal bool } `json:"readonlyReferencePreservation"`
			Values    []struct {
				Ordinal string `json:"sourceOrdinal"`
				Cells   []Cell `json:"cells"`
			} `json:"rowsInObservedNativeOrder"`
		} `json:"lists"`
	}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}
	if snapshot.Kind != "Readonly native DAO Region/Ecosection fixture, not Access-reader implementation" ||
		!snapshot.ReadOnly || snapshot.Rows != p.Rows || snapshot.Cells != p.Cells ||
		snapshot.SourceTable != p.SourceTable || snapshot.Schema.Name != p.SourceTable ||
		snapshot.Schema.SourceTableName != p.SourceTable || len(snapshot.Schema.Fields) != 10 || len(snapshot.Lists) != 2 {
		return nil, errors.New("region native snapshot provenance mismatch")
	}
	for _, hash := range []string{snapshot.Source.Before, snapshot.Source.CanonicalAfter, snapshot.Source.BaselineAfter, snapshot.Source.Restored} {
		if strings.ToLower(hash) != p.SourceSHA256 {
			return nil, errors.New("region native source preservation mismatch")
		}
	}
	for i, field := range snapshot.Schema.Fields {
		want := p.SourceSchema[i]
		if field.Name != want.Name || field.Type != want.DAOType || field.Size != want.Size {
			return nil, errors.New("region native schema mismatch")
		}
	}
	result := make([]Choice, 0, p.Rows)
	for _, list := range RegionProfile().Lists {
		source, ok := snapshot.Lists[list.Name]
		filter := list.Name
		sql := `SELECT * FROM USysTableOfLists WHERE ListName="Region" ORDER BY ItemOrder`
		if list.Name == "Ecosection" {
			filter = "ecosection"
			sql = `SELECT * FROM USysTableOfLists WHERE ListName="ecosection"`
		}
		if !ok || !source.Unchanged.Equal || source.Rows != list.Rows || source.Cells != list.Rows*10 ||
			len(source.Values) != list.Rows || source.Filter != filter || source.SQL != sql || source.Ordered != (list.Name == "Region") {
			return nil, fmt.Errorf("region list %s query/order/count mismatch", list.Name)
		}
		for i, value := range source.Values {
			if value.Ordinal != strconv.Itoa(i+1) {
				return nil, fmt.Errorf("region list %s ordinal mismatch", list.Name)
			}
			row, err := DecodeCells(value.Ordinal, value.Cells)
			if err != nil {
				return nil, err
			}
			if row.ListName == nil || *row.ListName != list.Name {
				return nil, fmt.Errorf("region list %s membership mismatch", list.Name)
			}
			result = append(result, row)
		}
	}
	return result, nil
}
