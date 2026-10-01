package listcatalog

import (
	"errors"
	"fmt"
)

func SoilSnapshotRows(data []byte, p Provenance) ([]Choice, error) {
	if err := ValidateProvenanceFor(p, SoilProfile()); err != nil {
		return nil, err
	}
	if checksum(data) != p.SnapshotSHA256 {
		return nil, errors.New("soil DAO snapshot checksum mismatch")
	}
	return decodeSoilSnapshot(data, p)
}

func decodeSoilSnapshot(data []byte, p Provenance) ([]Choice, error) {
	var snapshot struct {
		Kind   string `json:"snapshotKind"`
		Source struct {
			SHA256   string `json:"sha256"`
			Bytes    int    `json:"logicalBytes"`
			Readonly bool   `json:"sourceExclusiveReadLock"`
			Equal    bool   `json:"payloadUnchanged"`
		} `json:"source"`
		DAO struct {
			Readonly      bool `json:"Readonly"`
			NoApplication bool `json:"NoAccessApplication"`
			GUIStarted    bool `json:"NativeGUIStarted"`
			HandlesClosed bool `json:"DAOHandlesClosed"`
			Schema        daoSnapshotSchema
			Catalogues    []daoSnapshotCatalogue
		} `json:"dao"`
		Reference struct {
			Rows   int    `json:"rowCount"`
			Cells  int    `json:"cellCount"`
			SHA256 string `json:"typedCellsSHA256"`
			Lists  []struct {
				Name    string   `json:"queryLiteral"`
				Rows    int      `json:"rowCount"`
				Cells   int      `json:"cellCount"`
				Choices []Choice `json:"choices"`
			} `json:"lists"`
		} `json:"reference"`
	}
	if err := readDAOGzip(data, &snapshot); err != nil {
		return nil, err
	}
	if snapshot.Kind != "Actual readonly DAO.DBEngine.120 owned Soil catalogue; not form/nativeUI proof" ||
		snapshot.Source.SHA256 != p.SourceSHA256 || snapshot.Source.Bytes != 8818688 ||
		!snapshot.Source.Readonly || !snapshot.Source.Equal ||
		!snapshot.DAO.Readonly || !snapshot.DAO.NoApplication || snapshot.DAO.GUIStarted || !snapshot.DAO.HandlesClosed ||
		len(snapshot.DAO.Catalogues) != 2 || snapshot.Reference.Rows != p.Rows || snapshot.Reference.Cells != p.Cells ||
		snapshot.Reference.SHA256 != SoilNativeTypedCellsSHA256 || len(snapshot.Reference.Lists) != 2 {
		return nil, errors.New("soil current DAO source/schema/provenance mismatch")
	}
	if err := validateDAOSchema(snapshot.DAO.Schema, p); err != nil {
		return nil, err
	}
	rows := make([]Choice, 0, p.Rows)
	allCells := make([][]daoNativeCell, 0, p.Rows)
	for index, list := range SoilProfile().Lists {
		reference := snapshot.Reference.Lists[index]
		if reference.Name != list.Name || reference.Rows != list.Rows || reference.Cells != list.Rows*10 {
			return nil, fmt.Errorf("soil %s query/casing/count mismatch", list.Name)
		}
		converted, native, err := decodeDAOCatalogue(snapshot.DAO.Catalogues[index], reference.Choices, list)
		if err != nil {
			return nil, err
		}
		rows = append(rows, converted...)
		allCells = append(allCells, native...)
	}
	hash, err := daoNativeHash(allCells)
	if err != nil {
		return nil, err
	}
	if hash != SoilNativeTypedCellsSHA256 {
		return nil, errors.New("soil1010 native-cell checksum mismatch")
	}
	return rows, nil
}
