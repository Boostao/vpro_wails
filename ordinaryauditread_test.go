package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestOrdinaryAuditNullablePresentationAndStrictHistory(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "embedded", true: "external"}[external], func(t *testing.T) {
			service, state, db := sourceAuditFixture(t, external, "TEXT")
			for _, field := range []string{"User", "EditWhen"} {
				for _, value := range []any{nil, "", "historical actor/date"} {
					if _, err := db.Exec(`DELETE FROM Sample_Audit; INSERT INTO Sample_Audit VALUES
						('Sample','actor','108050','_Env','SiteNotes','2020-01-01',NULL,'',-1,0,NULL);
						UPDATE Sample_Audit SET `+quoteHeaderIdentifier(field)+`=?`, value); err != nil {
						t.Fatal(err)
					}
					before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
					got, err := service.contexts.ListAuditReadEntries(context.Background(), state.ContextID, "108050")
					if err != nil || len(got) != 1 {
						t.Fatalf("nullable %s=%v presentation: %+v, %v", field, value, got, err)
					}
					cell := got[0].User
					if field == "EditWhen" {
						cell = got[0].EditWhen
					}
					if value == nil && cell != nil || value != nil && (cell == nil || *cell != value) {
						t.Fatalf("nullable %s=%v became %v", field, value, cell)
					}
					if got[0].ID != nil || got[0].BeforeEdit != nil || got[0].AfterEdit == nil ||
						*got[0].AfterEdit != "" || !got[0].Restore || got[0].Flag {
						t.Fatalf("unchanged ordinary nullable/boolean policy changed: %+v", got[0])
					}
					legacy, err := service.contexts.ListAuditEntries(context.Background(), state.ContextID, "108050")
					if value == nil {
						if err == nil || legacy != nil {
							t.Fatal("strict history reader normalized NULL actor/date")
						}
					} else {
						if err != nil || len(legacy) != 1 {
							t.Fatalf("valid legacy audit changed: %+v, %v", legacy, err)
						}
						oldWire, err := json.Marshal(legacy)
						if err != nil {
							t.Fatal(err)
						}
						newWire, err := json.Marshal(got)
						if err != nil {
							t.Fatal(err)
						}
						if string(oldWire) != string(newWire) {
							t.Fatalf("valid legacy presentation changed: %s / %s", oldWire, newWire)
						}
					}
					if after := databaseBytes(t, service.contexts.projects.sqlite.attachments); !reflect.DeepEqual(before, after) {
						t.Fatal("ordinary nullable read changed physical data")
					}
				}
			}
		})
	}
}

func TestOrdinaryAuditReadErrorsAndCorrection(t *testing.T) {
	service, state, db := sourceAuditFixture(t, false, "TEXT")
	if _, err := db.Exec(`INSERT INTO Sample_Audit VALUES
		('Sample',NULL,'108050','_Env','SiteNotes',NULL,NULL,'',-1,0,NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"User", "EditWhen", "BeforeEdit", "AfterEdit", "Table", "EditField"} {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(`UPDATE Sample_Audit SET ` + quoteHeaderIdentifier(column) + `=CAST(X'FF' AS TEXT)`); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
		got, err := service.contexts.ListAuditReadEntries(context.Background(), state.ContextID, "108050")
		if err == nil || got != nil || !strings.Contains(err.Error(), "malformed UTF-8") {
			t.Fatalf("malformed ordinary %s repaired: %+v, %v", column, got, err)
		}
		if !reflect.DeepEqual(before, databaseBytes(t, service.contexts.projects.sqlite.attachments)) {
			t.Fatal("rejected ordinary read changed source bytes")
		}
		if _, err := db.Exec(`DELETE FROM Sample_Audit; INSERT INTO Sample_Audit VALUES
			('Sample',NULL,'108050','_Env','SiteNotes',NULL,NULL,'',-1,0,NULL)`); err != nil {
			t.Fatal(err)
		}
		if got, err := service.contexts.ListAuditReadEntries(context.Background(), state.ContextID, "108050"); err != nil || len(got) != 1 {
			t.Fatalf("same-context correction retry: %+v, %v", got, err)
		}
	}
	if _, err := db.Exec(`UPDATE Sample_Audit SET Project='Other'`); err != nil {
		t.Fatal(err)
	}
	if got, err := service.contexts.ListAuditReadEntries(context.Background(), state.ContextID, "108050"); err == nil || got != nil {
		t.Fatal("foreign ordinary audit project crossed owner boundary")
	}
}

func TestOrdinaryAuditReadProductionWire(t *testing.T) {
	service, state, db := sourceAuditFixture(t, false, "TEXT")
	if _, err := db.Exec(`INSERT INTO Sample_Audit VALUES
		('Sample',NULL,'108050','_Env','SiteNotes',NULL,NULL,'',-1,0,NULL)`); err != nil {
		t.Fatal(err)
	}
	got, err := service.contexts.ListAuditReadEntries(context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join("frontend", "src", "ordinaryAuditWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var actual, expected any
	if err := json.Unmarshal(data, &actual); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(fixture, &expected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("ordinary nullable wire differs from frontend fixture: %s", data)
	}
}

func TestOrdinaryAuditReadOrderScopeAndUnavailableStrictRestoration(t *testing.T) {
	service, state, db := sourceAuditFixture(t, false, "TEXT")
	if _, err := db.Exec(`INSERT INTO Sample_Audit VALUES
		('Sample',NULL,'108050','_Env','SiteNotes',NULL,NULL,'',0,0,NULL),
		('Sample','','108050','_Env','SiteNotes','2020-01-01','',NULL,-1,0,NULL),
		('Sample','actor','108050','_Env','SiteNotes','2020-01-01',NULL,'',0,-1,NULL),
		('Sample','outside','other','_Env','SiteNotes','2030-01-01',NULL,'',0,0,NULL)`); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	got, err := service.contexts.ListAuditReadEntries(context.Background(), state.ContextID, "108050")
	if err != nil || len(got) != 3 || got[0].RowID != "3" || got[1].RowID != "2" || got[2].RowID != "1" {
		t.Fatalf("ordinary legacy order/filter changed: %+v, %v", got, err)
	}
	if got[2].User != nil || got[2].EditWhen != nil || !got[0].Flag || !got[1].Restore {
		t.Fatalf("nullable/boolean source presentation changed: %+v", got)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if entries, err := selectedAuditEntries(tx, "Sample", "108050", []int64{1}); err == nil || entries != nil {
		t.Fatalf("strict ordinary restoration selected normalized NULL history: %+v, %v", entries, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, service.contexts.projects.sqlite.attachments)) {
		t.Fatal("nullable presentation/strict restoration refusal changed any attachment")
	}
}

func TestOrdinaryAuditReadRejectsStaleContext(t *testing.T) {
	service, _, _ := sourceAuditFixture(t, false, "TEXT")
	before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
	if got, err := service.contexts.ListAuditReadEntries(context.Background(), "retired-context", "108050"); err == nil || got != nil {
		t.Fatalf("nullable presentation read stale context: %+v, %v", got, err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, service.contexts.projects.sqlite.attachments)) {
		t.Fatal("stale ordinary audit request changed source")
	}
}
