package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func numericProfileCriterionTable(table, raw, operator string) ProjectMetadataTable {
	names := []string{"Order", "Table", "Field", "Operator", "Layer", "Species", "Criteria", "Operation", "PlotCount"}
	field := "CriterionWitness"
	if table == "Veg" {
		field = "Species"
	} else if table == "Lump" {
		field = "LumpCode"
	}
	values := []string{"", table, field, operator, "1", "TARGET", raw, "Add plots", ""}
	result := ProjectMetadataTable{Rows: []ProjectMetadataRow{{RowID: "1"}}}
	for index, name := range names {
		result.Columns = append(result.Columns, ProjectMetadataColumn{Name: name, DeclaredType: "TEXT"})
		cell := ProjectMetadataCell{Storage: "null"}
		if name == "Order" {
			order := "1"
			cell = ProjectMetadataCell{Storage: "integer", Integer: &order}
		} else if name != "PlotCount" {
			cell = ProjectMetadataCell{Storage: "text", Text: &values[index]}
		}
		result.Rows[0].Cells = append(result.Rows[0].Cells, cell)
	}
	return result
}

func nativeCriterionWitness(t *testing.T, littleEndian string) float64 {
	t.Helper()
	raw, err := hex.DecodeString(littleEndian)
	if err != nil || len(raw) != 8 {
		t.Fatal("incomplete independently observed binary64 witness", littleEndian, err)
	}
	return math.Float64frombits(binary.LittleEndian.Uint64(raw))
}

func TestSourceProfileEnvNumericCriterionNativeThresholds(t *testing.T) {
	conn := profileSQLFixture(t)
	ctx := context.Background()
	if _, err := conn.ExecContext(ctx, `ALTER TABLE USysEnv ADD COLUMN CriterionWitness REAL; DELETE FROM USysEnv`); err != nil {
		t.Fatal(err)
	}
	bits := []string{
		"0000000000000000", "ffffffffffffef3f", "000000000000f03f", "010000000000f03f",
		"999999999999b93f", "9a9999999999b93f", "9b9999999999b93f", "0000000000003040",
	}
	for index, witness := range bits {
		if _, err := conn.ExecContext(ctx, `INSERT INTO USysEnv(PlotNumber,CriterionWitness) VALUES(?,?)`,
			strconv.Itoa(index+1), nativeCriterionWitness(t, witness)); err != nil {
			t.Fatal(err)
		}
	}
	env := []ProjectMetadataColumn{{Name: "CriterionWitness", DeclaredType: "REAL"}}
	for _, test := range []struct {
		raw               string
		equal, more, less []string
	}{
		{"0", []string{"1"}, []string{"2", "3", "4", "5", "6", "7", "8"}, []string{}},
		{"+1", []string{"3"}, []string{"4", "8"}, []string{"1", "2", "5", "6", "7"}},
		{"001", []string{"3"}, []string{"4", "8"}, []string{"1", "2", "5", "6", "7"}},
		{"1e0", []string{"3"}, []string{"4", "8"}, []string{"1", "2", "5", "6", "7"}},
		{"1.0000000000000002", []string{"3"}, []string{"4", "8"}, []string{"1", "2", "5", "6", "7"}},
		{"1.000000000000002", []string{"3"}, []string{"4", "8"}, []string{"1", "2", "5", "6", "7"}},
		{"0.10000000000000002", []string{"5", "6", "7"}, []string{"2", "3", "4", "8"}, []string{"1"}},
	} {
		for index, operator := range []string{"=", ">", "<"} {
			t.Run(test.raw+"/"+operator, func(t *testing.T) {
				rules, err := compilePlotProfileRules(numericProfileCriterionTable("Env", test.raw, operator), env)
				if err != nil || len(rules) != 1 {
					t.Fatal("source numeric criterion unavailable", rules, err)
				}
				query, args := profileMatchQuery(rules[0], env, "Lumps")
				got, err := queryProfilePlots(ctx, conn, query+" ORDER BY CAST(PlotNumber AS INTEGER)", args)
				want := [][]string{test.equal, test.more, test.less}[index]
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatal("native ChangeType/concatenated SQL threshold differs", got, want, err)
				}
			})
		}
	}
}

func TestSourceProfileEnvNumericCriterionNativeDecimalMatrix(t *testing.T) {
	base := []string{
		"0000000000000000", "999999999999b93f", "9a9999999999b93f", "9b9999999999b93f",
		"ffffffffffffef3f", "000000000000f03f", "010000000000f03f", "ffffffffffffdf41",
		"000000000000e041", "010000000000e041", "81121f2fe727c045", "21d7e6fae031f445",
		"a0c8eb85f3cce17f", "a0c8eb85f3cce1ff", "186050bef6b01f3a", "196050bef6b01f3a",
		"1a6050bef6b01f3a", "47b3a6fe5e5ae939", "47b3a6fe5e5ae9b9", "0100000000000000",
		"9b9999999999b9bf", "9a9999999999b9bf", "999999999999b9bf",
	}
	scale := append(append([]string{}, base...),
		"419b124f216e213a", "dedcd11e132f263a", "1348bc0eb9c4273a", "47b3a6fe5e5a293a",
		"419b124f216e21ba", "47b3a6fe5e5a29ba", "ffffffffffffef7f", "ffffffffffffefff",
		"fbffffffffffef45", "fcffffffffffef45", "fdffffffffffef45")
	rounding := append(append([]string{}, scale...),
		"0f3cf2369ace333a", "103cf2369ace333a", "113cf2369ace333a",
		"103cf2369ace33ba", "165486e6d7ba3b3a", "165486e6d7ba3bba")
	type expectation struct{ literal, comparison string }
	groups := []struct {
		name string
		bits []string
		rows []expectation
	}{
		{"decimal", base, []expectation{
			{"0.1", "LEEEGGGGGGGGGLLLLLLLLLL"},
			{"2147483648", "LLLLLLLEEEGGGLLLLLLLLLL"},
			{"1e-28", "LGGGGGGGGGGGGLEEELLLLLL"},
			{"1e29", "LLLLLLLLLLLEGLLLLLLLLLL"},
			{"-0.1", "GGGGGGGGGGGGGLGGGGGGEEE"},
			{"1.23456789123456e-20", "LGGGGGGGGGGGGLLLLLLLLLL"},
			{"1.234e-20", "LGGGGGGGGGGGGLLLLLLLLLL"},
			{"1.5e-28", "LGGGGGGGGGGGGLLLLLLLLLL"},
		}},
		{"scale", scale, []expectation{
			{"1e-28", "LGGGGGGGGGGGGLEEELLLLLLEEGGLLGLGGG"},
			{"-1e-28", "GGGGGGGGGGGGGLGGGGGGLLLGGGGELGLGGG"},
			{"7.92281625142643e28", "LLLLLLLLLLLGGLLLLLLLLLLLLLLLLGLEEE"},
			{"0.1", "LEEEGGGGGGGGGLLLLLLLLLLLLLLLLGLGGG"},
		}},
		{"rounding", rounding, []expectation{
			{"2e-28", "LGGGGGGGGGGGGLLLLLLLLLLLLEELLGLGGGEEGLGL"},
			{"3e-28", "LGGGGGGGGGGGGLLLLLLLLLLLLLLLLGLGGGLLELGL"},
			{"-2e-28", "GGGGGGGGGGGGGLGGGGGGLLLGGGGGEGLGGGGGGEGL"},
			{"-3e-28", "GGGGGGGGGGGGGLGGGGGGLLLGGGGGGGLGGGGGGGGL"},
		}},
	}
	env := []ProjectMetadataColumn{{Name: "CriterionWitness", DeclaredType: "REAL"}}
	total := 0
	for _, group := range groups {
		t.Run(group.name, func(t *testing.T) {
			conn := profileSQLFixture(t)
			ctx := context.Background()
			if _, err := conn.ExecContext(ctx, `ALTER TABLE USysEnv ADD COLUMN CriterionWitness REAL; DELETE FROM USysEnv`); err != nil {
				t.Fatal(err)
			}
			for index, bits := range group.bits {
				if _, err := conn.ExecContext(ctx, `INSERT INTO USysEnv(PlotNumber,CriterionWitness) VALUES(?,?)`,
					strconv.Itoa(index+1), nativeCriterionWitness(t, bits)); err != nil {
					t.Fatal(err)
				}
			}
			for _, row := range group.rows {
				if len(row.comparison) != len(group.bits) {
					t.Fatal("incomplete native matrix", row.literal, len(row.comparison), len(group.bits))
				}
				total += len(group.bits)
				for index, operator := range []string{"=", ">", "<"} {
					t.Run(fmt.Sprintf("%s/%s", row.literal, operator), func(t *testing.T) {
						rules, err := compilePlotProfileRules(numericProfileCriterionTable("Env", row.literal, operator), env)
						if err != nil || len(rules) != 1 {
							t.Fatal(rules, err)
						}
						query, args := profileMatchQuery(rules[0], env, "Lumps")
						got, err := queryProfilePlots(ctx, conn, query+" ORDER BY CAST(PlotNumber AS INTEGER)", args)
						want := []string{}
						for witness, comparison := range row.comparison {
							if comparison == rune("EGL"[index]) {
								want = append(want, strconv.Itoa(witness+1))
							}
						}
						if err != nil || !reflect.DeepEqual(got, want) {
							t.Fatal("independent native decimal comparison differs", got, want, err)
						}
					})
				}
			}
		})
	}
	if total != 480 {
		t.Fatal("native witness matrix incomplete", total)
	}
}

func TestSourceProfileEnvNumericCriterionNativeLiteralTypes(t *testing.T) {
	for _, test := range []struct {
		literal string
		decimal bool
	}{
		{"0", false}, {"1", false}, {"0.1", true}, {"0.10", true}, {"0.10000000000000002", true},
		{"1.0", false}, {"1.0000000000000002", true}, {"0.9999999999999999", true},
		{"1e0", false}, {"1e+0", false}, {"1e-1", true}, {"0.1e0", true},
		{"0.10000000000000002e0", true}, {"1.0000000000000002e0", true}, {"1.2", true}, {"16", false},
		{"2147483647", false}, {"2147483648", true}, {"-2147483648", false}, {"-2147483649", true},
		{"4294967296", true}, {"1e10", true}, {"1e15", true}, {"1e28", true}, {"1e29", false},
		{"1e30", false}, {"1e308", false}, {"7.92281625142643e28", true}, {"7.92281625142644e28", false},
		{"1e-15", true}, {"1e-28", true}, {"1e-29", false}, {"1e-30", false},
		{"4.94065645841247e-324", false}, {"-0.1", true}, {"-1e29", false}, {"-1e-29", false},
		{"1.23456789123456e-20", false}, {"1.234e-20", true}, {"1.5e-28", false},
	} {
		decimal, valid := new(big.Rat).SetString(test.literal)
		if !valid {
			t.Fatal("invalid independent native literal", test.literal)
		}
		if got := sourceSQLDecimalLiteral(decimal); got != test.decimal {
			t.Fatal("native SQL LONG/DECIMAL/DOUBLE routing differs", test.literal, got, test.decimal)
		}
	}
}

func TestSourceProfileEnvNumericCriterionPreservesOtherBoundaries(t *testing.T) {
	env := []ProjectMetadataColumn{{Name: "CriterionWitness", DeclaredType: "REAL"}}
	raw := "1.0000000000000002"
	for _, table := range []string{"Veg", "Lump"} {
		rules, err := compilePlotProfileRules(numericProfileCriterionTable(table, raw, ">"), env)
		if err != nil || len(rules) != 1 {
			t.Fatal(rules, err)
		}
		criterion, valid := rules[0].criterion.(profileSQLNumber)
		if !valid || math.Float64bits(criterion.value) != 0x3ff0000000000001 || criterion.decimal != raw {
			t.Fatal("Env conversion leaked into raw cover criteria", table, rules[0])
		}
		query, args := profileMatchQuery(rules[0], env, "Lumps")
		if strings.Contains(query, "vpro_profile_env_decimal_compare") || args[1] != raw {
			t.Fatal("Env conversion leaked into Veg/Lump SQL", table, query, args)
		}
	}
	for _, raw := range []string{"1e309", "1.7976931348623157e308", "-1.7976931348623157e308"} {
		if _, err := compilePlotProfileRules(numericProfileCriterionTable("Env", raw, ">"), env); err == nil {
			t.Fatal("nonfinite source SQL threshold accepted", raw)
		}
	}
	for _, value := range []float64{math.NaN(), math.Inf(-1), math.Inf(1), math.MaxFloat64} {
		if _, err := profileEnvNumberCriterion(value); err == nil {
			t.Fatal("source conversion helper accepted nonfinite literal", value)
		}
	}
	for _, operator := range []string{"Like", "Not Like"} {
		if _, err := compilePlotProfileRules(numericProfileCriterionTable("Env", "0.1", operator), env); err == nil {
			t.Fatal("numeric Like was enabled", operator)
		}
	}
	for _, raw := range []any{"0.1", []byte("0.1"), math.NaN(), math.Inf(1)} {
		if _, err := profileEnvDecimalCompare(raw, "0.1"); err == nil {
			t.Fatal("historical malformed numeric storage repaired", raw)
		}
	}
	for _, literal := range []string{"", "1", "1e29", "1e9999999", "1e-9999999", "NaN", "1 OR 1=1", "0,1", " 0.1 "} {
		if _, err := profileEnvDecimalCompare(0.1, literal); err == nil {
			t.Fatal("invalid/non-DECIMAL comparison literal accepted", literal)
		}
	}
	for _, raw := range []any{nil, []byte(nil)} {
		if got, err := profileEnvDecimalCompare(raw, "0.1"); err != nil || got != nil {
			t.Fatal("SQL NULL numeric comparison changed", got, err)
		}
	}
	for _, zero := range []float64{0, math.Copysign(0, -1)} {
		number, err := profileEnvNumberCriterion(zero)
		if err != nil || number.decimal != "" || math.Float64bits(number.value) != 0 {
			t.Fatal("source zero literal introduced signed zero", number, err)
		}
	}
}

func TestSourceProfileEnvNumericCriterionNullBooleanAndStorageErrors(t *testing.T) {
	conn := profileSQLFixture(t)
	ctx := context.Background()
	env := []ProjectMetadataColumn{{Name: "Flag", DeclaredType: "BOOLEAN"}}
	table := numericProfileCriterionTable("Env", "0.1", "<")
	*table.Rows[0].Cells[2].Text = "Flag"
	rules, err := compilePlotProfileRules(table, env)
	if err != nil {
		t.Fatal(err)
	}
	query, args := profileMatchQuery(rules[0], env, "Lumps")
	if !strings.Contains(query, "vpro_profile_env_decimal_compare(vpro_profile_bool(") {
		t.Fatal("source BOOLEAN normalization was bypassed", query)
	}
	got, err := queryProfilePlots(ctx, conn, query+" ORDER BY PlotNumber", args)
	if err != nil || !reflect.DeepEqual(got, []string{"M", "P", "Z"}) {
		t.Fatal("NULL or true=-1 numeric comparison changed", got, err)
	}
	if _, err := conn.ExecContext(ctx, `UPDATE USysEnv SET Flag=2 WHERE PlotNumber='P'`); err != nil {
		t.Fatal(err)
	}
	if _, err := queryProfilePlots(ctx, conn, query, args); err == nil || !strings.Contains(err.Error(), "historical BOOLEAN") {
		t.Fatal("invalid source BOOLEAN became a success", err)
	}
}

func TestProjectProfilingEnvNumericCriterionFullServiceReadonly(t *testing.T) {
	service, state := contextServiceFixture(t)
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := db.Exec(`ALTER TABLE Sample_Env ADD COLUMN CriterionWitness REAL;
		DELETE FROM Sample_Env; DELETE FROM Sample_Admin;
		DELETE FROM Sample_Profile WHERE rowid<>3;
		UPDATE Sample_Profile SET "Order"=1,"Table"='Env',"Field"='CriterionWitness',"Operator"='=',
			Criteria='0.10000000000000002',Operation='Add plots',PlotCount=999 WHERE rowid=3`); err != nil {
		t.Fatal(err)
	}
	for index, bits := range []string{"999999999999b93f", "9a9999999999b93f", "9b9999999999b93f"} {
		plot := strconv.Itoa(index + 1)
		if _, err := db.Exec(`INSERT INTO Sample_Env(PlotNumber,CriterionWitness) VALUES(?,?)`,
			plot, nativeCriterionWitness(t, bits)); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO Sample_Admin(Plot) VALUES(?)`, plot); err != nil {
			t.Fatal(err)
		}
	}
	review, err := service.ReviewProjectPlotProfile(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	request := ProjectPlotProfileRunRequest{OriginalRules: review.Rules}
	before := databaseBytes(t, service.projects.sqlite.attachments)
	result, err := service.RunProjectPlotProfile(context.Background(), state.ContextID, request)
	if err != nil || !reflect.DeepEqual(result.PlotNumbers, []string{"1", "2", "3"}) ||
		result.TotalPlots != 3 || len(result.Steps) != 1 || result.Steps[0].PlotCount != 3 ||
		result.Steps[0].Remaining != 3 {
		t.Fatal("full owned profile service differs from native fractional threshold", result, err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.RunProjectPlotProfile(cancelled, state.ContextID, request); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, ProjectPlotProfileResult{}) {
		t.Fatal("cancelled numeric preview returned a result", result, err)
	}
	after, err := service.ReviewProjectPlotProfile(context.Background(), state.ContextID)
	if err != nil || !reflect.DeepEqual(review, after) {
		t.Fatal("typed historical criterion/counts/schema changed", after, err)
	}
	for role, original := range before {
		current, err := os.ReadFile(service.projects.sqlite.attachments[role])
		if err != nil || !bytes.Equal(original, current) {
			t.Fatal("numeric preview changed a physical attached database", role, err)
		}
	}
}
