package vprocore

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"crypto/sha256"

	"errors"
	"fmt"
	"github.com/boostao/vpro-wails/resources"
	"os"
	"path/filepath"
)

var DatabaseFamilyFiles = resources.DatabaseFamilyFS

type databaseFamilySeed struct {
	Name, Hash string `json:"-" yaml:"-"`
	tables     []string
}

var DatabaseFamilySeeds = []databaseFamilySeed{
	{"VPro64", "4f299ffb69fd5d90ba7fd454ac200a935153acc762bfe45112659a8adc672c8b",
		[]string{"USysEnvTable", "USysVegTable", "USysSuTable", "USysHierarchyTable"}},
	{"VLists", "81e259244106b14d6dd3c65f0a7e7e87bdb06f60e6c54947c0506e2039379441",
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

const legacyVListsSeedSHA256 = "dd80135e45878d92dee626701ba29bd4cc1d69549d6784c2a91dc276f6a79a51"

func InstallDatabaseFamily(root string) (map[string]string, error) {
	directory := filepath.Join(root, "database-family")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	paths := make(map[string]string, len(DatabaseFamilySeeds))
	for _, seed := range DatabaseFamilySeeds {
		destination := filepath.Join(directory, seed.Name+".db")
		if _, err := os.Stat(destination); errors.Is(err, os.ErrNotExist) {
			data, err := DatabaseFamilyFiles.ReadFile("resources/database-family/" + seed.Name + ".db")
			if err != nil {
				return nil, err
			}
			if fmt.Sprintf("%x", sha256.Sum256(data)) != seed.Hash {
				return nil, fmt.Errorf("bundled %s seed hash does not match its provenance seal", seed.Name)
			}
			if err := installDatabaseSeed(destination, data); err != nil {
				return nil, fmt.Errorf("install %s: %w", seed.Name, err)
			}
		} else if err != nil {
			return nil, err
		} else if seed.Name == "VLists" && seed.Hash != legacyVListsSeedSHA256 {
			if existing, readErr := os.ReadFile(destination); readErr == nil && fmt.Sprintf("%x", sha256.Sum256(existing)) == legacyVListsSeedSHA256 {
				data, err := DatabaseFamilyFiles.ReadFile("resources/database-family/VLists.db")
				if err != nil {
					return nil, err
				}
				if fmt.Sprintf("%x", sha256.Sum256(data)) != seed.Hash {
					return nil, fmt.Errorf("bundled %s seed hash does not match its provenance seal", seed.Name)
				}
				if err := sqlutil.ReplaceBundledCatalogueFile(destination, data, 0600); err != nil {
					return nil, fmt.Errorf("upgrade %s: %w", seed.Name, err)
				}
			}
		}
		paths[seed.Name] = destination
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
	if err := InstallDesktopConfig(file.Name(), destination); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return nil
}
