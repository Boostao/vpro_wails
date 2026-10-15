package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type nativeCoverCriteriaFixture struct {
	Schema int `json:"schema"`
	Groups []struct {
		Name        string      `json:"name"`
		Table       string      `json:"table"`
		Rows        [][]*string `json:"rows"`
		Expressions []struct {
			Layer    string    `json:"layer"`
			Values   []*string `json:"values"`
			Criteria []struct {
				Literal    string `json:"literal"`
				Comparison string `json:"comparison"`
			} `json:"criteria"`
		} `json:"expressions"`
	} `json:"groups"`
	SingleMidpoints struct {
		Bits     []string `json:"bits"`
		Criteria []struct {
			Literal    string `json:"literal"`
			Comparison string `json:"comparison"`
		} `json:"criteria"`
	} `json:"singleMidpoints"`
	SumPromotion struct {
		Bits             []string `json:"bits"`
		Value            string   `json:"value"`
		MatchedLiteral   string   `json:"matchedLiteral"`
		UnmatchedLiteral string   `json:"unmatchedLiteral"`
	} `json:"sumPromotion"`
}

func readNativeCoverCriteriaFixture(t *testing.T) nativeCoverCriteriaFixture {
	t.Helper()
	file, err := os.Open(filepath.Join("testdata", "profiling-cover-native.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var fixture nativeCoverCriteriaFixture
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Schema != 1 || len(fixture.Groups) != 4 {
		t.Fatal("incomplete independent native cover corpus")
	}
	return fixture
}

func nativeSingleCriterionWitness(t *testing.T, bits *string) any {
	t.Helper()
	if bits == nil {
		return nil
	}
	raw, err := hex.DecodeString(*bits)
	if err != nil || len(raw) != 4 {
		t.Fatal("incomplete independently observed SINGLE witness", bits, err)
	}
	return float64(math.Float32frombits(binary.LittleEndian.Uint32(raw)))
}

func TestSourceProfileCoverNativeCriterionMatrix(t *testing.T) {
	fixture := readNativeCoverCriteriaFixture(t)
	total := 0
	for _, group := range fixture.Groups {
		t.Run(group.Name, func(t *testing.T) {
			if len(group.Rows) == 0 || len(group.Expressions) == 0 {
				t.Fatal("empty source corpus group")
			}
			conn := profileSQLFixture(t)
			ctx := context.Background()
			if _, err := conn.ExecContext(ctx, `DELETE FROM USysEnv; DELETE FROM USysVeg`); err != nil {
				t.Fatal(err)
			}
			for index, row := range group.Rows {
				if len(row) != 3 {
					t.Fatal("source Cover1/Cover2/Cover7 witness incomplete")
				}
				plot := strconv.Itoa(index + 1)
				if _, err := conn.ExecContext(ctx, `INSERT INTO USysEnv(PlotNumber) VALUES(?)`, plot); err != nil {
					t.Fatal(err)
				}
				if _, err := conn.ExecContext(ctx, `INSERT INTO USysVeg(PlotNumber,Species,Cover1,Cover2,Cover7)
					VALUES(?,'TARGET',?,?,?)`, plot, nativeSingleCriterionWitness(t, row[0]),
					nativeSingleCriterionWitness(t, row[1]), nativeSingleCriterionWitness(t, row[2])); err != nil {
					t.Fatal(err)
				}
			}
			for _, expression := range group.Expressions {
				if len(expression.Values) != len(group.Rows) || len(expression.Criteria) == 0 {
					t.Fatal("source expression observations incomplete")
				}
				for _, criterion := range expression.Criteria {
					if len(criterion.Comparison) != len(group.Rows) ||
						strings.Trim(criterion.Comparison, "EGLN") != "" {
						t.Fatal("source comparison witness matrix incomplete")
					}
					for index, operator := range []string{"=", ">", "<"} {
						total++
						t.Run(expression.Layer+"/"+criterion.Literal+"/"+operator, func(t *testing.T) {
							table := numericProfileCriterionTable(group.Table, criterion.Literal, operator)
							*table.Rows[0].Cells[4].Text = expression.Layer
							*table.Rows[0].Cells[7].Text = "Common plots"
							if group.Table == "Lump" {
								*table.Rows[0].Cells[5].Text = "LT"
							}
							rules, err := compilePlotProfileRules(table, nil)
							if err != nil || len(rules) != 1 {
								t.Fatal("native source numeric cover criterion unavailable", rules, err)
							}
							query, args := profileMatchQuery(rules[0], nil, "Lumps")
							got, err := queryProfilePlots(ctx, conn, query+" ORDER BY CAST(PlotNumber AS INTEGER)", args)
							want := []string{}
							for witness, comparison := range criterion.Comparison {
								if comparison == rune("EGL"[index]) {
									want = append(want, strconv.Itoa(witness+1))
								}
							}
							if err != nil || !reflect.DeepEqual(got, want) {
								t.Fatal("independent native cover literal comparison differs", got, want, err)
							}
						})
					}
				}
			}
		})
	}
	if total != 405 {
		t.Fatal("native source predicate coverage incomplete", total)
	}
}

func TestSourceProfileCoverSingleNativeMidpoints(t *testing.T) {
	fixture := readNativeCoverCriteriaFixture(t)
	if len(fixture.SingleMidpoints.Bits) != 18 || len(fixture.SingleMidpoints.Criteria) != 8 {
		t.Fatal("independent native SINGLE midpoint matrix incomplete")
	}
	for _, criterion := range fixture.SingleMidpoints.Criteria {
		if len(criterion.Comparison) != len(fixture.SingleMidpoints.Bits) ||
			strings.Trim(criterion.Comparison, "EGL") != "" {
			t.Fatal("independent native midpoint comparisons incomplete")
		}
		for index, bits := range fixture.SingleMidpoints.Bits {
			t.Run(criterion.Literal+"/"+bits, func(t *testing.T) {
				got, err := profileCoverSingleDecimalCompare(nativeSingleCriterionWitness(t, &bits), criterion.Literal)
				want := map[byte]int64{'E': 0, 'G': 1, 'L': -1}[criterion.Comparison[index]]
				if err != nil || got != want {
					t.Fatal("source SINGLE midpoint/scale conversion differs", got, want, err)
				}
			})
		}
	}
}

func TestSourceProfileCoverSumRetainsNativeDoublePromotion(t *testing.T) {
	fixture := readNativeCoverCriteriaFixture(t)
	if len(fixture.SumPromotion.Bits) != 5 {
		t.Fatal("native overflow/cancellation witness incomplete")
	}
	values := make([]any, len(fixture.SumPromotion.Bits))
	for index, bits := range fixture.SumPromotion.Bits {
		values[index] = nativeSingleCriterionWitness(t, &bits)
	}
	got, err := profileSingleSum(values...)
	want := nativeCriterionWitness(t, fixture.SumPromotion.Value)
	if err != nil || math.Float64bits(got) != math.Float64bits(want) {
		t.Fatal("source overflow promotion/cancellation value differs", got, want, err)
	}
	conn := profileSQLFixture(t)
	for _, test := range []struct {
		literal string
		equal   bool
	}{{fixture.SumPromotion.MatchedLiteral, true}, {fixture.SumPromotion.UnmatchedLiteral, false}} {
		args := append([]any{test.literal}, values...)
		var equal bool
		if err := conn.QueryRowContext(context.Background(),
			`SELECT vpro_profile_single_sum_decimal_compare(?,?,?,?,?,?)=0`, args...).Scan(&equal); err != nil || equal != test.equal {
			t.Fatal("source sum lost DOUBLE promotion after cancellation", equal, test.equal, err)
		}
	}
}

func TestSourceProfileCoverLiteralAndStorageBoundaries(t *testing.T) {
	for _, table := range []string{"Veg", "Lump"} {
		for _, test := range []struct {
			raw     string
			decimal bool
		}{{"0.1", true}, {"0.10000000149011612", true}, {"1.0000000000000002", true},
			{"1", false}, {"001", false}, {"2147483648", true}, {"1e28", true},
			{"1e29", false}, {"1e-28", true}, {"1e-29", false}, {"1.5e-28", false}} {
			rules, err := compilePlotProfileRules(numericProfileCriterionTable(table, test.raw, ">"), nil)
			if err != nil || len(rules) != 1 {
				t.Fatal(rules, err)
			}
			number, valid := rules[0].criterion.(profileSQLNumber)
			if !valid || (number.decimal != "") != test.decimal || test.decimal && number.decimal != test.raw {
				t.Fatal("source raw cover literal was rounded/repaired or mistyped", table, test, number)
			}
		}
		for _, raw := range []string{strings.Repeat("0", 256), "1e309", "NaN", "1 OR 1=1", " 0.1 ", "0,1", "０.1"} {
			if _, err := compilePlotProfileRules(numericProfileCriterionTable(table, raw, ">"), nil); err == nil {
				t.Fatal("malformed/overlength source cover criterion accepted", table, raw)
			}
		}
	}
	for _, compare := range []func(any, string) (any, error){profileCoverSingleDecimalCompare, profileCoverDecimalCompare} {
		for _, value := range []any{nil, []byte(nil)} {
			if got, err := compare(value, "0.1"); err != nil || got != nil {
				t.Fatal("source raw-cover SQL NULL changed", got, err)
			}
		}
		for _, value := range []any{"0.1", []byte("0.1"), math.NaN(), math.Inf(1)} {
			if _, err := compare(value, "0.1"); err == nil {
				t.Fatal("malformed historical cover storage repaired", value)
			}
		}
		for _, literal := range []string{"0", "1", "1e29", "1e99999999", "0e99999999", strings.Repeat("0", 256)} {
			if _, err := compare(0.1, literal); err == nil {
				t.Fatal("invalid/non-DECIMAL callback literal accepted", literal)
			}
		}
	}
	if _, err := profileCoverSingleDecimalCompare(math.MaxFloat64, "0.1"); err == nil {
		t.Fatal("unsupported source SINGLE value overflow accepted")
	}
	for _, value := range []any{"0.1", []byte("0.1"), math.NaN(), math.Inf(1), math.MaxFloat64} {
		if _, err := profileSingleSum(value); err == nil {
			t.Fatal("invalid historical source SINGLE sum input repaired", value)
		}
	}
	if got, err := profileSingleSum(nil, []byte(nil)); err != nil || got != 0 {
		t.Fatal("source Nz all-NULL sum changed", got, err)
	}
}

func TestProjectProfilingCoverCriteriaFullServiceReadonly(t *testing.T) {
	fixture := readNativeCoverCriteriaFixture(t)
	group := fixture.Groups[1]
	if group.Name != "profile-cover-arithmetic-source" || len(group.Rows) != 7 {
		t.Fatal("complete independent arithmetic source group required")
	}
	service, state := contextServiceFixture(t)
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := db.Exec(`DELETE FROM Sample_Env; DELETE FROM Sample_Admin; DELETE FROM Sample_Veg; DELETE FROM Sample_Profile;
				INSERT INTO Sample_Profile("Order","Table","Field","Operator","Layer","Species","Criteria","Operation","PlotCount")
				VALUES(1,'Env','PlotNumber','Like','','','*','Add plots',999),
				(2,'Veg','Species','=','SumAll','TARGET','0.3','Common plots',999)`); err != nil {
		t.Fatal(err)
	}
	for index, row := range group.Rows {
		plot := strconv.Itoa(index + 1)
		if _, err := db.Exec(`INSERT INTO Sample_Env(PlotNumber) VALUES(?)`, plot); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO Sample_Admin(Plot) VALUES(?)`, plot); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,Cover1,Cover2,Cover7) VALUES(?,'TARGET',?,?,?)`,
			plot, nativeSingleCriterionWitness(t, row[0]), nativeSingleCriterionWitness(t, row[1]),
			nativeSingleCriterionWitness(t, row[2])); err != nil {
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
	if err != nil || !reflect.DeepEqual(result.PlotNumbers, []string{"1", "3"}) || result.TotalPlots != 7 ||
		len(result.Steps) != 2 || result.Steps[0].PlotCount != 7 || result.Steps[1].PlotCount != 2 ||
		result.Steps[1].Remaining != 2 {
		t.Fatal("full owned cover preview differs from native sum/literal observation", result, err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.RunProjectPlotProfile(cancelled, state.ContextID, request); !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(result, ProjectPlotProfileResult{}) {
		t.Fatal("cancelled cover preview returned successful partial results", result, err)
	}
	if result, err := service.RunProjectPlotProfile(context.Background(), state.ContextID, request); err != nil ||
		!reflect.DeepEqual(result.PlotNumbers, []string{"1", "3"}) {
		t.Fatal("cover preview cancellation prevented explicit retry", result, err)
	}
	after, err := service.ReviewProjectPlotProfile(context.Background(), state.ContextID)
	if err != nil || !reflect.DeepEqual(review, after) {
		t.Fatal("cover preview changed stored typed rules/counts/schema", after, err)
	}
	for role, original := range before {
		current, err := os.ReadFile(service.projects.sqlite.attachments[role])
		if err != nil || !bytes.Equal(original, current) {
			t.Fatal("cover preview changed physical attached data/history", role, err)
		}
	}
}

func TestSourceProfileCoverSingleSumNativeValues(t *testing.T) {
	fixture := readNativeCoverCriteriaFixture(t)
	conn := profileSQLFixture(t)
	ctx := context.Background()
	count := 0
	for _, group := range fixture.Groups {
		if group.Table != "Veg" {
			continue
		}
		for _, expression := range group.Expressions {
			if expression.Layer != "SumAll" {
				continue
			}
			for index, row := range group.Rows {
				var actual float64
				err := conn.QueryRowContext(ctx, `SELECT vpro_profile_single_sum(?,?,?)`,
					nativeSingleCriterionWitness(t, row[0]), nativeSingleCriterionWitness(t, row[1]),
					nativeSingleCriterionWitness(t, row[2])).Scan(&actual)
				if expression.Values[index] == nil {
					t.Fatal("native Nz sum cannot be NULL")
				}
				want := nativeCriterionWitness(t, *expression.Values[index])
				if err != nil || math.Float64bits(actual) != math.Float64bits(want) {
					t.Fatal("source left-associated SINGLE sum differs", group.Name, index+1, actual, want, err)
				}
				count++
			}
		}
	}
	if count != 20 {
		t.Fatal("native SINGLE sum output coverage incomplete", count)
	}
}
