package main

import (
	"context"
	"errors"
)

const siteUnitSummarySpeciesFeatureEnvironment = "VPRO_SITE_UNIT_SUMMARY_SPECIES"

type SiteUnitSpeciesListRequest struct {
	Method              int `json:"method"`
	OrderBy             int `json:"orderBy"`
	CoverCalculation    int `json:"coverCalculation"`
	AndOr               int `json:"andOr"`
	PresenceGreaterThan int `json:"presenceGreaterThan"`
	CoverGreaterThan    int `json:"coverGreaterThan"`
}

func (request *SiteUnitSpeciesListRequest) UnmarshalJSON(data []byte) error {
	type plain SiteUnitSpeciesListRequest
	var value plain
	if err := decodeStrictRequiredJSON(data, &value, "Summary species preview",
		"method", "orderBy", "coverCalculation", "andOr", "presenceGreaterThan", "coverGreaterThan"); err != nil {
		return err
	}
	*request = SiteUnitSpeciesListRequest(value)
	return nil
}

type SiteUnitSpeciesListRow struct {
	Species         string               `json:"species"`
	ScientificName  ProjectMetadataCell  `json:"scientificName"`
	EnglishName     ProjectMetadataCell  `json:"englishName"`
	CodeType        *ProjectMetadataCell `json:"codeType"`
	Cover           string               `json:"cover"`
	Presence        string               `json:"presence"`
	PhysicalValues  int                  `json:"physicalValues"`
	Included        bool                 `json:"included"`
	ReferenceRowIDs []string             `json:"referenceRowIds"`
}

type SiteUnitSpeciesListGroup struct {
	Index   int                      `json:"index"`
	Caption string                   `json:"caption"`
	Rows    []SiteUnitSpeciesListRow `json:"rows"`
}

type SiteUnitSpeciesListUnit struct {
	Code   string                     `json:"code"`
	NPlots int                        `json:"nPlots"`
	Groups []SiteUnitSpeciesListGroup `json:"groups"`
}

type SiteUnitSpeciesListPreview struct {
	Environment SiteUnitSummaryPreview     `json:"environment"`
	Options     SiteUnitSpeciesListRequest `json:"options"`
	Units       []SiteUnitSpeciesListUnit  `json:"units"`
}

func (s *ContextService) PreviewSiteUnitSummarySpecies(ctx context.Context, contextID string,
	request SiteUnitSpeciesListRequest) (SiteUnitSpeciesListPreview, error) {
	if ctx == nil {
		return SiteUnitSpeciesListPreview{}, errors.New("Summary species preview requires a context")
	}
	if err := ctx.Err(); err != nil {
		return SiteUnitSpeciesListPreview{}, err
	}
	if s == nil || !s.siteUnitSummarySpeciesEnabled {
		return SiteUnitSpeciesListPreview{}, errors.New("Summary species preview is disabled in this session")
	}
	options := siteUnitSpeciesListOptions{request.OrderBy, request.CoverCalculation, request.AndOr,
		request.PresenceGreaterThan, request.CoverGreaterThan}
	if err := validateSiteUnitSpeciesListOptions(options); err != nil {
		return SiteUnitSpeciesListPreview{}, err
	}
	if request.OrderBy == 2 && !s.siteUnitSummaryLifeformsEnabled {
		return SiteUnitSpeciesListPreview{}, errors.New("Summary lifeform grouping is disabled in this session")
	}
	input, err := s.readSiteUnitQuickVegetationInput(ctx, contextID, request.Method, publicationReadSnapshotHooks{})
	if err != nil {
		return SiteUnitSpeciesListPreview{}, err
	}
	return siteUnitSummarySpeciesPreview(ctx, input, request)
}
