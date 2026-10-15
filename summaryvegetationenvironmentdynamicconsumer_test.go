package main

import (
	"context"
	"errors"
	"strconv"
	"testing"
)

func TestSummaryEnvironmentalDynamicSelectedArgumentOwnedCases(t *testing.T) {
	service, state := summaryEnvironmentalCombinedFixture(t, false)
	owner := service.report.contexts.projects.sqlite
	mutateContextFixture(t, owner.attachments["project"], `UPDATE `+quoteHeaderIdentifier(state.ActiveProject+"_Env")+`
		SET BedrockGeology1='B1',BedrockGeology2='B2',BedrockGeology3='B3',
		SiteDisturbance1='D1',SiteDisturbance2='D2' WHERE PlotNumber='P1'`)
	for _, spec := range []struct {
		function, expected string
		argument           int
	}{{"SVBedrockType", "B1", 1}, {"SVBedrockType", "B2", 2}, {"SVBedrockType", "B3", 3},
		{"SVSiteDisturbance", "D1", 1}, {"SVSiteDisturbance", "D2", 2}} {
		mutateContextFixture(t, owner.attachments["VPro64"], `UPDATE USysShortVegSummaryFieldList SET FunctionName='`+spec.function+`',FieldNumber=`+
			strconv.Itoa(spec.argument)+` WHERE FunctionName<>'SVnPlots'`)
		got, err := service.PreviewEnvironmental(t.Context(), state.ContextID, 1, 6, 0)
		if err != nil || got == nil || *got.Rows[1].Values[0].Text != spec.expected+"(2)  (Null 1)" ||
			*got.Rows[1].Values[1].Text != "(Null 1)" {
			t.Fatal(got, err)
		}
	}
	for _, clause := range []string{
		"FunctionName='SVSiteDisturbance',FieldNumber=3",
		"FunctionName='SVBedrockType',FieldNumber=NULL",
		"FunctionName='SVBedrockType',FieldNumber=4",
		"FunctionName='SVBedrockType',FieldNumber=1.5",
		"FunctionName='SVBedrockType',FieldNumber=CAST('1' AS BLOB)",
		"FunctionName='SVSmr',FieldNumber=1",
	} {
		mutateContextFixture(t, owner.attachments["VPro64"], `UPDATE USysShortVegSummaryFieldList SET `+clause+` WHERE FunctionName<>'SVnPlots'`)
		if got, err := service.PreviewEnvironmental(t.Context(), state.ContextID, 1, 6, 0); err == nil || got != nil {
			t.Fatal("invalid selected argument accepted", clause, got, err)
		}
	}
	mutateContextFixture(t, owner.attachments["VPro64"], `UPDATE USysShortVegSummaryFieldList SET FunctionName='SVBedrockType',FieldNumber=2 WHERE FunctionName<>'SVnPlots'`)
	if got, err := service.PreviewEnvironmental(t.Context(), state.ContextID, 1, 6, 0); err != nil || got == nil {
		t.Fatal("corrected selection did not recover", got, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if got, err := service.PreviewEnvironmental(ctx, state.ContextID, 1, 6, 0); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(got, err)
	}
}

func TestSummaryEnvironmentalDynamicSelectedPayloadRefusals(t *testing.T) {
	text := "1"
	real := 1.0
	for _, cell := range []ProjectMetadataCell{
		{Storage: "null"}, metadataInteger("1"), {Storage: "real", Real: &real},
		{Storage: "integer", Integer: &text, Text: &text}, metadataInteger("0"), metadataInteger("32768"),
	} {
		if _, _, err := summaryEnvironmentalSelectedFunction("SVBedrockType", cell); err == nil {
			t.Fatal("invalid argument accepted", cell)
		}
	}
}
