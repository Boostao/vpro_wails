package listcatalog

import "errors"

func GeologySnapshotRows(data []byte, p Provenance) ([]Choice, error) {
	if err := ValidateProvenanceFor(p, GeologyProfile()); err != nil {
		return nil, err
	}
	if checksum(data) != p.SnapshotSHA256 {
		return nil, errors.New("geology DAO snapshot checksum mismatch")
	}
	return decodeGeologySnapshot(data, p)
}

func decodeGeologySnapshot(data []byte, p Provenance) ([]Choice, error) {
	var snapshot struct {
		Kind         string `json:"snapshotKind"`
		SourceSHA256 string `json:"sourceByteSHA256"`
		DAO          struct {
			Readonly      bool `json:"Readonly"`
			NoApplication bool `json:"NoAccessApplication"`
			GUIStarted    bool `json:"NativeGUIStarted"`
			HandlesClosed bool `json:"DAOHandlesClosed"`
			Schema        daoSnapshotSchema
			Catalogues    []daoSnapshotCatalogue
		} `json:"dao"`
		Reference struct {
			List      string   `json:"queryLiteral"`
			SQL       string   `json:"SQL"`
			Rows      int      `json:"rowCount"`
			Cells     int      `json:"cellCount"`
			RawSHA256 string   `json:"rawRowsSHA256"`
			SHA256    string   `json:"typedCellsSHA256"`
			RowHashes []string `json:"typedRowSHA256"`
			Choices   []Choice `json:"choices"`
		} `json:"reference"`
	}
	if err := readDAOGzip(data, &snapshot); err != nil {
		return nil, err
	}
	if snapshot.Kind != "Current readonly DAO BedrockType catalogue only; not form/native/write evidence" ||
		snapshot.SourceSHA256 != p.SourceSHA256 || !snapshot.DAO.Readonly || !snapshot.DAO.NoApplication ||
		snapshot.DAO.GUIStarted || !snapshot.DAO.HandlesClosed || len(snapshot.DAO.Catalogues) != 1 ||
		snapshot.Reference.List != "BedrockType" || snapshot.Reference.SQL != p.SQL ||
		snapshot.Reference.Rows != p.Rows || snapshot.Reference.Cells != p.Cells ||
		snapshot.Reference.SHA256 != GeologyNativeTypedCellsSHA256 || len(snapshot.Reference.RowHashes) != p.Rows {
		return nil, errors.New("geology current DAO source/schema/provenance mismatch")
	}
	if err := validateDAOSchema(snapshot.DAO.Schema, p); err != nil {
		return nil, err
	}
	catalogue := snapshot.DAO.Catalogues[0]
	if catalogue.SHA256 != snapshot.Reference.RawSHA256 {
		return nil, errors.New("geology DAO raw-row reference checksum mismatch")
	}
	rows, native, err := decodeDAOCatalogue(catalogue, snapshot.Reference.Choices, GeologyProfile().Lists[0])
	if err != nil {
		return nil, err
	}
	for i, cells := range native {
		hash, err := daoNativeHash(cells)
		if err != nil || hash != snapshot.Reference.RowHashes[i] {
			return nil, errors.New("geology DAO native row checksum mismatch")
		}
	}
	hash, err := daoNativeHash(native)
	if err != nil {
		return nil, err
	}
	if hash != GeologyNativeTypedCellsSHA256 {
		return nil, errors.New("geology870 native-cell checksum mismatch")
	}
	return rows, nil
}
