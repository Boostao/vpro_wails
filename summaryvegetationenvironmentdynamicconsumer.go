package main

import (
	"errors"
	"fmt"
	"strconv"
)

func summaryEnvironmentalSelectedFunction(function string, argument ProjectMetadataCell) (bool, int, error) {
	if function == "SVBedrockType" || function == "SVSiteDisturbance" {
		text, err := vegetationReportIdentity(argument)
		if err != nil || text == nil || (*text != "1" && *text != "2" && *text != "3") ||
			function == "SVSiteDisturbance" && *text == "3" {
			return false, 0, fmt.Errorf("function %q requires calibrated literal TEXT FieldNumber: bedrock1/2/3 or disturbance1/2; disturbance3 unavailable", function)
		}
		number, err := strconv.Atoi(*text)
		if err != nil {
			return false, 0, err
		}
		return true, number, nil
	}
	if function != "SVnPlots" {
		if _, supported := summaryVegetationEnvironmentalCategoryField(function); !supported {
			return false, 0, fmt.Errorf("function %q is unavailable for environmental consumption", function)
		}
	}
	if argument.Storage != "null" {
		return false, 0, fmt.Errorf("function %q has an unavailable FieldNumber argument", function)
	}
	if _, err := metadataCellValue(argument); err != nil {
		return false, 0, errors.New("FieldNumber NULL carries contradictory payload")
	}
	return false, 0, nil
}
