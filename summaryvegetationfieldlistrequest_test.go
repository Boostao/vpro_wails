package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func summaryFieldListWireFixture(t *testing.T) (SummaryVegetationFieldListRequest, string) {
	t.Helper()
	service, id := summaryFieldListOwnedFixture(t)
	review, err := service.GetSummaryVegetationFieldList(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	request := SummaryVegetationFieldListRequest{
		RequestID: "2e6b14c0-77b2-49e7-8fa5-9d1b4bb6a854",
		Review:    *review,
		Actions:   []SummaryVegetationFieldListAction{{Kind: "all", Selected: true}, {Kind: "row", RowID: "2"}},
	}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return request, string(raw)
}

func TestSummaryVegetationFieldListRequestStrictGoRoundtrip(t *testing.T) {
	request, raw := summaryFieldListWireFixture(t)
	var decoded SummaryVegetationFieldListRequest
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil || !reflect.DeepEqual(decoded, request) {
		t.Fatal("actual Go production wire failed strict roundtrip", decoded, err)
	}
	if request.Review.SU != "None" || request.Review.SUPath != "" {
		t.Fatal("roundtrip must cover actual None scope with no selected SU path", request.Review)
	}
	if !strings.Contains(raw, `"declaredType":"INTEGER"`) || strings.Contains(raw, `"Kind"`) ||
		!strings.Contains(raw, `"kind":"all","rowId":"","selected":true`) {
		t.Fatal("wire differs from intended primitive shapes", raw)
	}
	for _, actions := range [][]SummaryVegetationFieldListAction{
		{}, {{Kind: "none"}}, {{Kind: "row", RowID: "1", Selected: true}},
	} {
		request.Actions = actions
		*request.Review.Physical.Rows[0].Cells[2].Text = strings.Repeat("x", 256) + "\x00"
		raw, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &decoded); err != nil || !reflect.DeepEqual(decoded, request) {
			t.Fatal("historical original cells or explicit empty actions were repaired/defaulted", decoded, err)
		}
	}
}

func TestSummaryVegetationFieldListRequestNestedStrictness(t *testing.T) {
	_, valid := summaryFieldListWireFixture(t)
	tests := []struct{ old, replacement string }{
		{`"requestId":`, `"extra":0,"requestId":`},
		{`"requestId":`, `"requestId":"first","requestId":`},
		{`"requestId":`, `"\u0072equestId":"first","requestId":`},
		{`"requestId":`, `"RequestId":`},
		{`"review":`, `"review":null,"review":`},
		{`"physical":`, `"extra":{},"physical":`},
		{`"contextId":`, `"\u0063ontextId":"replaced","contextId":`},
		{`"contextId":`, `"ContextId":`},
		{`"columns":`, `"columns":null,"columns":`},
		{`"declaredType":"INTEGER"`, `"declaredType":"INTEGER","declaredType":"TEXT"`},
		{`"declaredType":"INTEGER"`, `"\u0064eclaredType":"TEXT","declaredType":"INTEGER"`},
		{`"declaredType":"INTEGER"`, `"type":"INTEGER"`},
		{`"rows":`, `"rows":null,"rows":`},
		{`"rowId":"1"`, `"rowId":"01"`},
		{`"rowId":"1"`, `"rowId":"2"`},
		{`"rowId":"1"`, `"\u0072owId":"2","rowId":"1"`},
		{`"cells":`, `"cells":null,"cells":`},
		{`"storage":"integer"`, `"storage":null`},
		{`"storage":"integer"`, `"storage":"integer","Storage":"integer"`},
		{`"integer":"1"`, `"integer":"1","\u0069nteger":"2"`},
		{`"integer":"1"`, `"integer":1`},
		{`"text":"duplicate"`, `"text":"\ud800"`},
		{`"text":"duplicate"`, `"text":"` + string([]byte{0xff}) + `"`},
		{`"text":"duplicate"`, `"text":"duplicate","unknown":null`},
		{`"kind":"all"`, `"kind":"all","\u006bind":"none"`},
		{`"kind":"all"`, `"Kind":"all"`},
		{`"selected":true`, `"selected":null`},
		{`"selected":true`, `"selected":1`},
		{`"selected":true`, `"selected":true,"selected":false`},
		{`"rowId":"","selected":true`, `"rowId":"1","selected":true`},
	}
	for _, test := range tests {
		if !strings.Contains(valid, test.old) {
			t.Fatal("strict test did not match production wire", test.old)
		}
		invalid := strings.Replace(valid, test.old, test.replacement, 1)
		var decoded SummaryVegetationFieldListRequest
		if err := json.Unmarshal([]byte(invalid), &decoded); err == nil {
			t.Fatal("strict nested request silently defaulted/repaired/replaced authority", invalid)
		}
	}
	for _, invalid := range []string{`{}`, `null`, valid + `{}`} {
		var decoded SummaryVegetationFieldListRequest
		if err := json.Unmarshal([]byte(invalid), &decoded); err == nil {
			t.Fatal("invalid outer request accepted", invalid)
		}
	}
}

func TestSummaryVegetationFieldListRequestMissingNullAndInvalidAuthority(t *testing.T) {
	request, valid := summaryFieldListWireFixture(t)
	var outer map[string]json.RawMessage
	if err := json.Unmarshal([]byte(valid), &outer); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"requestId", "review", "actions"} {
		for _, missing := range []bool{true, false} {
			copy := map[string]json.RawMessage{}
			for name, value := range outer {
				copy[name] = value
			}
			if missing {
				delete(copy, key)
			} else {
				copy[key] = json.RawMessage("null")
			}
			raw, err := json.Marshal(copy)
			if err != nil {
				t.Fatal(err)
			}
			var decoded SummaryVegetationFieldListRequest
			if err := json.Unmarshal(raw, &decoded); err == nil {
				t.Fatal("missing/null request authority accepted", key, missing)
			}
		}
	}
	for _, mutate := range []func(*SummaryVegetationFieldListRequest){
		func(r *SummaryVegetationFieldListRequest) { r.RequestID = strings.ToUpper(r.RequestID) },
		func(r *SummaryVegetationFieldListRequest) { r.RequestID = "2e6b14c0-77b2-19e7-8fa5-9d1b4bb6a854" },
		func(r *SummaryVegetationFieldListRequest) { r.Review.ContextID = "" },
		func(r *SummaryVegetationFieldListRequest) { r.Review.SU = "selected-but-no-path" },
		func(r *SummaryVegetationFieldListRequest) { r.Review.ProjectPath = "path\x00" },
		func(r *SummaryVegetationFieldListRequest) { r.Review.Source = "caller" },
		func(r *SummaryVegetationFieldListRequest) {
			r.Actions = []SummaryVegetationFieldListAction{{Kind: "label", RowID: "1"}}
		},
		func(r *SummaryVegetationFieldListRequest) {
			r.Actions = []SummaryVegetationFieldListAction{{Kind: "row", RowID: "5"}}
		},
	} {
		copy := request
		mutate(&copy)
		raw, err := json.Marshal(copy)
		if err != nil {
			t.Fatal(err)
		}
		var decoded SummaryVegetationFieldListRequest
		if err := json.Unmarshal(raw, &decoded); err == nil {
			t.Fatal("invalid request authority accepted", string(raw))
		}
	}
}
