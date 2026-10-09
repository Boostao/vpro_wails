package listcatalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const ParentManifestSHA256 = "8292d8bc7ba83d2bad248bf0b5371a2d1b41e386efd5824a2ae4c237223e6c20"
const ParentSnapshotSHA256 = "e1a6420ddb006fac5a91fcf88ab2703693aea4dee01b6e926558ec7c87427ec2"
const ParentCaptureProvenanceSHA256 = "b5f900ac836524b6577f59d28c2322bba9e105c5e520df84dfd4df754621a130"
const ParentNativeTypedCellsSHA256 = "531620568e5340289c214f276a94d2942a9cd9e80d398f9082dfb1c1f8e9d564"

func ParentProfile() Profile {
	lists := []ListDefinition{
		{"BedrockType", 87}, {"FloodingRegimeDur", 9}, {"FloodingRegimeFreq", 6}, {"GeoMorPro", 22},
		{"HumusForm", 24}, {"HumusFormPhase", 25}, {"HydrogeoSubsystem", 13}, {"HydrogeoSystem", 7},
		{"RealmClass", 45}, {"RootRestrictingType", 9}, {"RootZoneParticleSize", 25}, {"SoilDrainage", 14},
		{"SurfaceExp", 18}, {"SurficialMaterial", 17}, {"TerrainTexture", 17}, {"WaterSource", 9},
	}
	queries := make([]string, len(lists))
	for i, list := range lists {
		queries[i] = DAOSourceSQL(list.Name)
	}
	return Profile{"current-readonly-dao-parent-codes-v1", SourceSHA256, ParentSnapshotSHA256,
		strings.Join(queries, " "), lists}
}

// The 15-list capture and the earlier Bedrock capture have separate immutable seals.
func ParentSnapshotRows(data, captureProvenance, manifest, bedrock []byte, p Provenance) ([]Choice, error) {
	if err := ValidateProvenanceFor(p, ParentProfile()); err != nil {
		return nil, err
	}
	if checksum(data) != ParentSnapshotSHA256 || checksum(captureProvenance) != ParentCaptureProvenanceSHA256 ||
		checksum(manifest) != ParentManifestSHA256 || checksum(bedrock) != GeologySnapshotSHA256 {
		return nil, errors.New("parent DAO source/manifest checksum mismatch")
	}
	return decodeParentSnapshot(data, captureProvenance, manifest, bedrock, p)
}

func decodeParentSnapshot(data, captureProvenance, manifest, bedrock []byte, p Provenance) ([]Choice, error) {
	var seal struct {
		ChecklistVerified bool `json:"sourceChecklistVerified"`
		Lists             int  `json:"capturedLists"`
		Rows              int  `json:"capturedRows"`
		Columns           int  `json:"fullTypedColumns"`
		Bedrock           []struct {
			List string `json:"listName"`
			SHA  string `json:"sha256"`
		} `json:"BedrockTypeReusedNotRecaptured"`
		Artifacts []struct {
			Path  string `json:"path"`
			Bytes int    `json:"bytes"`
			SHA   string `json:"sha256"`
		} `json:"artifacts"`
	}
	var source struct {
		BeforeSHA256                       string
		AfterSHA256                        string
		Failure                            *string
		LengthBefore                       int
		LengthAfter                        int
		ByteIdentityUnchanged              bool
		DAOHandlesClosed                   bool
		LockFileAbsentAfter                bool
		NoNewAccessPIDObserved             bool
		NoAccessApplicationMechanismUsed   bool
		ExclusiveReadonlyHashHandlesClosed bool
		CatalogueCounts                    []struct {
			Count int
			SQL   string
		}
	}
	var snapshot struct {
		Readonly, NoAccessApplication, NativeGUIStarted, DAOHandlesClosed, OnlyClosedOwnedSource bool
		Schema                                                                                   daoSnapshotSchema
		Catalogues                                                                               []daoSnapshotCatalogue
	}
	for _, input := range []struct {
		data   []byte
		target any
	}{{manifest, &seal}, {captureProvenance, &source}, {data, &snapshot}} {
		if err := json.Unmarshal(input.data, input.target); err != nil {
			return nil, err
		}
	}
	if !seal.ChecklistVerified || seal.Lists != 15 || seal.Rows != 260 || seal.Columns != 10 ||
		len(seal.Bedrock) != 1 || seal.Bedrock[0].List != "BedrockType" || seal.Bedrock[0].SHA != GeologySnapshotSHA256 ||
		source.BeforeSHA256 != p.SourceSHA256 || source.AfterSHA256 != p.SourceSHA256 || source.Failure != nil ||
		source.LengthBefore != 8818688 || source.LengthAfter != source.LengthBefore ||
		!source.ByteIdentityUnchanged || !source.DAOHandlesClosed || !source.LockFileAbsentAfter ||
		!source.NoNewAccessPIDObserved || !source.NoAccessApplicationMechanismUsed || !source.ExclusiveReadonlyHashHandlesClosed ||
		!snapshot.Readonly || !snapshot.NoAccessApplication || snapshot.NativeGUIStarted ||
		!snapshot.DAOHandlesClosed || !snapshot.OnlyClosedOwnedSource ||
		len(snapshot.Catalogues) != 15 || len(source.CatalogueCounts) != 15 {
		return nil, errors.New("parent current DAO source/provenance mismatch")
	}
	for _, file := range []struct {
		name string
		data []byte
	}{{"catalogue-snapshot.json", data}, {"catalogue-provenance.json", captureProvenance}} {
		matches := 0
		for _, artifact := range seal.Artifacts {
			if strings.HasSuffix(artifact.Path, `\`+file.name) {
				matches++
				if artifact.Bytes != len(file.data) || artifact.SHA != checksum(file.data) {
					return nil, fmt.Errorf("parent manifest %s seal mismatch", file.name)
				}
			}
		}
		if matches != 1 {
			return nil, fmt.Errorf("parent manifest %s identity missing/ambiguous", file.name)
		}
	}
	if err := validateDAOSchema(snapshot.Schema, p); err != nil {
		return nil, err
	}
	geoProfile := GeologyProfile()
	geo := p
	geo.Exporter, geo.SnapshotSHA256, geo.SQL = geoProfile.Exporter, geoProfile.SnapshotSHA256, geoProfile.SQL
	geo.Rows, geo.Cells = 87, 870
	rows, err := GeologySnapshotRows(bedrock, geo)
	if err != nil {
		return nil, fmt.Errorf("parent Bedrock boundary: %w", err)
	}
	var bedrockSnapshot struct {
		DAO struct{ Catalogues []daoSnapshotCatalogue } `json:"dao"`
	}
	if err := readDAOGzip(bedrock, &bedrockSnapshot); err != nil {
		return nil, err
	}
	_, allCells, err := decodeDAORows(bedrockSnapshot.DAO.Catalogues[0], geoProfile.Lists[0])
	if err != nil {
		return nil, err
	}
	for i, list := range ParentProfile().Lists[1:] {
		catalogue := snapshot.Catalogues[i]
		if source.CatalogueCounts[i].SQL != catalogue.SQL || source.CatalogueCounts[i].Count != list.Rows {
			return nil, fmt.Errorf("parent %s provenance query/count mismatch", list.Name)
		}
		converted, native, err := decodeDAORows(catalogue, list)
		if err != nil {
			return nil, err
		}
		rows = append(rows, converted...)
		allCells = append(allCells, native...)
	}
	hash, err := daoNativeHash(allCells)
	if err != nil || hash != ParentNativeTypedCellsSHA256 {
		return nil, errors.New("parent3470 native-cell checksum mismatch")
	}
	return rows, nil
}
