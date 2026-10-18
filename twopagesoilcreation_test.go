package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTwoPageSoilCreationWire(t *testing.T) {
	type wireCase struct {
		Name    string          `json:"name"`
		Request json.RawMessage `json:"request"`
		Receipt json.RawMessage `json:"receipt"`
	}
	target := os.Getenv("VPRO_SOIL_CREATION_WIRE_OUTPUT")
	if target != "" {
		cases := []wireCase{}
		for _, form := range []string{"SoilHumusXL", "SoilMineralXL"} {
			for strength := 0; strength <= 3; strength++ {
				service, state, _, request := soilCreationWriteFixture(t, false, form, strength)
				raw, err := json.Marshal(request)
				if err != nil {
					t.Fatal(err)
				}
				result, err := service.CreateHorizon(context.Background(), state.ContextID, string(raw))
				if err != nil {
					t.Fatal(err)
				}
				encoded, err := json.Marshal(result)
				if err != nil {
					t.Fatal(err)
				}
				cases = append(cases, wireCase{Name: fmt.Sprintf("%s/%d", form, strength), Request: raw, Receipt: encoded})
			}
		}
		file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		encodeErr := json.NewEncoder(file).Encode(cases)
		if err := errors.Join(encodeErr, file.Close()); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "twoPageSoilCreationWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []wireCase
	if err := json.Unmarshal(raw, &cases); err != nil || len(cases) != 8 {
		t.Fatal("missing strict-Go creation cases", err)
	}
	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			var request twoPageSoilRequest
			if err := json.Unmarshal(test.Request, &request); err != nil {
				t.Fatal(err)
			}
			var receipt twoPageSoilWriteResult
			if err := json.Unmarshal(test.Receipt, &receipt); err != nil {
				t.Fatal(err)
			}
			if !receipt.DidCommit || receipt.Replayed || !reflect.DeepEqual(receipt.History.Request, request) {
				t.Fatal("shared fixture lost its exact immutable creation request")
			}
			if err := validateTwoPageSoilHistory(context.Background(), receipt.History); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func soilCreationWriteFixture(t *testing.T, external bool, form string, strength int) (*TwoPageSoilService, ProjectState, *sql.DB, twoPageSoilRequest) {
	t.Helper()
	contexts, state, db := twoPageSoilReadFixture(t, external, "TEXT")
	if err := contexts.plots.SetAuditStrength(strength); err != nil {
		t.Fatal(err)
	}
	contexts.plots.mu.Lock()
	contexts.plots.currentUser = "soil-creation"
	contexts.plots.mu.Unlock()
	prototype := soilCreationOriginal(form)
	table := "Sample_Humus"
	if form == "SoilMineralXL" {
		table = "Sample_Mineral"
	}
	definitions := []string{}
	for _, column := range prototype.Records.Columns {
		declared := ""
		if column.Name == "ID" || column.Name == "Flag" {
			declared = " INTEGER"
		}
		if column.Name == "PlotNumber" || column.Name == "Horizon" {
			declared = " TEXT"
		}
		definitions = append(definitions, quoteHeaderIdentifier(column.Name)+declared)
	}
	if _, err := db.Exec("DROP TABLE " + quoteHeaderIdentifier(table) + "; CREATE TABLE " +
		quoteHeaderIdentifier(table) + " (" + strings.Join(definitions, ",") +
		"); INSERT INTO " + quoteHeaderIdentifier(table) +
		"(ID,PlotNumber,Horizon,Flag) VALUES(2,'108050','old',0),(1,'other','peer',-1),(-7,'108050','history',NULL),(NULL,'108050','null ID',NULL)"); err != nil {
		t.Fatal(err)
	}
	service := soilFacade(t, contexts, map[string]string{twoPageSoilReviewEnvironment: "true",
		twoPageSoilWritingEnvironment: "true", twoPageSoilCreationEnvironment: "true"})
	original, err := service.GetOriginal(context.Background(), state.ContextID, "108050", form)
	if err != nil {
		t.Fatal(err)
	}
	horizon := " Oh "
	return service, state, db, twoPageSoilRequest{RequestID: soilWriteUUID, Original: original, Edits: []twoPageSoilEdit{}, Horizon: &horizon}
}

func TestTwoPageSoilCreationAtomicWriterAndRecovery(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, form := range []string{"SoilHumusXL", "SoilMineralXL"} {
			for strength := 0; strength <= 3; strength++ {
				t.Run(fmt.Sprintf("%s/external-%v/strength-%d", form, external, strength), func(t *testing.T) {
					service, state, db, request := soilCreationWriteFixture(t, external, form, strength)
					raw, err := json.Marshal(request)
					if err != nil {
						t.Fatal(err)
					}
					before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
					if result, err := service.Save(context.Background(), state.ContextID, string(raw)); err == nil || result != nil {
						t.Fatal("ordinary Save accepted creation", result, err)
					}
					if after := databaseBytes(t, service.contexts.projects.sqlite.attachments); !reflect.DeepEqual(before, after) {
						t.Fatal("rejected ordinary Save changed physical storage")
					}
					priorAudit := soilWriteCount(t, db, "Sample_Audit")
					result, err := service.CreateHorizon(context.Background(), state.ContextID, string(raw))
					if err != nil || result == nil || !result.DidCommit || result.Replayed || result.History.CreatedRowID == nil {
						t.Fatalf("creation = %#v %v", result, err)
					}
					wantAudits := 0
					if strength >= 2 {
						wantAudits = 1
					}
					if len(result.History.Audits) != wantAudits || soilWriteCount(t, db, "Sample_Audit") != priorAudit+wantAudits {
						t.Fatal("source NULL-addition policy invented default/option audits")
					}
					for _, audit := range result.History.Audits {
						if audit.EditField != "Horizon" || audit.BeforeEdit != nil || audit.AfterEdit == nil ||
							*audit.AfterEdit != " Oh " || audit.ID == nil || *audit.ID != 3 {
							t.Fatal("creation audit changed literal text/allocated ID", audit)
						}
					}
					original, err := service.GetOriginal(context.Background(), state.ContextID, "108050", form)
					if err != nil || !reflect.DeepEqual(original.Records, result.History.Committed) ||
						len(original.Records.Rows) != len(request.Original.Records.Rows)+1 {
						t.Fatal("reopened complete rows differ", original, err)
					}
					columns, err := siteUnitTransferColumns(original.Records, "ID", "Flag", "Horizon")
					if err != nil {
						t.Fatal(err)
					}
					for _, row := range original.Records.Rows {
						if row.RowID == *result.History.CreatedRowID {
							if row.Cells[columns["ID"]].Integer == nil || *row.Cells[columns["ID"]].Integer != "3" ||
								row.Cells[columns["Flag"]].Integer == nil || *row.Cells[columns["Flag"]].Integer != "0" {
								t.Fatal("generated ID/default Flag differ", row)
							}
							for i, column := range original.Records.Columns {
								if column.Name != "ID" && column.Name != "Flag" && column.Name != "PlotNumber" && column.Name != "Horizon" &&
									row.Cells[i].Storage != "null" {
									t.Fatal("ordinary default was invented", column, row.Cells[i])
								}
							}
						}
					}
					before = databaseBytes(t, service.contexts.projects.sqlite.attachments)
					readOnly := soilFacade(t, service.contexts, map[string]string{twoPageSoilReviewEnvironment: "true"})
					outcome, err := readOnly.RecoverOutcome(context.Background(), state.ContextID, string(raw))
					if err != nil || outcome.Status != "committed" || outcome.Receipt == nil || !outcome.Receipt.Replayed {
						t.Fatal("read-only recovery lost committed creation", outcome, err)
					}
					replay, err := service.CreateHorizon(context.Background(), state.ContextID, string(raw))
					if err != nil || replay == nil || !replay.Replayed || replay.DidCommit {
						t.Fatal("stable UUID did not return its existing receipt", replay, err)
					}
					if after := databaseBytes(t, service.contexts.projects.sqlite.attachments); !reflect.DeepEqual(before, after) {
						t.Fatal("creation replay/recovery changed physical storage")
					}
				})
			}
		}
	}
}

func TestTwoPageSoilCreationRollbackAndPermanentIDs(t *testing.T) {
	service, state, db, request := soilCreationWriteFixture(t, false, "SoilHumusXL", 3)
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	failure := errors.New("held creation rollback")
	result, err := service.dispatchTwoPageSoilObserved(context.Background(), state.ContextID, string(raw), true,
		func(*sql.Tx) error { return failure }, nil)
	if !errors.Is(err, failure) || result != nil {
		t.Fatal("precommit creation failure not surfaced", result, err)
	}
	if after := databaseBytes(t, service.contexts.projects.sqlite.attachments); !reflect.DeepEqual(before, after) {
		t.Fatal("failed creation did not roll back all data/audit/history/reservations")
	}
	result, err = service.CreateHorizon(context.Background(), state.ContextID, string(raw))
	if err != nil || result == nil || !result.DidCommit {
		t.Fatal("explicit retry after rolled-back attempt failed", result, err)
	}
	if _, err := db.Exec("DELETE FROM Sample_Humus WHERE ID IN(1,3)"); err != nil {
		t.Fatal(err)
	}
	original, err := service.GetOriginal(context.Background(), state.ContextID, "108050", "SoilHumusXL")
	if err != nil {
		t.Fatal(err)
	}
	request.RequestID = "f4b1aa00-1234-4123-8123-000000000002"
	request.Original = original
	raw, err = json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	result, err = service.CreateHorizon(context.Background(), state.ContextID, string(raw))
	if err != nil || result == nil {
		t.Fatal("second explicit creation failed", result, err)
	}
	var id int64
	if err := db.QueryRow("SELECT ID FROM Sample_Humus WHERE rowid=?", *result.History.CreatedRowID).Scan(&id); err != nil || id != 4 {
		t.Fatal("deleted current/peer identity was reused", id, err)
	}
}
func TestTwoPageSoilCreationIndependentGatesAndStrictTransport(t *testing.T) {
	service, state, _, request := soilCreationWriteFixture(t, false, "SoilHumusXL", 3)
	raw := soilAttemptJSON(t, request)
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	for mask := 0; mask < 7; mask++ {
		flags := map[string]string{}
		for bit, name := range []string{twoPageSoilReviewEnvironment, twoPageSoilWritingEnvironment, twoPageSoilCreationEnvironment} {
			if mask&(1<<bit) != 0 {
				flags[name] = "true"
			}
		}
		if result, err := soilFacade(t, service.contexts, flags).CreateHorizon(context.Background(), state.ContextID, raw); err == nil || result != nil {
			t.Fatal("creation bypassed an independent prerequisite", mask, result, err)
		}
	}
	for _, malformed := range []string{
		strings.Replace(raw, `"horizon":" Oh "`, `"horizon":null`, 1),
		strings.Replace(raw, `"horizon":" Oh "`, `"horizon":""`, 1),
		strings.Replace(raw, `"horizon":" Oh "`, `"horizon":"123456789"`, 1),
		strings.Replace(raw, `"horizon":" Oh "`, `"horizon":"\ud800"`, 1),
		strings.Replace(raw, `"horizon":" Oh "`, `"horizon":"Oh","horizon":"Of"`, 1),
		strings.Replace(raw, `"horizon":" Oh "`, `"horizon":"Oh","\u0068orizon":"Of"`, 1),
		strings.Replace(raw, `"horizon":" Oh "`, `"horizon":123`, 1),
		strings.Replace(raw, `"horizon":" Oh "`, `"horizon":"Oh","unrecognized":true`, 1),
		strings.Replace(raw, `"edits":[]`, `"edits":[{}]`, 1),
	} {
		if malformed == raw {
			t.Fatal("strict transport test did not alter the actual wire")
		}
		if result, err := service.CreateHorizon(context.Background(), state.ContextID, malformed); err == nil || result != nil {
			t.Fatal("invalid creation transport accepted", malformed, result, err)
		}
	}
	ordinary := request
	ordinary.Horizon = nil
	if result, err := service.CreateHorizon(context.Background(), state.ContextID, soilAttemptJSON(t, ordinary)); err == nil || result != nil {
		t.Fatal("explicit creation accepted an ordinary request")
	}
	if after := databaseBytes(t, service.contexts.projects.sqlite.attachments); !reflect.DeepEqual(before, after) {
		t.Fatal("gate/transport refusals changed physical storage or schema")
	}
	result, err := service.CreateHorizon(context.Background(), state.ContextID, raw)
	if err != nil || result == nil || !result.DidCommit {
		t.Fatal("refused transport prevented corrected explicit creation", result, err)
	}
	before = databaseBytes(t, service.contexts.projects.sqlite.attachments)
	changed := "Of"
	request.Horizon = &changed
	if result, err := service.CreateHorizon(context.Background(), state.ContextID, soilAttemptJSON(t, request)); err == nil || result != nil {
		t.Fatal("creation UUID authorized a different payload")
	}
	if after := databaseBytes(t, service.contexts.projects.sqlite.attachments); !reflect.DeepEqual(before, after) {
		t.Fatal("UUID payload conflict mutated committed storage")
	}
}

func TestTwoPageSoilCreationCancellationAndCompletionRecovery(t *testing.T) {
	for _, form := range []string{"SoilHumusXL", "SoilMineralXL"} {
		for _, committed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/committed-%v", form, committed), func(t *testing.T) {
				service, state, db, request := soilCreationWriteFixture(t, true, form, 3)
				before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
				raw := soilAttemptJSON(t, request)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				var prior func(*sql.Tx) error
				var completion func(*twoPageSoilWriteResult) error
				count := soilWriteCount(t, db, request.Original.Table)
				if committed {
					completion = func(result *twoPageSoilWriteResult) error {
						if result == nil || !result.DidCommit || soilWriteCount(t, db, request.Original.Table) != count+1 {
							t.Fatal("completion observer lost independent committed creation")
						}
						cancel()
						return ctx.Err()
					}
				} else {
					prior = func(*sql.Tx) error {
						cancel()
						return ctx.Err()
					}
				}
				result, err := service.dispatchTwoPageSoilObserved(ctx, state.ContextID, raw, true, prior, completion)
				if !errors.Is(err, context.Canceled) || !committed && result != nil ||
					committed && (result == nil || !result.DidCommit) {
					t.Fatal("creation completion confused physical commit boundary", result, err)
				}
				readOnly := soilFacade(t, service.contexts, map[string]string{twoPageSoilReviewEnvironment: "true"})
				outcome, err := readOnly.RecoverOutcome(context.Background(), state.ContextID, raw)
				if err != nil || outcome == nil {
					t.Fatal("read-only creation recovery failed", outcome, err)
				}
				if committed {
					if outcome.Status != "committed" || outcome.Receipt == nil ||
						!reflect.DeepEqual(outcome.Receipt.History, result.History) || soilWriteCount(t, db, request.Original.Table) != count+1 {
						t.Fatal("postcommit cancellation lost immutable creation history", outcome)
					}
				} else {
					if outcome.Status != "notCommitted" || outcome.Receipt != nil {
						t.Fatal("staged creation cancellation invented a commit", outcome)
					}
					if after := databaseBytes(t, service.contexts.projects.sqlite.attachments); !reflect.DeepEqual(before, after) {
						t.Fatal("cancelled creation leaked physical rows/audits/history/reservations")
					}
					if result, err := service.CreateHorizon(context.Background(), state.ContextID, raw); err == nil || result != nil {
						t.Fatal("certified absent creation replayed a sealed UUID")
					}
					request.RequestID = "f4b1aa00-1234-4123-8123-000000000002"
					if result, err := service.CreateHorizon(context.Background(), state.ContextID, soilAttemptJSON(t, request)); err != nil ||
						result == nil || !result.DidCommit {
						t.Fatal("corrected explicit new UUID could not create", result, err)
					}
				}
			})
		}
	}
}

func TestTwoPageSoilCreationStaleOriginalRefusalAndRetry(t *testing.T) {
	for _, form := range []string{"SoilHumusXL", "SoilMineralXL"} {
		t.Run(form, func(t *testing.T) {
			service, state, db, request := soilCreationWriteFixture(t, false, form, 3)
			if _, err := db.Exec("UPDATE " + quoteHeaderIdentifier(request.Original.Table) + " SET Horizon='changed' WHERE ID=2"); err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
			if result, err := service.CreateHorizon(context.Background(), state.ContextID, soilAttemptJSON(t, request)); err == nil || result != nil {
				t.Fatal("creation accepted a changed complete owned original", result, err)
			}
			if after := databaseBytes(t, service.contexts.projects.sqlite.attachments); !reflect.DeepEqual(before, after) {
				t.Fatal("stale creation changed independent physical storage")
			}
			if _, err := db.Exec("UPDATE " + quoteHeaderIdentifier(request.Original.Table) + " SET Horizon='old' WHERE ID=2"); err != nil {
				t.Fatal(err)
			}
			if result, err := service.CreateHorizon(context.Background(), state.ContextID, soilAttemptJSON(t, request)); err != nil ||
				result == nil || !result.DidCommit {
				t.Fatal("exact source restoration could not retry unsealed creation", result, err)
			}
		})
	}
}

func soilCreationOriginal(form string) *twoPageSoilOriginal {
	kind := "Humus"
	names := []string{"PlotNumber", "Horizon", "UpperDepth", "LowerDepth", "HumusStructureDegree",
		"HumusStructureKind", "MycelAbundance", "FecalAbundance", "RootsAbundance", "RootsSize",
		"vonPost", "HumusFormpH", "Consistence", "Character", "Fauna", "Comment", "Flag", "ID"}
	if form == "SoilMineralXL" {
		kind = "Mineral"
		names = []string{"PlotNumber", "Horizon", "UpperDepth", "LowerDepth", "PitDepthLimit", "Colour",
			"ASP", "Texture", "PercentCoarseFragsGravel", "PercentCoarseFragsCobbles",
			"PercentCoarseFragsStones", "PercentCoarseFragsTotal", "PercentCoarseFragsShape",
			"RootsAbundance", "RootsSize", "MineralStructureClass", "MineralStructureKind", "MineralFormpH",
			"MottlesAbundance", "MottlesSize", "MottlesContrast", "ClayFilmsFreq", "ClayFilmThickness",
			"Effervescence", "Porosity", "Comments", "Flag", "ID"}
	}
	original := &twoPageSoilOriginal{ContextID: "synthetic-context", Project: "Draft", Plot: "00017",
		Form: form, RecordSource: "Usys" + kind, Table: "Draft_" + kind,
		Records: ProjectMetadataTable{Columns: []ProjectMetadataColumn{}, Rows: []ProjectMetadataRow{}}}
	for _, name := range names {
		original.Records.Columns = append(original.Records.Columns, ProjectMetadataColumn{Name: name, DeclaredType: "TEXT"})
	}
	return original
}

func TestTwoPageSoilCreationCompleteDefaults(t *testing.T) {
	for _, form := range []string{"SoilHumusXL", "SoilMineralXL"} {
		t.Run(form, func(t *testing.T) {
			original := soilCreationOriginal(form)
			for _, id := range []int64{-2147483648, -1, 0, 1, 2147483647} {
				cells, err := planTwoPageSoilCreation(context.Background(), original, id, " Oh ")
				if err != nil {
					t.Fatal(err)
				}
				if len(cells) != len(original.Records.Columns) {
					t.Fatalf("incomplete cells: %d", len(cells))
				}
				for index, column := range original.Records.Columns {
					cell := cells[index]
					switch column.Name {
					case "PlotNumber":
						if cell.Storage != "text" || cell.Text == nil || *cell.Text != "00017" {
							t.Fatalf("literal parent changed: %#v", cell)
						}
					case "Horizon":
						if cell.Storage != "text" || cell.Text == nil || *cell.Text != " Oh " {
							t.Fatalf("Horizon was trimmed or recased: %#v", cell)
						}
					case "ID":
						value, err := metadataCellValue(cell)
						if err != nil || value != id {
							t.Fatalf("signed identity changed: %v %v", value, err)
						}
					case "Flag":
						if cell.Storage != "integer" || cell.Integer == nil || *cell.Integer != "0" {
							t.Fatalf("native False default was not canonical integer0: %#v", cell)
						}
					default:
						if !reflect.DeepEqual(cell, ProjectMetadataCell{Storage: "null"}) {
							t.Fatalf("invented %s default: %#v", column.Name, cell)
						}
					}
				}
			}
		})
	}
}

func TestTwoPageSoilCreationLiteralBounds(t *testing.T) {
	for _, form := range []string{"SoilHumusXL", "SoilMineralXL"} {
		for _, test := range []struct {
			name, plot, horizon string
			valid               bool
		}{
			{"ascii-bound", "1234567", "12345678", true},
			{"supplementary-bound", "A\U0001f332\U0001f332\U0001f332", "\U0001f332\U0001f332\U0001f332\U0001f332", true},
			{"no-trim-case", " P1 ", " oh ", true},
			{"empty-parent", "", "Oh", false},
			{"parent-overlength", "12345678", "Oh", false},
			{"parent-supplementary-overlength", "\U0001f332\U0001f332\U0001f332\U0001f332", "Oh", false},
			{"empty-horizon", "P1", "", false},
			{"horizon-overlength", "P1", "123456789", false},
			{"horizon-supplementary-overlength", "P1", "\U0001f332\U0001f332\U0001f332\U0001f332A", false},
			{"malformed-parent", string([]byte{0xff}), "Oh", false},
			{"malformed-horizon", "P1", string([]byte{0xff}), false},
		} {
			t.Run(form+"/"+test.name, func(t *testing.T) {
				original := soilCreationOriginal(form)
				original.Plot = test.plot
				_, err := planTwoPageSoilCreation(context.Background(), original, 42, test.horizon)
				if (err == nil) != test.valid {
					t.Fatalf("valid=%v error=%v", test.valid, err)
				}
			})
		}
	}
}

func TestTwoPageSoilCreationScopeAndCompleteSchema(t *testing.T) {
	for _, form := range []string{"SoilHumusXL", "SoilMineralXL"} {
		for name, change := range map[string]func(*twoPageSoilOriginal){
			"context": func(value *twoPageSoilOriginal) { value.ContextID = "" },
			"project": func(value *twoPageSoilOriginal) { value.Project = string([]byte{0xff}) },
			"form":    func(value *twoPageSoilOriginal) { value.Form = "SoilHumus" },
			"source":  func(value *twoPageSoilOriginal) { value.RecordSource = "UsysVeg" },
			"table":   func(value *twoPageSoilOriginal) { value.Table = "Foreign_Humus" },
			"missing": func(value *twoPageSoilOriginal) { value.Records.Columns = value.Records.Columns[1:] },
			"extra": func(value *twoPageSoilOriginal) {
				value.Records.Columns = append(value.Records.Columns, ProjectMetadataColumn{Name: "Extra"})
			},
			"duplicate":      func(value *twoPageSoilOriginal) { value.Records.Columns[1] = value.Records.Columns[0] },
			"replacement":    func(value *twoPageSoilOriginal) { value.Records.Columns[2].Name = "Invented" },
			"malformed-type": func(value *twoPageSoilOriginal) { value.Records.Columns[0].DeclaredType = string([]byte{0xff}) },
		} {
			t.Run(form+"/"+name, func(t *testing.T) {
				original := soilCreationOriginal(form)
				change(original)
				if _, err := planTwoPageSoilCreation(context.Background(), original, 42, "Oh"); err == nil {
					t.Fatal("invalid source original was accepted")
				}
			})
		}
	}
	if _, err := planTwoPageSoilCreation(nil, soilCreationOriginal("SoilHumusXL"), 42, "Oh"); err == nil {
		t.Fatal("nil context accepted")
	}
	if _, err := planTwoPageSoilCreation(context.Background(), nil, 42, "Oh"); err == nil {
		t.Fatal("nil original accepted")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := planTwoPageSoilCreation(cancelled, soilCreationOriginal("SoilHumusXL"), 42, "Oh"); err == nil {
		t.Fatal("cancelled context accepted")
	}
	for _, id := range []int64{-2147483649, 2147483648} {
		if _, err := planTwoPageSoilCreation(context.Background(), soilCreationOriginal("SoilHumusXL"), id, "Oh"); err == nil {
			t.Fatal("out-of-range source identity accepted")
		}
	}
}

func TestTwoPageSoilCreationOriginalRowsAndNoAliasing(t *testing.T) {
	for _, form := range []string{"SoilHumusXL", "SoilMineralXL"} {
		original := soilCreationOriginal(form)
		cells, err := planTwoPageSoilCreation(context.Background(), original, 42, "Oh")
		if err != nil {
			t.Fatal(err)
		}
		original.Records.Rows = []ProjectMetadataRow{{RowID: "-3", Cells: cells}}
		if _, err := planTwoPageSoilCreation(context.Background(), original, 42, "Of"); err == nil {
			t.Fatal("original identity collision accepted")
		}
		planned, err := planTwoPageSoilCreation(context.Background(), original, 43, "Of")
		if err != nil {
			t.Fatal(err)
		}
		for index, column := range original.Records.Columns {
			if column.Name == "PlotNumber" {
				*planned[index].Text = "other"
				if *cells[index].Text != "00017" || original.Plot != "00017" {
					t.Fatal("plan aliased its source parent")
				}
				*cells[index].Text = "foreign"
			}
		}
		if _, err := planTwoPageSoilCreation(context.Background(), original, 44, "Of"); err == nil ||
			!strings.Contains(err.Error(), "literal TEXT parent") {
			t.Fatalf("foreign original parent accepted: %v", err)
		}
		original.Records.Rows[0].Cells = nil
		if _, err := planTwoPageSoilCreation(context.Background(), original, 44, "Of"); err == nil {
			t.Fatal("incomplete physical row accepted")
		}
	}
}
