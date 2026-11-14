package spatial

import (
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestGoogleEarthKMLUnavailableAndCancelledGuards(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, service := range []*GoogleEarthKMLService{nil, {}, NewGoogleEarthKMLService(nil)} {
		if got, err := service.GetGoogleEarthKMLReview(context.Background(), "id", "{}"); got != nil || err == nil {
			t.Fatal("unavailable service returned KML", got, err)
		}
		if got, err := service.GetGoogleEarthKMLReview(ctx, "id", "{}"); got != nil || !errors.Is(err, context.Canceled) {
			t.Fatal("cancelled call lost precedence", got, err)
		}
	}
}

func TestGoogleEarthKMLFacadeOwnedBytesAndStrictRawJSON(t *testing.T) {
	contexts, state := vprotest.PrepareKMLContextFixture(t)
	check := vprotest.AssertPlotLocationFacadeReadOnly(t, contexts)
	defer check()
	service := NewGoogleEarthKMLService(contexts)
	title := " Explicit <&\U0001f332\r\n "
	request, err := json.Marshal(GoogleEarthKMLRequest{DescriptionField: "Zone", Title: title})
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.GetGoogleEarthKMLReview(context.Background(), state.ContextID, string(request))
	want, readErr := ReadGoogleEarthKML(contexts, context.Background(), state.ContextID, "Zone", title)
	if err != nil || readErr != nil || got == nil || got.ContextID != state.ContextID || got.Project != state.ActiveProject ||
		got.ProjectPath != state.ProjectPath || got.SU != "None" || got.SUPath != "" || got.Title != title ||
		got.DescriptionField != "Zone" || got.KML != string(want.Bytes) || got.PlacemarkCount != want.PlacemarkCount ||
		got.ByteCount != len(want.Bytes) || got.PlacemarkCount == 0 {
		t.Fatal("owned public preparation shape differs", got, err, readErr)
	}
	data, err := json.Marshal(got)
	var roundtrip GoogleEarthKMLReview
	if err != nil || json.Unmarshal(data, &roundtrip) != nil || !reflect.DeepEqual(*got, roundtrip) {
		t.Fatal("literal XML transport roundtrip differs", err)
	}
	for _, raw := range []string{
		`{}`, `{"descriptionField":"Zone"}`, `{"title":"title"}`,
		`{"descriptionField":"Zone","title":"title","unknown":0}`,
		`{"descriptionField":"Zone","title":"one","title":"two"}`,
		`{"descriptionField":"Zone","title":"one","Title":"two"}`,
		`{"descriptionField":"Zone","title":"one","\u0074itle":"two"}`,
		`{"descriptionField":"Zone","DescriptionField":"PlotNumber","title":"title"}`,
		`{"descriptionField":"Zone","title":"\ud800"}`,
		`{"descriptionField":"Zo\udfffne","title":"title"}`,
		`{"descriptionField":"Zone","title":"\u0000"}`,
		`{"descriptionField":"Zone","title":null}`,
		`{"descriptionField":"zone","title":"title"}`,
		`{"descriptionField":"Longitude","title":"title"}`,
		`{"descriptionField":"Zone","title":"` + string([]byte{0xff}) + `"}`,
	} {
		if got, err := service.GetGoogleEarthKMLReview(context.Background(), state.ContextID, raw); got != nil || err == nil {
			t.Fatal("malformed/unsupported raw request produced KML", raw, got, err)
		}
	}
	if got, err := service.GetGoogleEarthKMLReview(context.Background(), "foreign", string(request)); got != nil || err == nil {
		t.Fatal("foreign context returned public bytes", got, err)
	}
	empty, err := service.GetGoogleEarthKMLReview(context.Background(), state.ContextID, `{"descriptionField":"Zone","title":""}`)
	if err != nil || empty.Title != "" || !strings.Contains(empty.KML, "<name></name>") {
		t.Fatal("explicit empty title defaulted", empty, err)
	}
}
