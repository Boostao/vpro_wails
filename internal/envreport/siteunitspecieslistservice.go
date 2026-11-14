package envreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
)

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
	if err := vprocore.DecodeStrictRequiredJSON(data, &value, "Summary species preview",
		"method", "orderBy", "coverCalculation", "andOr", "presenceGreaterThan", "coverGreaterThan"); err != nil {
		return err
	}
	*request = SiteUnitSpeciesListRequest(value)
	return nil
}

type SiteUnitSpeciesListRow struct {
	Species         string                        `json:"species"`
	ScientificName  vprocore.ProjectMetadataCell  `json:"scientificName"`
	EnglishName     vprocore.ProjectMetadataCell  `json:"englishName"`
	CodeType        *vprocore.ProjectMetadataCell `json:"codeType"`
	Cover           string                        `json:"cover"`
	Presence        string                        `json:"presence"`
	PhysicalValues  int                           `json:"physicalValues"`
	Included        bool                          `json:"included"`
	ReferenceRowIDs []string                      `json:"referenceRowIds"`
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

func PreviewSiteUnitSummarySpecies(s *vprocore.ContextService, ctx context.Context, contextID string,
	request SiteUnitSpeciesListRequest) (SiteUnitSpeciesListPreview, error) {
	if ctx == nil {
		return SiteUnitSpeciesListPreview{}, errors.New("Summary species preview requires a context")
	}
	if err := ctx.Err(); err != nil {
		return SiteUnitSpeciesListPreview{}, err
	}
	if s == nil {
		return SiteUnitSpeciesListPreview{}, errors.New("Summary species preview requires owned contexts")
	}
	options := siteUnitSpeciesListOptions{request.OrderBy, request.CoverCalculation, request.AndOr,
		request.PresenceGreaterThan, request.CoverGreaterThan}
	if err := validateSiteUnitSpeciesListOptions(options); err != nil {
		return SiteUnitSpeciesListPreview{}, err
	}
	input, err := ReadSiteUnitQuickVegetationInput(s, ctx, contextID, request.Method, vprocore.PublicationReadSnapshotHooks{})
	if err != nil {
		return SiteUnitSpeciesListPreview{}, err
	}
	return siteUnitSummarySpeciesPreview(ctx, input, request)
}
