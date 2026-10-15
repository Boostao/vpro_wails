package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func twoPageSpeciesListWriteFixture(t *testing.T, external bool, form string, strength int) (*ContextService, ProjectState, *sql.DB, twoPageVegetationRequest) {
	t.Helper()
	contexts, state, db, request := vegetationWriteFixture(t, external, form, strength)
	mutateContextFixture(t, contexts.projects.sqlite.attachments["VLists"], `DELETE FROM USysAllSpecs;
		INSERT INTO USysAllSpecs(rowid,Code,ScientificName,Lifeform,EnglishName,Codetype,OldCode) VALUES
		(-9,'A',NULL,1,'','U','OLD'),(-8,'A','',4,NULL,'x',NULL),
		(-7,'HISTORY',NULL,2147483647,NULL,'s',NULL)`)
	mutateContextFixture(t, contexts.projects.sqlite.attachments["VUser"], `DELETE FROM USysUserSpp;
		INSERT INTO USysUserSpp(Code,ScientificName,Lifeform,EnglishName,Codetype) VALUES('A','personal',2,NULL,'u')`)
	references, err := contexts.readTwoPageVegetationSpeciesReferences(context.Background(), state.ContextID, request.Original.Plot, form)
	if err != nil {
		t.Fatal(err)
	}
	request.SpeciesList = references
	request.Edits = append([]twoPageVegetationEdit{vegetationPlanEdit(request.Original, "Species", metadataText("A"))}, request.Edits...)
	return contexts, state, db, request
}

func twoPageSpeciesListFacade(t *testing.T, contexts *ContextService, values map[string]string) *TwoPageVegetationSpeciesListService {
	t.Helper()
	service, err := NewTwoPageVegetationSpeciesListService(contexts, func(name string) (string, bool) {
		if name != twoPageVegetationReviewEnvironment && name != twoPageVegetationWritingEnvironment &&
			name != twoPageVegetationSpeciesListEnvironment {
			t.Fatal("list facade consulted another workflow's gate", name)
		}
		value, found := values[name]
		return value, found
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestTwoPageSpeciesListRequestStrictWireAndLegacyShape(t *testing.T) {
	_, _, _, request := twoPageSpeciesListWriteFixture(t, false, "SubVegAXL_BC", 3)
	raw := vegetationAttemptJSON(t, request)
	var got twoPageVegetationRequest
	if err := json.Unmarshal([]byte(raw), &got); err != nil || !reflect.DeepEqual(request, got) {
		t.Fatal("qualified request failed strict round-trip", err)
	}
	for _, altered := range []string{
		strings.Replace(raw, `"speciesList":`, `"speciesList":null,"speciesList":`, 1),
		strings.Replace(raw, `"speciesList":`, `"\u0073peciesList":null,"speciesList":`, 1),
		strings.Replace(raw, `"speciesList":{`, `"speciesList":{"form":"SubVegCXL",`, 1),
		strings.Replace(raw, `"master":{`, `"master":{"rows":[],"rows":[],"extra":0,`, 1),
		strings.Replace(raw, `"declaredType":"TEXT"`, `"declaredType":"TEXT","type":"TEXT"`, 1),
		strings.Replace(raw, `"storage":"text","text":"A"`, `"storage":"text","text":"A","text":"B"`, 1),
		strings.Replace(raw, `"source":"configured`, `"source":"other-configured`, 1),
		strings.Replace(raw, `"personal":{`, `"personal":null,"personal":{`, 1),
		strings.Replace(raw, `"form":"SubVegAXL_BC"`, `"form":"SubVegAXL"`, 1),
	} {
		var decoded twoPageVegetationRequest
		if altered == raw {
			t.Fatal("strict refusal case did not change the production wire")
		}
		if err := json.Unmarshal([]byte(altered), &decoded); err == nil {
			t.Fatal("malformed/ambiguous authority admitted", altered)
		}
	}
	legacy := request
	legacy.SpeciesList, legacy.Edits = nil, legacy.Edits[1:]
	legacyJSON := vegetationAttemptJSON(t, legacy)
	if strings.Contains(legacyJSON, `"speciesList"`) || json.Unmarshal([]byte(legacyJSON), &got) != nil ||
		!reflect.DeepEqual(got, legacy) {
		t.Fatal("omitted legacy request changed shape or inherited new selection authority")
	}
	legacy.Edits = request.Edits
	if err := json.Unmarshal([]byte(vegetationAttemptJSON(t, legacy)), &got); err == nil {
		t.Fatal("legacy envelope admitted Species without physical selection authority")
	}
	request.Edits = request.Edits[1:]
	if err := json.Unmarshal([]byte(vegetationAttemptJSON(t, request)), &got); err == nil {
		t.Fatal("list envelope admitted a cover-only request")
	}
}

func TestTwoPageSpeciesListIndependentGatesAndLegacySaveDenial(t *testing.T) {
	contexts, state, _, request := twoPageSpeciesListWriteFixture(t, false, "SubVegAXL_BC", 3)
	raw := vegetationAttemptJSON(t, request)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	for _, values := range []map[string]string{
		{}, {twoPageVegetationSpeciesListEnvironment: "true"},
		{twoPageVegetationReviewEnvironment: "true", twoPageVegetationSpeciesListEnvironment: "true"},
		{twoPageVegetationReviewEnvironment: "true", twoPageVegetationWritingEnvironment: "true"},
		{twoPageVegetationReviewEnvironment: "true", twoPageVegetationWritingEnvironment: "true", twoPageVegetationSpeciesListEnvironment: "false"},
	} {
		service := twoPageSpeciesListFacade(t, contexts, values)
		if result, err := service.Save(context.Background(), state.ContextID, raw); result != nil || err == nil {
			t.Fatal("partial/default-off gates granted selection", result, err)
		}
	}
	legacy := vegetationFacade(t, contexts, map[string]string{
		twoPageVegetationReviewEnvironment: "true", twoPageVegetationWritingEnvironment: "true",
	})
	if result, err := legacy.Save(context.Background(), state.ContextID, raw); result != nil || err == nil {
		t.Fatal("legacy Save inherited independently gated Species authority", result, err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) ||
		len(contexts.twoPageVegetationAttempts.requests) != 0 {
		t.Fatal("refused gates mutated files or began an attempt")
	}
	service := twoPageSpeciesListFacade(t, contexts, map[string]string{
		twoPageVegetationReviewEnvironment: "true", twoPageVegetationWritingEnvironment: "true", twoPageVegetationSpeciesListEnvironment: "true",
	})
	legacyRequest := request
	legacyRequest.SpeciesList, legacyRequest.Edits = nil, legacyRequest.Edits[1:]
	if result, err := service.Save(context.Background(), state.ContextID, vegetationAttemptJSON(t, legacyRequest)); result != nil || err == nil {
		t.Fatal("selection service accepted a legacy envelope", result, err)
	}
}

func TestTwoPageSpeciesListAtomicAuditsPeersReplayAndReadonlyRecovery(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, form := range []string{"SubVegAXL_BC", "SubVegAXL"} {
			for strength, auditCount := range []int{0, 2, 3, 3} {
				t.Run(fmt.Sprintf("%t/%s/%d", external, form, strength), func(t *testing.T) {
					contexts, state, db, request := twoPageSpeciesListWriteFixture(t, external, form, strength)
					before := databaseBytes(t, contexts.projects.sqlite.attachments)
					priorAudits := soilWriteCount(t, db, "Sample_Audit")
					result, err := contexts.writeTwoPageVegetation(context.Background(), state.ContextID, request)
					if err != nil || result == nil || !result.DidCommit || len(result.History.Audits) != auditCount ||
						!reflect.DeepEqual(result.History.Request, request) ||
						soilWriteCount(t, db, "Sample_Audit") != priorAudits+auditCount ||
						soilWriteCount(t, db, twoPageVegetationHistoryTable) != 1 {
						t.Fatal("atomic source/audit/history count differs", result, err)
					}
					var species, peer, outside string
					var cover float64
					if err := db.QueryRow(`SELECT Species,Cover1 FROM Sample_Veg WHERE rowid=-7`).Scan(&species, &cover); err != nil ||
						species != "A" || cover != 5 {
						t.Fatal("independent physical assignment differs", species, cover, err)
					}
					if err := db.QueryRow(`SELECT Species FROM Sample_Veg WHERE rowid=3`).Scan(&peer); err != nil || peer != "PEER" {
						t.Fatal("duplicate logical ID peer merged", peer, err)
					}
					if err := db.QueryRow(`SELECT Species FROM Sample_Veg WHERE rowid=9`).Scan(&outside); err != nil || outside != "OUTSIDE" {
						t.Fatal("outside-parent row changed", outside, err)
					}
					after := databaseBytes(t, contexts.projects.sqlite.attachments)
					for role, bytes := range before {
						if contexts.projects.sqlite.attachments[role] != contexts.projects.sqlite.attachments["project"] &&
							!reflect.DeepEqual(bytes, after[role]) {
							t.Fatal("reference/config/support file changed", role)
						}
					}
					replayed, err := contexts.writeTwoPageVegetation(context.Background(), state.ContextID, request)
					if err != nil || replayed == nil || !replayed.Replayed || replayed.DidCommit ||
						!reflect.DeepEqual(replayed.History, result.History) || !reflect.DeepEqual(after, databaseBytes(t, contexts.projects.sqlite.attachments)) {
						t.Fatal("once-only exact replay changed source/history", replayed, err)
					}
					mutateContextFixture(t, contexts.projects.sqlite.attachments["VLists"], `UPDATE USysAllSpecs SET ScientificName='later' WHERE rowid=-9`)
					after = databaseBytes(t, contexts.projects.sqlite.attachments)
					readonly := vegetationFacade(t, contexts, map[string]string{twoPageVegetationReviewEnvironment: "true"})
					outcome, err := readonly.RecoverOutcome(context.Background(), state.ContextID, vegetationAttemptJSON(t, request))
					if err != nil || outcome == nil || outcome.Status != "committed" || outcome.Receipt == nil ||
						!reflect.DeepEqual(outcome.Receipt.History, result.History) || !reflect.DeepEqual(after, databaseBytes(t, contexts.projects.sqlite.attachments)) {
						t.Fatal("readonly recovery required current definitions/writing or lost committed receipt", outcome, err)
					}
				})
			}
		}
	}
}

func TestTwoPageSpeciesListReferenceDriftRollbackAndPostcommitRecovery(t *testing.T) {
	for _, role := range []string{"VLists", "VUser"} {
		contexts, state, _, request := twoPageSpeciesListWriteFixture(t, false, "SubVegAXL_BC", 3)
		table := "USysAllSpecs"
		if role == "VUser" {
			table = "USysUserSpp"
		}
		mutateContextFixture(t, contexts.projects.sqlite.attachments[role], `UPDATE `+table+` SET ScientificName='drift' WHERE Code='A'`)
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		result, err := contexts.writeTwoPageVegetation(context.Background(), state.ContextID, request)
		if result != nil || err == nil || !strings.Contains(err.Error(), "definitions changed") ||
			!reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
			t.Fatal("complete reference drift passed or wrote source/history", role, result, err)
		}
	}
	contexts, state, _, request := twoPageSpeciesListWriteFixture(t, true, "SubVegAXL", 3)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	failure := errors.New("planned rollback")
	if result, err := contexts.writeTwoPageVegetationObserved(context.Background(), state.ContextID, request,
		func(*sql.Tx) error { return failure }, nil); result != nil || !errors.Is(err, failure) ||
		!reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
		t.Fatal("fallible precommit hook left data/audits/history", result, err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	if result, err := contexts.writeTwoPageVegetationObserved(canceled, state.ContextID, request,
		func(*sql.Tx) error { cancel(); return nil }, nil); result != nil || !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
		t.Fatal("in-flight cancellation committed a partial selection", result, err)
	}
	service := twoPageSpeciesListFacade(t, contexts, map[string]string{
		twoPageVegetationReviewEnvironment: "true", twoPageVegetationWritingEnvironment: "true", twoPageVegetationSpeciesListEnvironment: "true",
	})
	raw := vegetationAttemptJSON(t, request)
	result, err := service.saveObserved(context.Background(), state.ContextID, raw, nil,
		func(*twoPageVegetationWriteResult) error { return failure })
	if result == nil || !result.DidCommit || !errors.Is(err, failure) {
		t.Fatal("postcommit error erased actual committed result", result, err)
	}
	readonly := vegetationFacade(t, contexts, map[string]string{twoPageVegetationReviewEnvironment: "true"})
	outcome, err := readonly.RecoverOutcome(context.Background(), state.ContextID, raw)
	if err != nil || outcome == nil || outcome.Status != "committed" || outcome.Receipt == nil {
		t.Fatal("postcommit selection outcome could not recover readonly", outcome, err)
	}
}
