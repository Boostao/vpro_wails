package main

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var profileDecimal = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)
var profileCovers = []string{"Cover1", "Cover2", "Cover3", "Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c", "Cover6", "Cover7"}

type plotProfileRule struct {
	rowID, table, field, operation, operator, layer, species string
	order                                                    int16
	criterion                                                any
	envNull                                                  bool
}

func profileASCII(value string) bool {
	for _, character := range value {
		if character > 127 || character == 0 {
			return false
		}
	}
	return true
}

func profileChoice(value string, choices ...string) (string, error) {
	if profileASCII(value) {
		for _, choice := range choices {
			if strings.EqualFold(value, choice) {
				return choice, nil
			}
		}
	}
	return "", fmt.Errorf("unsupported literal choice %q; no value was repaired", value)
}

func compilePlotProfileRules(table ProjectMetadataTable, env []ProjectMetadataColumn) ([]plotProfileRule, error) {
	indices := map[string]int{}
	for index, column := range table.Columns {
		if _, duplicate := indices[column.Name]; duplicate {
			return nil, errors.New("profile rule schema is ambiguous")
		}
		indices[column.Name] = index
	}
	for _, name := range []string{"Order", "Table", "Field", "Operator", "Layer", "Species", "Criteria", "Operation", "PlotCount"} {
		if _, present := indices[name]; !present {
			return nil, fmt.Errorf("profile rule schema is missing %s", name)
		}
	}
	if len(table.Rows) == 0 {
		return nil, errors.New("profile has no physical rules; no filter or scratch results changed")
	}
	seenRows, seenOrders := map[string]bool{}, map[int64]bool{}
	rules := []plotProfileRule{}
	for _, row := range table.Rows {
		identity, err := strconv.ParseInt(row.RowID, 10, 64)
		if err != nil || strconv.FormatInt(identity, 10) != row.RowID || seenRows[row.RowID] || len(row.Cells) != len(table.Columns) {
			return nil, errors.New("profile requires distinct observed physical rows and complete cells")
		}
		seenRows[row.RowID] = true
		cell := row.Cells[indices["Order"]]
		if cell.Storage != "integer" || cell.Integer == nil {
			return nil, fmt.Errorf("profile row %s requires a stored signed16 Order", row.RowID)
		}
		order, err := strconv.ParseInt(*cell.Integer, 10, 16)
		if err != nil || seenOrders[order] {
			return nil, fmt.Errorf("profile row %s has invalid or duplicate Order; execution cannot choose the source FindFirst row", row.RowID)
		}
		seenOrders[order] = true
		text := func(name string) (string, error) {
			cell := row.Cells[indices[name]]
			if cell.Storage != "text" || cell.Text == nil {
				return "", fmt.Errorf("%s requires literal text; NULL/historical storage was not coerced", name)
			}
			return *cell.Text, nil
		}
		compile := func() (plotProfileRule, error) {
			rule := plotProfileRule{rowID: row.RowID, order: int16(order)}
			var err error
			if rule.table, err = text("Table"); err != nil {
				return rule, err
			}
			if rule.table, err = profileChoice(rule.table, "Env", "Veg", "Lump"); err != nil {
				return rule, err
			}
			if rule.operation, err = text("Operation"); err != nil {
				return rule, err
			}
			if rule.operation, err = profileChoice(rule.operation, "Add plots", "Subtract plots", "Common plots"); err != nil {
				return rule, err
			}
			if rule.operator, err = text("Operator"); err != nil {
				return rule, err
			}
			operators := []string{"=", ">", "<"}
			if rule.table == "Env" {
				operators = append(operators, "Like", "Not Like")
			}
			if rule.operator, err = profileChoice(rule.operator, operators...); err != nil {
				return rule, err
			}
			if rule.field, err = text("Field"); err != nil {
				return rule, err
			}
			raw, err := text("Criteria")
			if err != nil {
				return rule, err
			}
			if rule.table == "Env" {
				column := slices.IndexFunc(env, func(column ProjectMetadataColumn) bool { return column.Name == rule.field })
				if column < 0 {
					return rule, errors.New("Env Field is not an observed literal column; stored SQL expressions are unavailable")
				}
				if strings.EqualFold(raw, "Null") && profileASCII(raw) {
					rule.envNull = true
					return rule, nil
				}
				kind := strings.ToUpper(env[column].DeclaredType)
				if strings.Contains(kind, "DATE") || strings.Contains(kind, "TIME") {
					return rule, errors.New("date criteria require separately verified Access conversion; no machine locale was imported")
				}
			} else {
				expected := "Species"
				if rule.table == "Lump" {
					expected = "LumpCode"
				}
				if rule.field != expected {
					return rule, fmt.Errorf("%s requires its explicit %s selector", rule.table, expected)
				}
				if rule.layer, err = text("Layer"); err != nil {
					return rule, err
				}
				layers := []string{"Any", "SumAll", "SumA", "SumB", "1", "2", "3", "4", "5", "5a", "5b", "5c", "6", "7"}
				if rule.layer, err = profileChoice(rule.layer, layers...); err != nil {
					return rule, err
				}
				if rule.species, err = text("Species"); err != nil {
					return rule, err
				}
				if !profileASCII(rule.species) {
					return rule, errors.New("non-ASCII species/lump matching remains unverified")
				}
				if rule.table == "Lump" && rule.layer == "Any" && rule.operator == "=" && rule.operation == "Add plots" {
					return rule, errors.New("source Lump/Any/Add equality also calls defective MadEquals; this combination remains unavailable")
				}
			}
			if strings.EqualFold(raw, "NULL") && profileASCII(raw) && rule.table != "Env" {
				rule.criterion = nil
			} else if profileDecimal.MatchString(raw) {
				number, err := strconv.ParseFloat(raw, 64)
				if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
					return rule, errors.New("criterion is not a finite decimal")
				}
				if rule.operator == "Like" || rule.operator == "Not Like" {
					return rule, errors.New("numeric Like coercion remains unavailable")
				}
				rule.criterion = number
			} else if rule.table == "Env" {
				if !profileASCII(raw) || (rule.operator == "Like" || rule.operator == "Not Like") && strings.ContainsAny(raw, "[]#") ||
					strings.Contains(raw, ",") && strings.IndexAny(strings.TrimLeft(raw, "+- \t\r\n"), "0123456789") == 0 ||
					raw != strings.TrimSpace(raw) && profileDecimal.MatchString(strings.TrimSpace(raw)) {
					return rule, errors.New("locale/non-ASCII/bracket/hash criteria require separately verified semantics")
				}
				rule.criterion = raw
			} else {
				return rule, errors.New("cover criterion must be a finite decimal or explicit SQL NULL literal; raw SQL is unavailable")
			}
			return rule, nil
		}
		rule, err := compile()
		if err != nil {
			return nil, fmt.Errorf("profile row %s (Order %d): %w", row.RowID, order, err)
		}
		rules = append(rules, rule)
	}
	slices.SortFunc(rules, func(a, b plotProfileRule) int { return int(a.order) - int(b.order) })
	return rules, nil
}

func profileLayerColumns(layer string, absence bool) []string {
	switch layer {
	case "Any", "SumAll":
		return profileCovers
	case "SumA":
		return profileCovers[:3]
	case "SumB":
		if absence {
			return profileCovers[3:5]
		}
		return profileCovers[3:8]
	default:
		return []string{"Cover" + layer}
	}
}
