package plotentry

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type PersonalSpeciesDefinitionRequest struct {
	Entered        string  `json:"entered"`
	ScientificName *string `json:"scientificName"`
	Lifeform       *int    `json:"lifeform"`
	EnglishName    *string `json:"englishName"`
}

func (request *PersonalSpeciesDefinitionRequest) UnmarshalJSON(data []byte) error {
	if err := sqlutil.ValidateJSONTextProperties(data, map[string]string{
		"entered": "Personal species entered code", "scientificName": "Personal species scientific name",
		"englishName": "Personal species English name",
	}); err != nil {
		return err
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	for _, property := range []string{"entered", "scientificName", "lifeform", "englishName"} {
		value, present := properties[property]
		if !present || property == "entered" && strings.TrimSpace(string(value)) == "null" {
			return fmt.Errorf("personal species requires explicit %s, preserving NULL metadata", property)
		}
	}
	for property := range properties {
		if property != "entered" && property != "scientificName" && property != "lifeform" && property != "englishName" {
			return fmt.Errorf("personal species property %q is unavailable; hidden defaults and family ownership are not caller-selected", property)
		}
	}
	type plain PersonalSpeciesDefinitionRequest
	return json.Unmarshal(data, (*plain)(request))
}

func preparePersonalSpecies(request PersonalSpeciesDefinitionRequest) (vprocore.VegetationSpeciesOption, error) {
	code, err := vprocore.VegetationSpeciesEventUpper(request.Entered)
	if err != nil {
		return vprocore.VegetationSpeciesOption{}, err
	}
	for name, value := range map[string]*string{"ScientificName": request.ScientificName, "EnglishName": request.EnglishName} {
		if value != nil {
			if err := vprocore.ValidateChildPhysicalText("USysUserSpp."+name, *value, 255); err != nil {
				return vprocore.VegetationSpeciesOption{}, err
			}
		}
	}
	if request.Lifeform != nil && (*request.Lifeform < -32768 || *request.Lifeform > 32767) {
		return vprocore.VegetationSpeciesOption{}, errors.New("personal species LifeForm is outside the source signed16 INTEGER domain")
	}
	return vprocore.VegetationSpeciesOption{Code: &code, ScientificName: request.ScientificName,
		Lifeform: request.Lifeform, EnglishName: request.EnglishName}, nil
}

func ValidatePersonalSpeciesFiles(c *vprocore.SqliteContext) error {
	userInfo := c.AttachmentInfo["VUser"]
	if userInfo == nil || c.AttachmentInfo["VLists"] == nil {
		return errors.New("personal species requires identified original user and reference attachments")
	}
	for role, info := range c.AttachmentInfo {
		current, err := os.Stat(c.Attachments[role])
		if err != nil {
			return fmt.Errorf("personal species %s ownership unavailable: %w", role, err)
		}
		if !os.SameFile(current, info) {
			return fmt.Errorf("personal species %s file identity changed; reopen the context", role)
		}
		if role != "VUser" && os.SameFile(userInfo, info) {
			return fmt.Errorf("personal species user file is also owned by %s; shared-role writes are unavailable", role)
		}
	}
	return nil
}

func WithPersonalSpeciesWriter(c *vprocore.SqliteContext, ctx context.Context, operation func(*sql.Conn) error) (resultErr error) {
	if err := sqlutil.AcquireMutexLease(ctx, &c.Mu); err != nil {
		return err
	}
	defer c.Mu.Unlock()
	if c.Conn == nil {
		return errors.New("personal species context is closed")
	}
	if err := ValidatePersonalSpeciesFiles(c); err != nil {
		return err
	}
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(c.Attachments["VUser"], "rw")+"&_foreign_keys=on&_busy_timeout=5000&_txlock=immediate")
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, conn.Close()) }()
	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS "reference"`, sqlutil.SqliteFileURI(c.Attachments["VLists"], "ro")); err != nil {
		return fmt.Errorf("personal species references unavailable: %w", err)
	}
	return operation(conn)
}

type personalSpeciesCommittedError struct{ cause error }

func (err *personalSpeciesCommittedError) Error() string {
	return "personal species definition committed, but writer cleanup failed; reload definitions before retrying: " + err.cause.Error()
}

func (err *personalSpeciesCommittedError) Unwrap() error { return err.cause }

// This saves only a user definition; assigning it to a plot is a separate draft Save.
func CreatePersonalSpeciesDefinition(s *vprocore.ContextService, ctx context.Context, contextID string, request PersonalSpeciesDefinitionRequest) (vprocore.VegetationSpeciesOption, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (vprocore.VegetationSpeciesOption, error) {
		return PlotCreatePersonalSpeciesDefinition(plots, request)
	})
}

func CtxCreatePersonalSpeciesDefinition(s *vprocore.ContextService, ctx context.Context, contextID string, request PersonalSpeciesDefinitionRequest) (vprocore.VegetationSpeciesOption, error) {
	return CreatePersonalSpeciesDefinition(s, ctx, contextID, request)
}

func PlotCreatePersonalSpeciesDefinition(s *vprocore.PlotService, request PersonalSpeciesDefinitionRequest) (vprocore.VegetationSpeciesOption, error) {
	if err := s.RequireContextEdit(); err != nil {
		return vprocore.VegetationSpeciesOption{}, err
	}
	proposed, err := preparePersonalSpecies(request)
	if err != nil {
		return vprocore.VegetationSpeciesOption{}, err
	}
	s.Projects.Mu.RLock()
	coordinator := s.Projects.Sqlite
	project := s.Projects.Active
	s.Projects.Mu.RUnlock()
	if coordinator == nil {
		return vprocore.VegetationSpeciesOption{}, errors.New("personal species requires the canonical SQLite family context")
	}
	s.Mu.RLock()
	user := s.CurrentUser
	s.Mu.RUnlock()
	if err := vprocore.ValidateChildPhysicalText("USysAuditTrail.User", user, 255); err != nil {
		return vprocore.VegetationSpeciesOption{}, err
	}
	committed := false
	err = WithPersonalSpeciesWriter(coordinator, s.OperationContext(), func(conn *sql.Conn) (resultErr error) {
		ctx := s.OperationContext()
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("personal species transaction unavailable: %w", err)
		}
		defer func() {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				resultErr = errors.Join(resultErr, err)
			}
		}()
		var collision bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(
			SELECT 1 FROM main.USysUserSpp WHERE Code COLLATE NOCASE=?
			UNION ALL SELECT 1 FROM reference.USysAllSpecs
			WHERE Code COLLATE NOCASE=? OR (OldCode COLLATE NOCASE=? AND Code IS NOT NULL)
		)`, *proposed.Code, *proposed.Code, request.Entered).Scan(&collision); err != nil {
			return fmt.Errorf("personal species collision review unavailable: %w", err)
		}
		if collision {
			return errors.New("personal species code already has a user/master/old-code definition; review the existing definitions instead")
		}
		result, err := tx.ExecContext(ctx, `INSERT INTO main.USysUserSpp(Code,ScientificName,LifeForm,EnglishName) VALUES (?,?,?,?)`,
			*proposed.Code, proposed.ScientificName, proposed.Lifeform, proposed.EnglishName)
		if err != nil {
			return fmt.Errorf("personal species insertion failed: %w", err)
		}
		if affected, err := result.RowsAffected(); err != nil || affected != 1 {
			return errors.Join(fmt.Errorf("personal species insertion expected one row, found %d", affected), err)
		}
		snapshot := struct {
			Code           string  `json:"Code"`
			ScientificName *string `json:"ScientificName"`
			Lifeform       *int    `json:"LifeForm"`
			EnglishName    *string `json:"EnglishName"`
			Report         float64 `json:"Report"`
			SppNumber      *int    `json:"SppNumber"`
			CodeType       *string `json:"Codetype"`
		}{Code: *proposed.Code, ScientificName: proposed.ScientificName,
			Lifeform: proposed.Lifeform, EnglishName: proposed.EnglishName, Report: 1}
		after, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		when := time.Now().Format("2006-01-02 15:04:05")
		result, err = tx.ExecContext(ctx, `INSERT INTO main.USysAuditTrail
			("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit")
			VALUES (?,?,NULL,'USysUserSpp','CreateRecord',?,NULL,?)`,
			project, user, when, string(after))
		if err != nil {
			return fmt.Errorf("personal species audit failed: %w", err)
		}
		if affected, err := result.RowsAffected(); err != nil || affected != 1 {
			return errors.Join(fmt.Errorf("personal species audit expected one row, found %d", affected), err)
		}
		auditID, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("personal species audit identity observation failed: %w", err)
		}
		var audited bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM main.USysAuditTrail
			WHERE rowid=? AND "Project" IS ? AND "User" IS ? AND PlotNumber IS NULL
			AND "Table"='USysUserSpp' AND EditField='CreateRecord' AND EditWhen IS ?
			AND BeforeEdit IS NULL AND AfterEdit IS ?)`, auditID, project, user, when, string(after)).Scan(&audited); err != nil {
			return fmt.Errorf("personal species stored audit observation failed: %w", err)
		}
		if !audited {
			return errors.New("personal species stored audit differs from the planned definition; creation rolled back")
		}
		var matches int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM main.USysUserSpp
			WHERE Code COLLATE NOCASE=?`, *proposed.Code).Scan(&matches); err != nil {
			return fmt.Errorf("personal species stored identity observation failed: %w", err)
		}
		var planned bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM main.USysUserSpp
			WHERE Code COLLATE BINARY=? AND ScientificName IS ? AND LifeForm IS ? AND EnglishName IS ?
			AND Report IS 1.0 AND SppNumber IS NULL AND Codetype IS NULL)`,
			*proposed.Code, proposed.ScientificName, proposed.Lifeform, proposed.EnglishName).Scan(&planned); err != nil {
			return fmt.Errorf("personal species stored metadata observation failed: %w", err)
		}
		if matches != 1 || !planned {
			return errors.New("personal species stored definition differs from planned values/source defaults; no hidden values are inferred")
		}
		if err := ValidatePersonalSpeciesFiles(coordinator); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("personal species commit failed: %w", err)
		}
		committed = true
		return nil
	})
	if err != nil {
		if committed {
			return proposed, &personalSpeciesCommittedError{cause: err}
		}
		return vprocore.VegetationSpeciesOption{}, err
	}
	return proposed, nil
}
