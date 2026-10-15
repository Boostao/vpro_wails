package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPlotLabelPDFStrictRequests(t *testing.T) {
	valid := PlotLabelPDFExportRequest{"LB001", strings.Repeat("a", 64), `C:\owned\label.pdf`, "00000000-0000-4000-8000-000000000001", "reviewed-service-owner"}
	raw, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	var decoded PlotLabelPDFExportRequest
	if err := json.Unmarshal(raw, &decoded); err != nil || decoded != valid {
		t.Fatalf("strict roundtrip: %+v, %v", decoded, err)
	}
	for _, corrupt := range []string{
		`null`, `[]`, `{}`, string(raw) + `{}`,
		strings.Replace(string(raw), `"plotNumber":"LB001"`, `"plotNumber":"LB001","plotNumber":"foreign"`, 1),
		strings.Replace(string(raw), `"plotNumber":"LB001"`, `"plotNumber":"LB001","\u0070lotNumber":"foreign"`, 1),
		strings.Replace(string(raw), `"plotNumber"`, `"PlotNumber"`, 1),
		strings.Replace(string(raw), `"LB001"`, `null`, 1),
		strings.Replace(string(raw), `"LB001"`, `"\ud800"`, 1),
		strings.Replace(string(raw), `"LB001"`, string([]byte{'"', 0xff, '"'}), 1),
		strings.Replace(string(raw), `"destination"`, `"path"`, 1),
		strings.Replace(string(raw), valid.ApprovalHash, strings.Repeat("A", 64), 1),
		strings.Replace(string(raw), valid.ApprovalHash, strings.Repeat("a", 63), 1),
		strings.Replace(string(raw), valid.RequestID, "not-a-uuid", 1),
	} {
		if err := json.Unmarshal([]byte(corrupt), &decoded); err == nil {
			t.Fatalf("invalid authority accepted: %s", corrupt)
		}
	}
	for _, corrupt := range []string{
		`{"plotNumber":"LB001","extra":false}`, `{"plotNumber":"LB001","plotNumber":"LB002"}`,
		`{"plotNumber":null}`, `{"plotNumber":"\ud800"}`, `{"plotNumber":"LB001"} {}`,
	} {
		var review PlotLabelPDFReviewRequest
		if err := json.Unmarshal([]byte(corrupt), &review); err == nil {
			t.Fatalf("invalid review request accepted: %s", corrupt)
		}
	}
}
