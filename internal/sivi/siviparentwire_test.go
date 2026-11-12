package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestSIVIParentOriginalWireRoundTrip(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	original, err := sourceform.ReadSIVIParent(service, context.Background(), state.ContextID, "108050")
	if err != nil {
		t.Fatal(err)
	}
	if len(original.Rows) != 1 {
		t.Fatal("wire fixture requires one literal physical parent pair")
	}
	files := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	config, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var decoded sourceform.SiviParentProjection
	if err := json.Unmarshal(wire, &decoded); err != nil || !reflect.DeepEqual(original, &decoded) {
		t.Fatal("parent wire normalized raw physical identities or cells", err)
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(wire, &root); err != nil || len(root) != 12 {
		t.Fatal("parent wire must retain its twelve original projection properties", err)
	}
	for _, name := range []string{"ContextID", "Project", "Plot", "Form", "Query", "Membership",
		"EnvTable", "AdminTable", "EnvColumns", "AdminColumns", "Bindings", "Rows"} {
		if len(root[name]) == 0 || string(root[name]) == "null" {
			t.Fatalf("parent original wire lost %s", name)
		}
	}
	vprotest.AssertProfileSUFiles(t, service, files)
	after, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil || !reflect.DeepEqual(config, after) {
		t.Fatal("wire review changed runtime preferences", err)
	}
	if testing.Verbose() {
		owner, err := json.Marshal(map[string]string{
			"contextId": state.ContextID, "project": vprotest.ContextSelection(state).Project, "plot": "108050",
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("SIVI_PARENT_WIRE_OWNER:%s", base64.StdEncoding.EncodeToString(owner))
		t.Logf("SIVI_PARENT_WIRE_ORIGINAL:%s", base64.StdEncoding.EncodeToString(wire))
	}
}
