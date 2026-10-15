package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestSourceProfileSQLLiteralNativeDomain(t *testing.T) {
	raw, err := os.ReadFile("testdata/profiling-literal-native.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		SchemaVersion                  int      `json:"schemaVersion"`
		NativeDirectObservations       int      `json:"nativeDirectObservations"`
		UnavailableDomainPolicy        []string `json:"unavailableDomainPolicy"`
		NativeNegativeExponentBoundary struct {
			LastAccepted int `json:"lastAccepted"`
			FirstRefused int `json:"firstRefused"`
		} `json:"nativeNegativeExponentBoundary"`
		Cases []struct {
			Source                         string `json:"source"`
			Literal                        string `json:"literal"`
			NativeSyntaxError              bool   `json:"nativeSyntaxError"`
			DesktopUnavailableZeroExponent bool   `json:"desktopUnavailableZeroExponent"`
			DAOType                        int    `json:"daoType"`
			Displayed                      string `json:"displayed"`
			Binary64LE                     string `json:"binary64LE"`
		} `json:"cases"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		t.Fatal("unexpected trailing native fixture content", err)
	}
	if fixture.SchemaVersion != 1 || fixture.NativeDirectObservations != 106 || len(fixture.Cases) != 106 ||
		fixture.NativeNegativeExponentBoundary.LastAccepted != 1073741819 ||
		fixture.NativeNegativeExponentBoundary.FirstRefused != 1073741820 || len(fixture.UnavailableDomainPolicy) != 2 {
		t.Fatal("incomplete independently observed literal domain")
	}
	for index, row := range fixture.Cases {
		t.Run(fmt.Sprintf("%03d/%s", index, row.Literal), func(t *testing.T) {
			number, err := profileSQLNumberCriterion(row.Literal)
			if row.NativeSyntaxError || row.DesktopUnavailableZeroExponent {
				if err == nil {
					t.Fatal("native syntax failure/unqualified zero-exponent domain accepted", row, number)
				}
				return
			}
			if err != nil {
				t.Fatal("supported independently observed native literal refused", row, err)
			}
			expected, err := strconv.ParseFloat(row.Displayed, 64)
			if err != nil || number.value != expected {
				t.Fatal("native literal value changed", row, number, expected, err)
			}
			switch row.DAOType {
			case 4, 7:
				if number.decimal != "" {
					t.Fatal("native LONG/DOUBLE became DECIMAL", row, number)
				}
			case 20:
				if number.decimal != row.Literal {
					t.Fatal("native DECIMAL literal text changed", row, number)
				}
			default:
				t.Fatal("unqualified native literal descriptor", row)
			}
			if row.Binary64LE != "" && math.Float64bits(number.value) !=
				math.Float64bits(nativeCriterionWitness(t, row.Binary64LE)) {
				t.Fatal("independently measured native DOUBLE bits changed", row, number)
			}
		})
	}
}

func TestSourceProfileSQLLiteralDomainCallers(t *testing.T) {
	for _, table := range []string{"Veg", "Lump"} {
		for _, raw := range []string{"0e29", "0e308", "-0e308", "0e309", "0e-1073741820",
			"1e-1073741820", "0e+1073741820", "0e-999999999999999999999", "1e-0000001073741820"} {
			if _, err := compilePlotProfileRules(numericProfileCriterionTable(table, raw, "="), nil); err == nil ||
				!strings.Contains(err.Error(), "exponent") {
				t.Fatal("source literal domain refusal missing from compiler caller", table, raw, err)
			}
		}
		for _, raw := range []string{"0", "0e28", "-0e28", "0e-1073741819", "1e-1073741819",
			"0e-" + strings.Repeat("0", 241) + "1073741819"} {
			rules, err := compilePlotProfileRules(numericProfileCriterionTable(table, raw, "="), nil)
			if err != nil || len(rules) != 1 {
				t.Fatal("supported zero/boundary/leading-zero literal refused", table, raw, err)
			}
			number, valid := rules[0].criterion.(profileSQLNumber)
			if !valid || number.value != 0 || number.decimal != "" {
				t.Fatal("supported native zero/underflow changed", table, raw, number)
			}
		}
	}
	env := []ProjectMetadataColumn{{Name: "CriterionWitness", DeclaredType: "REAL"}}
	for _, raw := range []string{"0e29", "0e308", "0e309", "1e-1073741820"} {
		rules, err := compilePlotProfileRules(numericProfileCriterionTable("Env", raw, "="), env)
		if err != nil || len(rules) != 1 {
			t.Fatal("raw cover restriction leaked into accepted Env conversion", raw, err)
		}
		number, valid := rules[0].criterion.(profileSQLNumber)
		if !valid || number.value != 0 || number.decimal != "" {
			t.Fatal("Env converted zero changed", raw, number)
		}
	}
}

func TestProjectProfilingLiteralDomainServiceReadonly(t *testing.T) {
	service, state := contextServiceFixture(t)
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := db.Exec(`DELETE FROM Sample_Env; DELETE FROM Sample_Admin; DELETE FROM Sample_Veg; DELETE FROM Sample_Profile;
		INSERT INTO Sample_Env(PlotNumber) VALUES('1');
		INSERT INTO Sample_Admin(Plot) VALUES('1');
		INSERT INTO Sample_Veg(PlotNumber,Species,Cover1) VALUES('1','TARGET',0);
		INSERT INTO Sample_Profile("Order","Table","Field","Operator","Layer","Species","Criteria","Operation","PlotCount")
		VALUES(1,'Veg','Species','=','1','TARGET','0e307','Add plots',999)`); err != nil {
		t.Fatal(err)
	}
	for _, literal := range []string{"0e307", "1e-1073741820", "1e-1073741819"} {
		if _, err := db.Exec(`UPDATE Sample_Profile SET Criteria=?`, literal); err != nil {
			t.Fatal(err)
		}
		review, err := service.ReviewProjectPlotProfile(context.Background(), state.ContextID)
		if err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, service.projects.sqlite.attachments)
		result, err := service.RunProjectPlotProfile(context.Background(), state.ContextID,
			ProjectPlotProfileRunRequest{OriginalRules: review.Rules})
		if literal == "1e-1073741819" {
			if err != nil || !reflect.DeepEqual(result.PlotNumbers, []string{"1"}) || result.TotalPlots != 1 ||
				len(result.Steps) != 1 || result.Steps[0].PlotCount != 1 {
				t.Fatal("explicit correction/retry lost supported native underflow", result, err)
			}
		} else if err == nil || !strings.Contains(err.Error(), "exponent") ||
			!reflect.DeepEqual(result, ProjectPlotProfileResult{}) {
			t.Fatal("unsupported raw domain returned successful/partial preview", result, err)
		}
		after, err := service.ReviewProjectPlotProfile(context.Background(), state.ContextID)
		if err != nil || !reflect.DeepEqual(review, after) {
			t.Fatal("literal refusal/retry changed typed stored rules/counts/schema", after, err)
		}
		for role, original := range before {
			current, err := os.ReadFile(service.projects.sqlite.attachments[role])
			if err != nil || !bytes.Equal(original, current) {
				t.Fatal("literal refusal/retry changed physical data/history", role, err)
			}
		}
	}
}
