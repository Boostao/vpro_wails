package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSummaryEnvironmentalClientWireFixture(t *testing.T) {
	service, state := summaryEnvironmentalCombinedFixture(t, false)
	owner := service.report.contexts.projects.sqlite
	mutateContextFixture(t, owner.attachments["project"], `UPDATE `+quoteHeaderIdentifier(state.ActiveProject+"_Env")+`
		SET StructuralStage='A',SuccessionalStatus='B',SurfaceTopographyType='C',
		MesoSlopePosition='D',HydroGeoSubSystem='E',HydroGeoSystem='F',SoilDrainage='G',
		HumusForm='H',RootRestrictingType='I',SoilClassSubGroup='J',SoilClassGroup='K',
		RootZoneParticleSize='L',SurficialMaterialSurf='O',Exposure1='P',Zone='Q',
		SubZone='1',MoistureRegime='0',Aspect=0
		WHERE PlotNumber='P1'`)
	mutateContextFixture(t, owner.attachments["project"], `UPDATE `+quoteHeaderIdentifier(state.ActiveProject+"_Env")+`
		SET BedrockGeology1='B1',BedrockGeology2='B2',BedrockGeology3='B3',
		SiteDisturbance1='D1',SiteDisturbance2='D2' WHERE PlotNumber='P1'`)
	mutateContextFixture(t, owner.attachments["VPro64"], `INSERT INTO USysShortVegSummaryFieldList VALUES
		(NULL,'Env','Structure',NULL,-1,'SVStructureStage',NULL),
		(2,'Env','Successional',NULL,-1,'SVSuccessional',NULL),
		(3,'Env','Topography',NULL,-1,'SVSurfaceTopographyType',NULL),
		(4,'Env','Position',NULL,-1,'SVMesoSlopePosition',NULL),
		(5,'Env','Subsystem',NULL,-1,'SVHydroGeoSubSystem',NULL),
		(6,'Env','System',NULL,-1,'SVHydroGeoSystem',NULL),
		(7,'Env','Drainage',NULL,-1,'SVSoilDrainage',NULL),
		(8,'Env','Humus',NULL,-1,'SVHumusForm',NULL),
		(9,'Env','Restriction',NULL,-1,'SVRootRestrictingType',NULL),
		(10,'Env','Subgroup',NULL,-1,'SVSoilClassSubGroup',NULL),
		(11,'Env','Group',NULL,-1,'SVSoilClassGroup',NULL),
		(12,'Env','Particle',NULL,-1,'SVRootZonePartSize',NULL),
		(13,'Env','Material',NULL,-1,'SVSurficialMaterialSurf',NULL),
		(14,'Env','Exposure',NULL,-1,'SVExposure',NULL),
		(15,'Env','Zone',NULL,-1,'SVBioGeoClimaticZone',NULL),
		(16,'Env','Moisture',NULL,-1,'SVSmr',NULL),
		(17,'Env','BGC unit',NULL,-1,'SVBioGeoClimaticUnit',NULL),
		(18,'Env','Aspect',NULL,-1,'SVAspect',NULL),
		(19,'Env','Bedrock one',NULL,-1,'SVBedrockType',1),
		(20,'Env','Bedrock two',NULL,-1,'SVBedrockType',2),
		(21,'Env','Bedrock three',NULL,-1,'SVBedrockType',3),
		(22,'Env','Disturbance one',NULL,-1,'SVSiteDisturbance',1),
		(23,'Env','Disturbance two',NULL,-1,'SVSiteDisturbance',2)`)
	result, err := service.PreviewEnvironmental(t.Context(), state.ContextID, 1, 6, 0)
	if err != nil || result == nil || len(result.Rows) != 25 {
		t.Fatal(result, err)
	}
	result.Vegetation.ContextID = "summary-environment-wire"
	result.Vegetation.ProjectPath = `C:\fixture\Sample.db`
	result.Vegetation.SUPath = `C:\fixture\Sample.db`
	result.SourcePath = `C:\fixture\VPro64.db`
	result.EnvSourcePath = `C:\fixture\Sample.db`
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("frontend", "src", "summaryVegetationEnvironmentalWire.json")
	if private := os.Getenv("VPRO_SUMMARY_ENVIRONMENT_WIRE_PATH"); private != "" {
		path = private
	}
	if os.Getenv("VPRO_UPDATE_SUMMARY_ENVIRONMENT_WIRE") == "true" {
		if err := os.WriteFile(path, append(encoded, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, append(encoded, '\n')) {
		t.Fatal("production fixture differs", err)
	}
	var decoded SummaryVegetationEnvironmentalPreview
	decoder := json.NewDecoder(bytes.NewReader(actual))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil || !reflect.DeepEqual(*result, decoded) {
		t.Fatal("strict production roundtrip differs", err)
	}
}
