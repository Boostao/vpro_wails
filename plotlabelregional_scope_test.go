package main

import (
	"testing"
)

func TestPlotLabelRegionalLiteralStorageAndAmbiguousParents(t *testing.T) {
	service, state := plotLabelRegionalFixture(t)
	path := service.contexts.projects.sqlite.attachments["project"]
	mutateContextFixture(t, path, `ALTER TABLE Sample_Env RENAME TO OriginalLabelEnv;
CREATE TABLE Sample_Env(PlotNumber TEXT COLLATE NOCASE,Zone,SubZone,SiteSeries,PlotRepresenting,ProjectID);
INSERT INTO Sample_Env VALUES('lb001','wrong',NULL,NULL,NULL,NULL);
INSERT INTO Sample_Env VALUES('LB001 ','wrong',NULL,NULL,NULL,NULL);
INSERT INTO Sample_Env VALUES('LB001','CWH','dm','01','source','project')`)
	got, err := service.GetPreview(t.Context(), state.ContextID, "LB001")
	if err != nil || got == nil || *got.Header.Zone != "CWH" {
		t.Fatal("literal source scope inherited NOCASE/space aliases", got, err)
	}
	mutateContextFixture(t, path, `INSERT INTO Sample_Env VALUES('LB001','CWH','dm','01','duplicate','project')`)
	if got, err := service.GetPreview(t.Context(), state.ContextID, "LB001"); err == nil || got != nil {
		t.Fatal("duplicate Env parents returned an arbitrary label", got, err)
	}
	mutateContextFixture(t, path, `DROP TABLE Sample_Env;
CREATE TABLE Sample_Env(PlotNumber NUMERIC,Zone,SubZone,SiteSeries,PlotRepresenting,ProjectID);
INSERT INTO Sample_Env VALUES('00012','CWH','dm','01','source','project');
INSERT INTO Sample_Admin(Plot) VALUES('12')`)
	for _, plot := range []string{"12", "00012"} {
		if got, err := service.GetPreview(t.Context(), state.ContextID, plot); err == nil || got != nil {
			t.Fatal("numeric-affinity source invented text ownership", plot, got, err)
		}
	}
	mutateContextFixture(t, path, `DROP TABLE Sample_Env;
CREATE VIEW Sample_Env AS SELECT * FROM OriginalLabelEnv`)
	if got, err := service.GetPreview(t.Context(), state.ContextID, "LB001"); err == nil || got != nil {
		t.Fatal("replacement view established physical label ownership", got, err)
	}
}
