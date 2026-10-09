package main

import (
	"context"
	"encoding/json"
	"testing"
)

func TestProfileRuleLifecycleStrictTransportAndScopedAuthority(t *testing.T) {
	service, state, review := profileRunFixture(t)
	create := blankProfileCreation(review.OriginalRules)
	data, err := json.Marshal(create)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ProjectPlotProfileCreation
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal("explicit nullable creation proposal rejected", err)
	}
	for _, raw := range []string{
		`{"originalRules":{},"values":null}`,
		`{"values":[]}`,
		`{"originalRules":{},"values":[],"unavailable":true}`,
		`{"originalRules":{},"values":[{"column":"Criteria","value":{"storage":"text","text":"\ud800"}}]}`,
	} {
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
			t.Fatal("defaulted/unknown/repaired Unicode creation accepted", raw)
		}
	}
	var deletion ProjectPlotProfileDeletion
	for _, raw := range []string{
		`{"originalRules":{},"rowId":"3"}`,
		`{"originalRules":{},"rowId":null,"confirmed":true}`,
		`{"originalRules":{},"rowId":"3","confirmed":null}`,
		`{"originalRules":{},"rowId":"3","confirmed":true,"unavailable":true}`,
		`{"originalRules":{},"rowId":"\udc00","confirmed":true}`,
	} {
		if err := json.Unmarshal([]byte(raw), &deletion); err == nil {
			t.Fatal("defaulted/unknown/repaired Unicode deletion accepted", raw)
		}
	}
	if _, err := service.CreateProjectPlotProfileRule(context.Background(), "stale", create); err == nil {
		t.Fatal("stale creation context accepted")
	}
	created, err := service.CreateProjectPlotProfileRule(context.Background(), state.ContextID, create)
	if err != nil || created.RowID != "9" {
		t.Fatal("scoped creation authority is not wired", created, err)
	}
	deletion = ProjectPlotProfileDeletion{OriginalRules: profileLifecycleRules(t, service.projects.sqlite),
		RowID: created.RowID, Confirmed: true}
	if err := service.DeleteProjectPlotProfileRule(context.Background(), "stale", deletion); err == nil {
		t.Fatal("stale deletion context accepted")
	}
	if err := service.DeleteProjectPlotProfileRule(context.Background(), state.ContextID, deletion); err != nil {
		t.Fatal("scoped deletion authority is not wired", err)
	}
}
