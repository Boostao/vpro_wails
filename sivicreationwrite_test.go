package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func siviCreationFixture(t *testing.T, external bool, strength int) (*ContextService, ProjectState, *sql.DB, siviCreationRequest) {
	t.Helper()
	service, state, db, _, _ := siviWriteFixture(t, external, strength)
	mutateContextFixture(t, service.projects.sqlite.attachments["VLists"], `DELETE FROM USysAllSpecs;
		INSERT INTO USysAllSpecs(Code,Lifeform,Codetype,OldCode) VALUES
		('A',3,'U',NULL),('A',3,'U',NULL),('C',12,'x',NULL),('D',9,'u',NULL),
		('NEW',99,'S','old'),('OTHER',3,'U','old')`)
	mutateContextFixture(t, service.projects.sqlite.attachments["VUser"], `DELETE FROM USysUserSpp;
		INSERT INTO USysUserSpp(Code,LifeForm,Codetype) VALUES ('Personal',99,'S'),('old',1,'U')`)
	return service, state, db, siviCreationRequest{RequestID: "creation-1", ContextID: state.ContextID,
		Project: "Sample", Plot: "108050", Form: "SubVegA-SIVI", Species: "A",
		Covers: []siviCreationCover{{"Cover1", siviReal(0)}}}
}

func assertSIVICreationLookupRefused(t *testing.T, service *ContextService, contextID string, request siviCreationRequest) {
	t.Helper()
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if receipt, err := service.lookupSIVICreationReceipt(context.Background(), contextID, request); receipt != nil || err == nil {
		t.Fatal("receipt lookup inferred success from corrupt/conflicting state", receipt, err)
	}
	assertProfileSUFiles(t, service, before)
}

func TestSIVICreationLookupUnresolvedHeldAndLostReceipt(t *testing.T) {
	service, state, db, request := siviCreationFixture(t, true, 3)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	for i := 0; i < 2; i++ {
		if receipt, err := service.lookupSIVICreationReceipt(context.Background(), state.ContextID, request); receipt != nil || err != nil {
			t.Fatal("no history is unresolved, not failed or a mutation retry", receipt, err)
		}
		assertProfileSUFiles(t, service, before)
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(context.Background(), `BEGIN IMMEDIATE;
		CREATE TABLE "__VPRO_SIVICreationHistory"(RequestID TEXT NOT NULL PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL);
		INSERT INTO "__VPRO_SIVICreationHistory" VALUES('creation-1','pending','not committed');
		CREATE TABLE "__VPRO_ChildIdentity"(ChildTable TEXT NOT NULL,ID INTEGER NOT NULL,PRIMARY KEY(ChildTable,ID));
		INSERT INTO "__VPRO_ChildIdentity" VALUES('"Sample_Veg"',1);
		INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1,Flag) VALUES('108050','A',1,0,0)`); err != nil {
		t.Fatal(err)
	}
	receipt, lookupErr := service.lookupSIVICreationReceipt(context.Background(), state.ContextID, request)
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if receipt != nil || lookupErr != nil {
		t.Fatal("held uncommitted request was resolved or failed rather than unresolved", receipt, lookupErr)
	}
	assertProfileSUFiles(t, service, before)
	var technicalTables int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name IN ('__VPRO_SIVICreationHistory','__VPRO_ChildIdentity')`).Scan(&technicalTables); err != nil || technicalTables != 0 {
		t.Fatal("unresolved lookup created durable schema", technicalTables, err)
	}
	initial, err := service.createSIVIVegetation(context.Background(), state.ContextID, request)
	if err != nil || initial == nil || !initial.DidCommit || initial.Replayed {
		t.Fatal("initial complete creation receipt failed", initial, err)
	}
	before = databaseBytes(t, service.projects.sqlite.attachments)
	expected := *initial
	expected.DidCommit, expected.Replayed = false, true
	for i := 0; i < 3; i++ {
		receipt, err = service.lookupSIVICreationReceipt(context.Background(), state.ContextID, request)
		if err != nil || receipt == nil || !reflect.DeepEqual(receipt, &expected) {
			t.Fatal("lost acknowledgement lookup did not recover the complete verified receipt", receipt, err)
		}
		assertProfileSUFiles(t, service, before)
	}
	if receipt.ContextID != state.ContextID || receipt.RequestID != request.RequestID || receipt.HistoryID != request.RequestID ||
		receipt.Project != request.Project || receipt.Plot != request.Plot || receipt.Form != request.Form ||
		receipt.RowID != receipt.Committed.RowID || !reflect.DeepEqual(receipt.Request, request) ||
		!reflect.DeepEqual(receipt.Covers, request.Covers) || receipt.Actor == "" || receipt.AuditStrength != 3 {
		t.Fatal("receipt identities/provenance do not agree with the actual caller and durable request", receipt)
	}
	if _, err := time.Parse("2006-01-02 15:04:05", receipt.EditWhen); err != nil {
		t.Fatal(err)
	}
	if len(receipt.Columns) != 44 || len(receipt.Original.Cells) != 44 || len(receipt.Committed.Cells) != 44 || receipt.Original.RowID != "" {
		t.Fatal("receipt did not return the complete typed source-owned buffer and committed row")
	}
	for i, column := range receipt.Columns {
		original, committed := ProjectMetadataCell{Storage: "null"}, ProjectMetadataCell{Storage: "null"}
		switch column.Name {
		case "Flag":
			original, committed = metadataInteger("0"), metadataInteger("0")
		case "ID":
			committed = metadataInteger(strconv.FormatInt(receipt.ID, 10))
		case "Species":
			committed = metadataText(request.Species)
		case "PlotNumber":
			committed = metadataText(request.Plot)
		case "Cover1":
			committed = siviReal(0)
		}
		if !reflect.DeepEqual(receipt.Original.Cells[i], original) || !reflect.DeepEqual(receipt.Committed.Cells[i], committed) {
			t.Fatal("receipt independently verified44-field plan differs", column.Name, receipt.Original.Cells[i], receipt.Committed.Cells[i])
		}
	}
	data, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		t.Fatal(err)
	}
	keys := []string{"requestId", "contextId", "project", "plot", "form", "rowId", "historyId", "id",
		"actor", "auditStrength", "editWhen", "columns", "original", "committed", "covers", "request", "didCommit", "replayed"}
	if len(properties) != len(keys) {
		t.Fatal("receipt wire shape has implicit or missing properties", string(data))
	}
	for _, key := range keys {
		if properties[key] == nil {
			t.Fatal("receipt requires explicit lower-camel property", key)
		}
	}
	unknown := request
	unknown.RequestID = "unresolved-other-request"
	if receipt, err := service.lookupSIVICreationReceipt(context.Background(), state.ContextID, unknown); receipt != nil || err != nil {
		t.Fatal("unmatched durable request is not unresolved", receipt, err)
	}
	assertProfileSUFiles(t, service, before)
	conflict := request
	conflict.Covers = []siviCreationCover{{"Cover1", siviReal(1)}}
	assertSIVICreationLookupRefused(t, service, state.ContextID, conflict)
}

func TestSIVICreationLookupCancellationDoesNotRetryMutation(t *testing.T) {
	service, state, _, request := siviCreationFixture(t, false, 3)
	before := databaseBytes(t, service.projects.sqlite.attachments)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if receipt, err := service.lookupSIVICreationReceipt(ctx, state.ContextID, request); receipt != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled receipt lookup returned a result", receipt, err)
	}
	owner := service.projects.sqlite
	owner.mu.Lock()
	ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
	receipt, err := service.lookupSIVICreationReceipt(ctx, state.ContextID, request)
	cancel()
	owner.mu.Unlock()
	if receipt != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("held owner lookup ignored cancellation", receipt, err)
	}
	assertProfileSUFiles(t, service, before)
	if receipt, err := service.lookupSIVICreationReceipt(context.Background(), state.ContextID, request); receipt != nil || err != nil {
		t.Fatal("read-only retry changed unresolved to a write/error", receipt, err)
	}
	assertProfileSUFiles(t, service, before)
}

func TestSIVICreationTwoProjectsShareLibraryHistoryWithoutIdentityReuse(t *testing.T) {
	service, state, db, firstRequest := siviCreationFixture(t, false, 3)
	for _, suffix := range coreTables {
		if _, err := db.Exec(`CREATE TABLE ` + quoteHeaderIdentifier("Second_"+suffix) +
			` AS SELECT * FROM ` + quoteHeaderIdentifier("Sample_"+suffix)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO _table_metadata(table_name,description) VALUES('Second_Env','VP08')`); err != nil {
		t.Fatal(err)
	}
	first, err := service.createSIVIVegetation(context.Background(), state.ContextID, firstRequest)
	if err != nil || first == nil {
		t.Fatal("first family creation failed", first, err)
	}
	selection := contextSelection(state)
	selection.Project = "Second"
	secondState, err := service.SwitchContext(state.ContextID, selection)
	if err != nil || secondState.ProjectPath != state.ProjectPath {
		t.Fatal("second family did not share the same physical library", secondState, err)
	}
	secondRequest := firstRequest
	secondRequest.ContextID, secondRequest.Project, secondRequest.RequestID = secondState.ContextID, "Second", "second-creation"
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if receipt, err := service.lookupSIVICreationReceipt(context.Background(), secondState.ContextID, secondRequest); receipt != nil || err != nil {
		t.Fatal("another family's valid history blocked unresolved lookup", receipt, err)
	}
	assertProfileSUFiles(t, service, before)
	second, err := service.createSIVIVegetation(context.Background(), secondState.ContextID, secondRequest)
	if err != nil || second == nil || second.Project != "Second" {
		t.Fatal("another family's valid history blocked independent creation", second, err)
	}
	if second.ID != first.ID {
		t.Fatal("project-owned application identity unexpectedly became library-global", first.ID, second.ID)
	}
	before = databaseBytes(t, service.projects.sqlite.attachments)
	expectedSecond := *second
	expectedSecond.DidCommit, expectedSecond.Replayed = false, true
	receipt, err := service.lookupSIVICreationReceipt(context.Background(), secondState.ContextID, secondRequest)
	if err != nil || receipt == nil || !reflect.DeepEqual(receipt, &expectedSecond) {
		t.Fatal("second family's complete read-only receipt failed", receipt, err)
	}
	assertProfileSUFiles(t, service, before)
	reused := secondRequest
	reused.RequestID = firstRequest.RequestID
	assertSIVICreationLookupRefused(t, service, secondState.ContextID, reused)
	if receipt, err := service.createSIVIVegetation(context.Background(), secondState.ContextID, reused); receipt != nil || err == nil {
		t.Fatal("cross-project request identity reuse mutated or replayed another family", receipt, err)
	}
	assertProfileSUFiles(t, service, before)
	reopened, err := service.SwitchContext(secondState.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	firstRequest.ContextID = reopened.ContextID
	before = databaseBytes(t, service.projects.sqlite.attachments)
	expectedFirst := *first
	expectedFirst.ContextID, expectedFirst.DidCommit, expectedFirst.Replayed = reopened.ContextID, false, true
	receipt, err = service.lookupSIVICreationReceipt(context.Background(), reopened.ContextID, firstRequest)
	if err != nil || receipt == nil || !reflect.DeepEqual(receipt, &expectedFirst) {
		t.Fatal("second family's valid history blocked the first family's read-only receipt", receipt, err)
	}
	assertProfileSUFiles(t, service, before)
	reused = firstRequest
	reused.RequestID = secondRequest.RequestID
	assertSIVICreationLookupRefused(t, service, reopened.ContextID, reused)
	if receipt, err := service.createSIVIVegetation(context.Background(), reopened.ContextID, reused); receipt != nil || err == nil {
		t.Fatal("reverse cross-project request identity reuse mutated or replayed another family", receipt, err)
	}
	assertProfileSUFiles(t, service, before)
	var proposal string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVICreationHistory" WHERE RequestID=?`, secondRequest.RequestID).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	var corrupt siviCreationHistory
	if err := json.Unmarshal([]byte(proposal), &corrupt); err != nil {
		t.Fatal(err)
	}
	corrupt.Request.Project = "Sample"
	broken, err := json.Marshal(corrupt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE "__VPRO_SIVICreationHistory" SET Proposal=? WHERE RequestID=?`, string(broken), secondRequest.RequestID); err != nil {
		t.Fatal(err)
	}
	assertSIVICreationLookupRefused(t, service, reopened.ContextID, firstRequest)
}

func TestSIVICreationAll21SourceFieldsAndNumericDomains(t *testing.T) {
	service, _, db, request := siviCreationFixture(t, false, 3)
	veg, err := readSQLiteStorageRows(context.Background(), db, "main", "Sample_Veg", "", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	forms := map[string][]string{
		"SubVegA-SIVI":    {"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c", "TotalB"},
		"SubVegA-SIVI_BC": {"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB"},
		"SubVegC-SIVI":    {"Cover6"},
		"SubVegD-SIVI":    {"Cover7", "Cover8", "Cover9"},
	}
	count := 0
	before := databaseBytes(t, service.projects.sqlite.attachments)
	for form, fields := range forms {
		for _, field := range fields {
			count++
			for _, value := range []float64{0, -1, 99.999, -math.MaxFloat32} {
				t.Run(form+"/"+field+"/"+strconv.FormatFloat(value, 'g', -1, 64), func(t *testing.T) {
					draft := request
					draft.Form, draft.Covers = form, []siviCreationCover{{field, siviReal(value)}}
					if err := validateSIVICreationRequest(draft); err != nil {
						t.Fatal(err)
					}
					original, created, err := planSIVICreation(context.Background(), draft, veg, math.MaxInt32)
					if err != nil {
						t.Fatal(err)
					}
					columns, err := siteUnitTransferColumns(veg, siviCreationColumns...)
					if err != nil {
						t.Fatal(err)
					}
					for name, index := range columns {
						expected := ProjectMetadataCell{Storage: "null"}
						switch name {
						case "Flag":
							expected = metadataInteger("0")
						case "PlotNumber":
							expected = metadataText(draft.Plot)
						case "Species":
							expected = metadataText(draft.Species)
						case "ID":
							expected = metadataInteger("2147483647")
						case field:
							expected = siviReal(value)
						}
						if !reflect.DeepEqual(created.Cells[index], expected) {
							t.Fatal("unrequested value/default was assigned", name, created.Cells[index])
						}
						if name != "Flag" && original.Cells[index].Storage != "null" {
							t.Fatal("original buffer inferred an assignment", name)
						}
					}
				})
			}
		}
	}
	if count != 21 {
		t.Fatal("wrong exact form/field combination count", count)
	}
	for _, column := range []string{"Cover5a", "Cover6", "HeightA", "Layer", "ID", "cover1"} {
		draft := request
		draft.Form = "SubVegA-SIVI_BC"
		draft.Covers = []siviCreationCover{{column, siviReal(0)}}
		if _, _, err := planSIVICreation(context.Background(), draft, veg, 1); err == nil {
			t.Fatal("hidden/foreign/non-cover field accepted", column)
		}
	}
	for _, value := range []ProjectMetadataCell{{Storage: "null"}, metadataInteger("0"), metadataText("0"), siviReal(100), siviReal(math.Inf(1)), siviReal(math.NaN()), siviReal(math.MaxFloat64)} {
		draft := request
		draft.Covers = []siviCreationCover{{"Cover1", value}}
		if err := validateSIVICreationRequest(draft); err == nil {
			t.Fatal("invalid typed domain accepted", value)
		}
	}
	request.Covers = nil
	if err := validateSIVICreationRequest(request); err == nil {
		t.Fatal("species-only row accepted")
	}
	assertProfileSUFiles(t, service, before)
}

func TestSIVICreationStrictRawJSON(t *testing.T) {
	request := siviCreationRequest{RequestID: "r", ContextID: "c", Project: "Sample", Plot: "108050",
		Form: "SubVegC-SIVI", Species: "C", Covers: []siviCreationCover{{"Cover6", siviReal(0)}}}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	valid := string(data)
	var decoded siviCreationRequest
	if err := json.Unmarshal(data, &decoded); err != nil || !reflect.DeepEqual(request, decoded) {
		t.Fatal("strict request roundtrip", err)
	}
	for _, invalid := range []string{
		strings.Replace(valid, `"species":"C"`, `"species":"\ud800"`, 1),
		strings.Replace(valid, `"species":"C"`, "\"species\":\"\xff\"", 1),
		strings.Replace(valid, `"species":"C"`, `"species":"C","species":"D"`, 1),
		strings.Replace(valid, `"species":"C"`, `"Species":"C"`, 1),
		strings.Replace(valid, `"species":"C"`, `"species":null`, 1),
		strings.Replace(valid, `"species":"C"`, `"species":"123456789"`, 1),
		strings.Replace(valid, `"plot":"108050"`, `"plot":"12345678"`, 1),
		strings.Replace(valid, `"covers":`, `"height":0,"covers":`, 1),
		strings.Replace(valid, `"column":"Cover6"`, `"column":"Cover6","column":"Cover7"`, 1),
		strings.Replace(valid, `"storage":"real"`, `"storage":"real","storage":"null"`, 1),
		strings.Replace(valid, `"species":"C"`, `"species":"C","decision":{"kind":"keep","entered":"\udc00"}`, 1),
		strings.Replace(valid, `"requestId":"r"`, `"requestId":""`, 1),
		valid + `{}`,
	} {
		if invalid == valid {
			t.Fatal("test did not alter request")
		}
		var draft siviCreationRequest
		err := json.Unmarshal([]byte(invalid), &draft)
		if err == nil {
			err = validateSIVICreationRequest(draft)
		}
		if err == nil {
			t.Fatal("malformed authority accepted", invalid)
		}
	}
}

func TestSIVICreationOwnedFormsHistoryAndMutationFreeReplay(t *testing.T) {
	for strength := 0; strength <= 3; strength++ {
		t.Run(strconv.Itoa(strength), func(t *testing.T) {
			service, state, db, request := siviCreationFixture(t, strength%2 == 0, strength)
			for i, form := range []string{"SubVegA-SIVI", "SubVegA-SIVI_BC", "SubVegC-SIVI", "SubVegD-SIVI"} {
				request.RequestID, request.Form = "request-"+strconv.Itoa(i), form
				request.Species = []string{"A", "A", "C", "D"}[i]
				request.Covers = []siviCreationCover{{[]string{"Cover5a", "TotalB", "Cover6", "Cover9"}[i], siviReal(0)}}
				result, err := service.createSIVIVegetation(context.Background(), state.ContextID, request)
				if err != nil || result == nil || result.ID <= 0 || result.ID > math.MaxInt32 ||
					result.RequestID != request.RequestID || result.HistoryID != request.RequestID {
					t.Fatal("private creation failed", result, err)
				}
				before := databaseBytes(t, service.projects.sqlite.attachments)
				replay, err := service.createSIVIVegetation(context.Background(), state.ContextID, request)
				expectedReplay := *result
				expectedReplay.DidCommit, expectedReplay.Replayed = false, true
				if err != nil || replay == nil || !reflect.DeepEqual(&expectedReplay, replay) {
					t.Fatal("verified durable replay failed", result, replay, err)
				}
				assertProfileSUFiles(t, service, before)
				changed := request
				changed.Covers = []siviCreationCover{{request.Covers[0].Column, siviReal(1)}}
				if result, err := service.createSIVIVegetation(context.Background(), state.ContextID, changed); err == nil || result != nil {
					t.Fatal("same request-ID acquired different authority", result, err)
				}
				assertProfileSUFiles(t, service, before)
				var flag int
				var missing int
				if err := db.QueryRow(`SELECT CAST(Flag AS INTEGER), (HeightA IS NULL AND HeightB IS NULL AND Height6 IS NULL AND Layer IS NULL AND Collected IS NULL AND Cover10 IS NULL)
					FROM Sample_Veg WHERE rowid=?`, result.RowID).Scan(&flag, &missing); err != nil || flag != 0 || missing != 1 {
					t.Fatal("writer inferred hidden defaults", flag, missing, err)
				}
			}
		})
	}
}

func TestSIVICreationSpeciesDecisionsAndExactOwnership(t *testing.T) {
	service, state, _, request := siviCreationFixture(t, false, 3)
	selected := "NEW"
	personal := "Personal"
	for i, draft := range []siviCreationRequest{
		func() siviCreationRequest {
			r := request
			r.Species = "OLD"
			r.Decision = &siviCreationDecision{Kind: "keep", Entered: "old"}
			return r
		}(),
		func() siviCreationRequest {
			r := request
			r.Species = "NEW"
			r.Decision = &siviCreationDecision{Kind: "replace", Entered: "old", Selected: &selected}
			return r
		}(),
		func() siviCreationRequest {
			r := request
			r.Species = "PERSONAL"
			r.Decision = &siviCreationDecision{Kind: "user", Entered: "personal", Selected: &personal}
			return r
		}(),
	} {
		draft.RequestID = "decision-" + strconv.Itoa(i)
		if result, err := service.createSIVIVegetation(context.Background(), state.ContextID, draft); err != nil || result == nil {
			t.Fatal("reviewed source decision failed", result, err)
		}
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	for _, mutate := range []func(*siviCreationRequest){
		func(r *siviCreationRequest) { r.Species = "UNKNOWN" },
		func(r *siviCreationRequest) { r.Species = "C" },
		func(r *siviCreationRequest) { r.ContextID = "stale" },
		func(r *siviCreationRequest) { r.Project = "Other" },
		func(r *siviCreationRequest) { r.Plot = "outside" },
		func(r *siviCreationRequest) { r.Plot = "108050 " },
		func(r *siviCreationRequest) { r.Species = "Personal" },
		func(r *siviCreationRequest) { r.Decision = &siviCreationDecision{Kind: "keep", Entered: "A"} },
		func(r *siviCreationRequest) {
			r.Species = "OLD"
			v := "old"
			r.Decision = &siviCreationDecision{Kind: "user", Entered: "old", Selected: &v}
		},
	} {
		draft := request
		mutate(&draft)
		if result, err := service.createSIVIVegetation(context.Background(), state.ContextID, draft); err == nil || result != nil {
			t.Fatal("invalid species/owner accepted", result, err)
		}
		assertProfileSUFiles(t, service, before)
	}
	owner := service.projects.sqlite
	path := owner.attachments["VLists"]
	owner.attachments["VLists"] = owner.attachments["project"]
	if result, err := service.createSIVIVegetation(context.Background(), state.ContextID, request); err == nil || result != nil {
		t.Fatal("support/project alias granted a write", result, err)
	}
	owner.attachments["VLists"] = path
	assertProfileSUFiles(t, service, before)
}

func TestSIVICreationAllocationDeletedReservationsAndNoMatchingValueInference(t *testing.T) {
	service, state, db, request := siviCreationFixture(t, false, 0)
	if _, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover1) VALUES('108050','A',1,0);
		INSERT INTO Sample_Audit("Table",ID) VALUES('_vEg',2),('SAMPLE_VEG',3);
		CREATE TABLE "__VPRO_ChildIdentity"(ChildTable TEXT NOT NULL,ID INTEGER NOT NULL,PRIMARY KEY(ChildTable,ID));
		INSERT INTO "__VPRO_ChildIdentity" VALUES('"Sample_Veg"',4),('"Other_Veg"',5)`); err != nil {
		t.Fatal(err)
	}
	result, err := service.createSIVIVegetation(context.Background(), state.ContextID, request)
	if err != nil || result == nil || result.ID != 5 {
		t.Fatal("allocator reassigned a colliding/deleted/reserved identity", result, err)
	}
	request.RequestID = "distinct-explicit-request"
	next, err := service.createSIVIVegetation(context.Background(), state.ContextID, request)
	if err != nil || next == nil || next.ID == result.ID || next.RowID == result.RowID {
		t.Fatal("matching-value row was inferred as a request replay", next, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "__VPRO_ChildIdentity" WHERE ChildTable='"Sample_Veg"' AND ID IN (2,3,4,5,6)`).Scan(&count); err != nil || count != 5 {
		t.Fatal("audit deletion reservations were not retained", count, err)
	}
}

type siviCreationJournalCancellation struct {
	context.Context
	journal string
	cancel  context.CancelFunc
	once    sync.Once
	staged  bool
}

func (ctx *siviCreationJournalCancellation) Err() error {
	if info, err := os.Stat(ctx.journal); err == nil && info.Size() > 0 {
		ctx.once.Do(func() {
			ctx.staged = true
			ctx.cancel()
		})
	}
	return ctx.Context.Err()
}

func TestSIVICreationRollbackCancellationAndRetry(t *testing.T) {
	service, state, db, request := siviCreationFixture(t, true, 3)
	for _, trigger := range []string{
		`CREATE TRIGGER fail_creation AFTER INSERT ON Sample_Veg BEGIN UPDATE Sample_Veg SET HeightA=1 WHERE rowid=NEW.rowid; END`,
		`CREATE TRIGGER fail_creation AFTER INSERT ON Sample_Veg BEGIN DELETE FROM Sample_Admin; END`,
		`CREATE TRIGGER fail_creation AFTER INSERT ON Sample_Audit BEGIN UPDATE Sample_Audit SET AfterEdit='WRONG' WHERE rowid=NEW.rowid; END`,
		`CREATE TRIGGER fail_creation BEFORE INSERT ON Sample_Audit BEGIN SELECT RAISE(ABORT,'audit rejected'); END`,
	} {
		if _, err := db.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		if result, err := service.createSIVIVegetation(context.Background(), state.ContextID, request); err == nil || result != nil {
			t.Fatal("trigger drift committed", result, err)
		}
		assertProfileSUFiles(t, service, before)
		if _, err := db.Exec(`DROP TRIGGER fail_creation`); err != nil {
			t.Fatal(err)
		}
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.createSIVIVegetation(ctx, state.ContextID, request); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled request returned success", result, err)
	}
	assertProfileSUFiles(t, service, before)
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
	result, err := service.createSIVIVegetation(ctx, state.ContextID, request)
	cancel()
	if _, rollbackErr := conn.ExecContext(context.Background(), "ROLLBACK"); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
	if closeErr := conn.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if result != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("writer-wait cancellation returned success", result, err)
	}
	assertProfileSUFiles(t, service, before)
	ctx, cancel = context.WithCancel(context.Background())
	staged := &siviCreationJournalCancellation{Context: ctx, journal: service.projects.sqlite.attachments["project"] + "-journal", cancel: cancel}
	result, err = service.createSIVIVegetation(staged, state.ContextID, request)
	cancel()
	if result != nil || !errors.Is(err, context.Canceled) || !staged.staged {
		t.Fatal("cancellation did not exercise a staged database mutation", result, err, staged.staged)
	}
	assertProfileSUFiles(t, service, before)
	result, err = service.createSIVIVegetation(context.Background(), state.ContextID, request)
	if err != nil || result == nil || result.ID != 1 {
		t.Fatal("rollback/cancel consumed an identity or blocked retry", result, err)
	}
}

func TestSIVICreationFreshParentsSchemaAndReservationCorruption(t *testing.T) {
	service, state, db, request := siviCreationFixture(t, false, 3)
	for _, statements := range []struct{ breakSQL, repairSQL string }{
		{`UPDATE Sample_Admin SET Plot='other' WHERE Plot='108050'`, `UPDATE Sample_Admin SET Plot='108050' WHERE Plot='other'`},
		{`UPDATE Sample_Env SET PlotNumber='other' WHERE PlotNumber='108050'`, `UPDATE Sample_Env SET PlotNumber='108050' WHERE PlotNumber='other'`},
		{`UPDATE Sample_Admin SET Plot=CAST(Plot AS BLOB) WHERE Plot='108050'`, `UPDATE Sample_Admin SET Plot=CAST(Plot AS TEXT) WHERE typeof(Plot)='blob'`},
		{`INSERT INTO Sample_Audit("Table",ID) VALUES('_Veg',2147483648)`, `DELETE FROM Sample_Audit WHERE ID=2147483648`},
		{`INSERT INTO Sample_Audit("Table",ID) VALUES('_Veg','invalid')`, `DELETE FROM Sample_Audit WHERE ID='invalid'`},
	} {
		if _, err := db.Exec(statements.breakSQL); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		if result, err := service.createSIVIVegetation(context.Background(), state.ContextID, request); err == nil || result != nil {
			t.Fatal("fresh parent/reservation guard was weakened", result, err)
		}
		assertProfileSUFiles(t, service, before)
		if _, err := db.Exec(statements.repairSQL); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`ALTER TABLE Sample_Veg ADD COLUMN Unreviewed TEXT DEFAULT 'implicit'`); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if result, err := service.createSIVIVegetation(context.Background(), state.ContextID, request); err == nil || result != nil {
		t.Fatal("unknown initial column/default was silently accepted", result, err)
	}
	assertProfileSUFiles(t, service, before)
}

func TestSIVICreationDurableReopenedOwnerAndFreshSUMembership(t *testing.T) {
	service, state, db, request := siviCreationFixture(t, true, 3)
	result, err := service.createSIVIVegetation(context.Background(), state.ContextID, request)
	if err != nil || result == nil {
		t.Fatal(result, err)
	}
	reopened, err := service.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil || reopened.ContextID == state.ContextID {
		t.Fatal("same-project owner did not reopen", reopened, err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if replay, err := service.createSIVIVegetation(context.Background(), reopened.ContextID, request); err == nil || replay != nil {
		t.Fatal("old request owner acquired reopened context authority", replay, err)
	}
	request.ContextID = reopened.ContextID
	replay, err := service.createSIVIVegetation(context.Background(), reopened.ContextID, request)
	expectedReplay := *result
	expectedReplay.ContextID, expectedReplay.DidCommit, expectedReplay.Replayed = reopened.ContextID, false, true
	if err != nil || replay == nil || !reflect.DeepEqual(&expectedReplay, replay) {
		t.Fatal("request-ID history was not durable across same-project reopening", result, replay, err)
	}
	lookedUp, err := service.lookupSIVICreationReceipt(context.Background(), reopened.ContextID, request)
	if err != nil || lookedUp == nil || !reflect.DeepEqual(&expectedReplay, lookedUp) ||
		lookedUp.Request.ContextID != state.ContextID || lookedUp.ContextID != reopened.ContextID {
		t.Fatal("receipt fabricated original owner identity after reopening", lookedUp, err)
	}
	assertProfileSUFiles(t, service, before)
	mutateContextFixture(t, reopened.SUPath, `UPDATE Report_SU SET PlotNumber='outside' WHERE PlotNumber='108050'`)
	before = databaseBytes(t, service.projects.sqlite.attachments)
	if replay, err := service.createSIVIVegetation(context.Background(), reopened.ContextID, request); err == nil || replay != nil {
		t.Fatal("durable replay bypassed fresh SU parent membership", replay, err)
	}
	assertProfileSUFiles(t, service, before)
	mutateContextFixture(t, reopened.SUPath, `UPDATE Report_SU SET PlotNumber='108050' WHERE PlotNumber='outside'`)
	if _, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,ID) VALUES('outside','collision',?)`, result.ID); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, service.projects.sqlite.attachments)
	if replay, err := service.createSIVIVegetation(context.Background(), reopened.ContextID, request); err == nil || replay != nil {
		t.Fatal("replay ignored an outside-source identity collision", replay, err)
	}
	assertProfileSUFiles(t, service, before)
}

func TestSIVICreationRequestHistoryCorruption(t *testing.T) {
	service, state, db, request := siviCreationFixture(t, false, 3)
	result, err := service.createSIVIVegetation(context.Background(), state.ContextID, request)
	if err != nil || result == nil {
		t.Fatal(result, err)
	}
	var proposal string
	if err := db.QueryRow(`SELECT Proposal FROM "__VPRO_SIVICreationHistory" WHERE RequestID=?`, request.RequestID).Scan(&proposal); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*siviCreationHistory){
		func(h *siviCreationHistory) { h.Result.ID++ },
		func(h *siviCreationHistory) { h.Result.RequestID = "other" },
		func(h *siviCreationHistory) { h.Result.Project = "Other" },
		func(h *siviCreationHistory) { h.Request.Species = "C" },
		func(h *siviCreationHistory) { h.Original.Cells[0] = metadataText("inferred") },
		func(h *siviCreationHistory) { h.Committed.Cells[0] = metadataText("other") },
		func(h *siviCreationHistory) { h.Actor = "other" },
		func(h *siviCreationHistory) { h.When = "not-a-date" },
		func(h *siviCreationHistory) { h.Audits = nil },
		func(h *siviCreationHistory) { h.Audits[0].ID = nil },
		func(h *siviCreationHistory) { h.AuditStrength = 0 },
	} {
		var history siviCreationHistory
		if err := json.Unmarshal([]byte(proposal), &history); err != nil {
			t.Fatal(err)
		}
		mutate(&history)
		broken, err := json.Marshal(history)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE "__VPRO_SIVICreationHistory" SET Proposal=?`, string(broken)); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		if replay, err := service.createSIVIVegetation(context.Background(), state.ContextID, request); err == nil || replay != nil {
			t.Fatal("corrupt durable history inferred success", replay, err)
		}
		assertSIVICreationLookupRefused(t, service, state.ContextID, request)
		assertProfileSUFiles(t, service, before)
	}
	for _, broken := range []string{
		strings.Replace(proposal, `"Actor":`, `"Unknown":true,"Actor":`, 1),
		strings.Replace(proposal, `"Actor":`, `"Actor":"\ud800","Discard":`, 1),
		strings.Replace(proposal, `"ID":1`, `"ID":1,"ID":2`, 1),
	} {
		if broken == proposal {
			t.Fatal("history corruption did not change bytes")
		}
		if _, err := db.Exec(`UPDATE "__VPRO_SIVICreationHistory" SET Proposal=?`, broken); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		if replay, err := service.createSIVIVegetation(context.Background(), state.ContextID, request); err == nil || replay != nil {
			t.Fatal("raw corrupt history accepted", replay, err)
		}
		assertSIVICreationLookupRefused(t, service, state.ContextID, request)
		assertProfileSUFiles(t, service, before)
	}
	if _, err := db.Exec(`UPDATE "__VPRO_SIVICreationHistory" SET Proposal=?`, proposal); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE "__VPRO_SIVICreationHistory" SET RequestID='damaged-key'`); err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	if replay, err := service.createSIVIVegetation(context.Background(), state.ContextID, request); err == nil || replay != nil {
		t.Fatal("damaged history key allowed a new identity", replay, err)
	}
	assertSIVICreationLookupRefused(t, service, state.ContextID, request)
	assertProfileSUFiles(t, service, before)
	if _, err := db.Exec(`UPDATE "__VPRO_SIVICreationHistory" SET RequestID=?`, request.RequestID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM "__VPRO_ChildIdentity" WHERE ID=?`, result.ID); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, service.projects.sqlite.attachments)
	if replay, err := service.createSIVIVegetation(context.Background(), state.ContextID, request); err == nil || replay != nil {
		t.Fatal("source audits substituted for a missing durable ledger entry", replay, err)
	}
	assertSIVICreationLookupRefused(t, service, state.ContextID, request)
	assertProfileSUFiles(t, service, before)
	if _, err := db.Exec(`INSERT INTO "__VPRO_ChildIdentity" VALUES('"Sample_Veg"',?)`, result.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Veg SET Layer='drift' WHERE rowid=?`, result.RowID); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, service.projects.sqlite.attachments)
	if replay, err := service.createSIVIVegetation(context.Background(), state.ContextID, request); err == nil || replay != nil {
		t.Fatal("replay mutated or ignored committed row drift", replay, err)
	}
	assertSIVICreationLookupRefused(t, service, state.ContextID, request)
	assertProfileSUFiles(t, service, before)
}
