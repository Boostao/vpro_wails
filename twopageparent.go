package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/boostao/vpro-wails/internal/fs882layout"
)

//go:embed resources/fs882-two-page-layout.json
var twoPageParentLayoutJSON []byte

//go:embed resources/fs882-two-page-chars-layout.json
var twoPageCHARSParentLayoutJSON []byte

var twoPageParentSources = sync.OnceValues(func() (map[string]*fs882layout.Layout, error) {
	result := make(map[string]*fs882layout.Layout, 2)
	for _, source := range []struct {
		form string
		data []byte
	}{{"FS882-8x6XL", twoPageParentLayoutJSON}, {"FS882-8x6XL-CHARS", twoPageCHARSParentLayoutJSON}} {
		var layout fs882layout.Layout
		if err := json.Unmarshal(source.data, &layout); err != nil {
			return nil, fmt.Errorf("two-page parent metadata: %w", err)
		}
		if layout.Root != source.form || len(layout.Forms) != 7 || layout.Forms[0].Name != source.form ||
			layout.Forms[0].RecordSource != "USysEnv" || len(layout.Forms[0].Pages) != 2 {
			return nil, errors.New("two-page parent requires its exact exported variant and source closure")
		}
		result[source.form] = &layout
	}
	return result, nil
})

func twoPageParentBindings(form string) ([]siviParentBinding, error) {
	sources, err := twoPageParentSources()
	if err != nil {
		return nil, err
	}
	layout, exists := sources[form]
	if !exists {
		return nil, errors.New("two-page parent requires an exact normal or CHARS source variant")
	}
	result := []siviParentBinding{}
	controls, columns := map[string]bool{}, map[string]bool{}
	for _, field := range layout.Forms[0].Fields {
		if field.Binding == "" {
			continue
		}
		if field.ControlID == "" || controls[field.ControlID] || !field.ReadOnly || field.Implementation != "unmapped" {
			return nil, errors.New("two-page parent requires distinct unmapped source control identities")
		}
		controls[field.ControlID] = true
		columns[strings.ToLower(field.Binding)] = true
		result = append(result, siviParentBinding{ControlID: field.ControlID, Binding: field.Binding})
	}
	expectedColumns, expectedInstances := 118, 121
	if form == "FS882-8x6XL-CHARS" {
		expectedColumns, expectedInstances = 120, 122
	}
	if len(columns) != expectedColumns || len(result) != expectedInstances {
		return nil, errors.New("two-page parent source instance/column counts changed")
	}
	return result, nil
}

func projectTwoPageParent(ctx context.Context, contextID, project, plot, form string, env, admin ProjectMetadataTable) (*siviParentProjection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if contextID == "" || project == "" {
		return nil, errors.New("two-page parent requires owned context/project identities")
	}
	bindings, err := twoPageParentBindings(form)
	if err != nil {
		return nil, err
	}
	parent, err := projectSourceParent(ctx, contextID, project, plot, form, bindings, env, admin)
	if err != nil {
		return nil, fmt.Errorf("two-page parent projection: %w", err)
	}
	return parent, nil
}
