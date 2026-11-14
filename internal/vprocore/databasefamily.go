package vprocore

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"github.com/boostao/vpro-wails/resources"
	"os"
	"path/filepath"
	"strings"
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
const VPicsSeedSHA256 = "671ef38d6341a7a917107c457079dff336b24886369b4a3af339b45559c8312a"

// InstallVPicsDatabase installs the canonical VPics.db (tblVPics 5-column schema)
// inside <root>/database-family/VPics.db without adding VPics to the read-only
// attached DatabaseFamilySeeds map (preserving len(support) == len(DatabaseFamilySeeds)
// and allowing owned picture mutations against VPics.db).
func InstallVPicsDatabase(root string) (string, error) {
	directory := filepath.Join(root, "database-family")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	destination := filepath.Join(directory, "VPics.db")
	if _, err := os.Stat(destination); errors.Is(err, os.ErrNotExist) {
		data, err := DatabaseFamilyFiles.ReadFile("resources/database-family/VPics.db")
		if err != nil {
			return "", err
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != VPicsSeedSHA256 {
			return "", errors.New("bundled VPics seed hash does not match its provenance seal")
		}
		if err := installDatabaseSeed(destination, data); err != nil {
			return "", fmt.Errorf("install VPics: %w", err)
		}
	} else if err != nil {
		return "", err
	}
	return destination, nil
}

// EnsurePictureDirectory provisions the default <root>/pictures folder.
func EnsurePictureDirectory(root string) (string, error) {
	directory := filepath.Join(root, "pictures")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return "", err
	}
	return directory, nil
}

func InstallDatabaseFamily(root string) (map[string]string, error) {
	directory := filepath.Join(root, "database-family")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return nil, err
	}
	if _, err := InstallVPicsDatabase(root); err != nil {
		return nil, err
	}
	if _, err := EnsurePictureDirectory(root); err != nil {
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

// EnsureDefaultPictureLibrary ensures dataDir/pics/images exists and seeds dataDir/pics/VPics.db
// (if absent) with the canonical 5-column tblVPics schema and VP08 _table_metadata tag.
func EnsureDefaultPictureLibrary(dataDir string) (dbPath string, imagesDir string, err error) {
	if strings.TrimSpace(dataDir) == "" {
		return "", "", errors.New("dataDir must not be empty")
	}
	picsDir := filepath.Join(dataDir, "pics")
	imagesDir = filepath.Join(picsDir, "images")
	if err := os.MkdirAll(imagesDir, 0700); err != nil {
		return "", "", err
	}
	dbPath = filepath.Join(picsDir, "VPics.db")
	if _, statErr := os.Stat(dbPath); statErr == nil {
		return dbPath, imagesDir, nil
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return "", "", statErr
	}

	tmpFile, err := os.CreateTemp(picsDir, ".vpics-seed-*.db")
	if err != nil {
		return "", "", err
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmpFile.Chmod(0600); err != nil {
		_ = tmpFile.Close()
		return "", "", err
	}
	if err := tmpFile.Close(); err != nil {
		return "", "", err
	}

	sqlDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(tmpPath, "rw")+"&_journal_mode=DELETE&_synchronous=FULL&_txlock=immediate")
	if err != nil {
		return "", "", err
	}
	sqlDB.SetMaxOpenConns(1)
	tx, err := sqlDB.Begin()
	if err != nil {
		_ = sqlDB.Close()
		return "", "", err
	}
	const vpicsDDL = `
		CREATE TABLE "tblVPics" (
			"ID" INTEGER,
			"PicDir" TEXT,
			"PicName" TEXT,
			"PlotNumber" TEXT,
			"PicComment" TEXT
		);
		CREATE TABLE "_table_metadata" (
			"table_name" TEXT PRIMARY KEY,
			"description" TEXT
		);
		INSERT INTO "_table_metadata" ("table_name", "description") VALUES ('tblVPics', 'VP08');
	`
	if _, err := tx.Exec(vpicsDDL); err != nil {
		_ = tx.Rollback()
		_ = sqlDB.Close()
		return "", "", err
	}
	if err := tx.Commit(); err != nil {
		_ = sqlDB.Close()
		return "", "", err
	}
	if err := sqlDB.Close(); err != nil {
		return "", "", err
	}
	if err := InstallDesktopConfig(tmpPath, dbPath); err != nil && !errors.Is(err, os.ErrExist) {
		return "", "", err
	}
	return dbPath, imagesDir, nil
}

// ReplaceActiveVListsCatalogue validates staged VLists.db bytes, atomically swaps the
// attached VLists.db via sqlutil.ReplaceBundledCatalogueFile while safely cycling the
// SQLiteContext handle (preventing open-file rename failures on Windows), and returns
// the refreshed ProjectState with a newly minted ContextID.
func ReplaceActiveVListsCatalogue(ctx context.Context, s *ContextService, expectedID string, newVListsBytes []byte) (ProjectState, error) {
	if err := ctx.Err(); err != nil {
		return ProjectState{}, err
	}
	if s == nil || s.Projects == nil {
		return ProjectState{}, errors.New("context service is unavailable")
	}
	if len(newVListsBytes) < 4096 {
		return ProjectState{}, errors.New("staged VLists.db payload is empty or truncated")
	}

	// Pre-validate staged SQLite image in an isolated temporary file before touching owned state.
	probeDir, err := os.MkdirTemp("", "vpro-vlists-validate-*")
	if err != nil {
		return ProjectState{}, err
	}
	defer os.RemoveAll(probeDir)
	probePath := filepath.Join(probeDir, "VLists.db")
	if err := os.WriteFile(probePath, newVListsBytes, 0o600); err != nil {
		return ProjectState{}, err
	}
	probeDB, err := sqlutil.OpenReadOnlyContext(ctx, probePath)
	if err != nil {
		return ProjectState{}, fmt.Errorf("validate staged VLists.db: %w", err)
	}
	for _, requiredTbl := range []string{"USysAllSpecs", "USysTableOfLists", "MasterSiteUnitList", "USysZoneList"} {
		var exists bool
		if qErr := probeDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)`, requiredTbl).Scan(&exists); qErr != nil || !exists {
			_ = probeDB.Close()
			return ProjectState{}, fmt.Errorf("staged VLists.db is missing required table %s", requiredTbl)
		}
	}
	_ = probeDB.Close()

	s.Projects.OperationMu.Lock()
	defer s.Projects.OperationMu.Unlock()
	s.Projects.Mu.Lock()
	defer s.Projects.Mu.Unlock()

	if expectedID == "" || expectedID != s.Projects.ContextID || s.Projects.Sqlite == nil || s.Projects.Sqlite.Conn == nil {
		return ProjectState{}, errors.New("project context changed; reload before syncing VLists")
	}
	vlistsPath := s.Projects.SupportPaths["VLists"]
	if strings.TrimSpace(vlistsPath) == "" {
		vlistsPath = s.Projects.Sqlite.Attachments["VLists"]
	}
	if strings.TrimSpace(vlistsPath) == "" {
		return ProjectState{}, errors.New("active context has no VLists.db attachment path")
	}

	backupBytes, err := os.ReadFile(vlistsPath)
	if err != nil {
		return ProjectState{}, fmt.Errorf("read current VLists.db backup: %w", err)
	}
	sel := s.Projects.Sqlite.Selection
	prof := s.Projects.Sqlite.Profile
	oldID := s.Projects.ContextID

	// Close current SqliteContext before renaming VLists.db so Windows file locks are released cleanly.
	_ = s.Projects.Sqlite.Close()
	s.Projects.Sqlite = nil

	restorePrior := func(cause error) (ProjectState, error) {
		_ = sqlutil.ReplaceBundledCatalogueFile(vlistsPath, backupBytes, 0o600)
		if recovered, rErr := NewSQLiteContext(context.Background(), sel, s.Projects.SupportPaths, prof); rErr == nil {
			s.Projects.PublishSQLiteSelection(recovered, oldID)
		}
		return ProjectState{}, cause
	}

	if err := sqlutil.ReplaceBundledCatalogueFile(vlistsPath, newVListsBytes, 0o600); err != nil {
		return restorePrior(fmt.Errorf("replace VLists.db: %w", err))
	}

	candidate, err := NewSQLiteContext(context.Background(), sel, s.Projects.SupportPaths, prof)
	if err != nil {
		return restorePrior(fmt.Errorf("reopen SQLite context with updated VLists.db: %w", err))
	}
	newID, err := NewContextIdentity()
	if err != nil {
		_ = candidate.Close()
		return restorePrior(err)
	}
	s.Projects.PublishSQLiteSelection(candidate, newID)
	state, err := s.Projects.SqliteStateContextLocked(ctx)
	if err != nil {
		_ = candidate.Close()
		return restorePrior(err)
	}
	s.SelectedUnitSelections.Mu.Lock()
	s.SelectedUnitSelections.Entries = nil
	s.SelectedUnitSelections.Mu.Unlock()
	s.ProjectReportScopes.Mu.Lock()
	s.ProjectReportScopes.Entries = nil
	s.ProjectReportScopes.Mu.Unlock()
	return state, nil
}


