package main

import (
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed resources/database-family/*.db
var databaseFamilyFiles embed.FS

type databaseFamilySeed struct {
	name, hash string
	tables     []string
}

var databaseFamilySeeds = []databaseFamilySeed{
	{"VPro64", "4f299ffb69fd5d90ba7fd454ac200a935153acc762bfe45112659a8adc672c8b",
		[]string{"USysEnvTable", "USysVegTable", "USysSuTable", "USysHierarchyTable"}},
	{"VLists", "dd80135e45878d92dee626701ba29bd4cc1d69549d6784c2a91dc276f6a79a51",
		[]string{"USysAllSpecs", "USysTableOfLists", "MasterSiteUnitList", "USysMetadataTemplates",
			"USysZoneList", "USysMasterAudit", "USysSiteSeriesNames", "USysSppAttributes",
			"USysSppAttributeFieldList", "USysSppAttributeDescription"}},
	{"VUser", "19c8cf4d7b0cfae2ad141b5f02918de0428ad1dd1f27d3f3c8b40ca48874d4e8",
		[]string{"USysUserSpp", "UserSiteUnitList", "USysSiteUnitAll", "USysPrefs"}},
	{"VMetaData", "fc197b434d4fa22945ce915fe867898458b2ed71a6c887dac288b9f7e75fa35e",
		[]string{"ProjectMetadata", "ProjectMetaDataCodes"}},
	{"VMessageBoard", "9b6803b16389e88bca3aae45e589c46e2cfd3011809daa26f3331987dd15bac0",
		[]string{"tblMessageBoard", "tblMessageList"}},
}

func installDatabaseFamily(root string) (map[string]string, error) {
	directory := filepath.Join(root, "database-family")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	paths := make(map[string]string, len(databaseFamilySeeds))
	for _, seed := range databaseFamilySeeds {
		destination := filepath.Join(directory, seed.name+".db")
		if _, err := os.Stat(destination); errors.Is(err, os.ErrNotExist) {
			data, err := databaseFamilyFiles.ReadFile("resources/database-family/" + seed.name + ".db")
			if err != nil {
				return nil, err
			}
			if fmt.Sprintf("%x", sha256.Sum256(data)) != seed.hash {
				return nil, fmt.Errorf("bundled %s seed hash does not match its provenance seal", seed.name)
			}
			if err := installDatabaseSeed(destination, data); err != nil {
				return nil, fmt.Errorf("install %s: %w", seed.name, err)
			}
		} else if err != nil {
			return nil, err
		}
		paths[seed.name] = destination
	}
	return paths, nil
}

func installDatabaseSeed(destination string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(destination), ".family-*.db")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := installDesktopConfig(file.Name(), destination); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return nil
}
