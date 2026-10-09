package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestProfileRuleEditScopedServiceAndStrictJSONTransport(t *testing.T) {
	service, state, review := profileRunFixture(t)
	value := "literal"
	request := ProjectPlotProfileEdit{OriginalRules: review.OriginalRules,
		Drafts: []ProjectPlotProfileRuleDraft{{RowID: "3", Changes: []ProjectMetadataChange{
			{Column: "Criteria", Value: ProjectMetadataCell{Storage: "text", Text: &value}},
		}}}}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ProjectPlotProfileEdit
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal("explicit typed snapshot/drafts rejected", err)
	}
	for _, raw := range []string{
		`{"originalRules":null,"drafts":[]}`,
		`{"originalRules":{},"drafts":null}`,
		`{"originalRules":{},"drafts":[{"rowId":"3","changes":null}]}`,
		`{"originalRules":{},"drafts":[{"changes":[]}]}`,
		`{"originalRules":{},"drafts":[],"ignored":true}`,
		strings.Replace(string(data), `"literal"`, `"\ud800"`, 1),
		strings.Replace(string(data), `"literal"`, `"\udc00"`, 1),
		strings.Replace(string(data), `"rowId":"3"`, `"rowId":"3","unknown":true`, 1),
	} {
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
			t.Fatal("malformed/defaulted/repaired Unicode or unknown transport field accepted", raw)
		}
	}
	if err := service.SaveProjectPlotProfile(context.Background(), "stale", request); err == nil {
		t.Fatal("stale profile-edit context accepted")
	}
	if err := service.SaveProjectPlotProfile(context.Background(), state.ContextID, request); err != nil {
		t.Fatal("scoped profile-edit writer authority was not connected", err)
	}
}
