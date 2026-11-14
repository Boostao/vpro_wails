package main

import (
	"context"
	"errors"
	"log"

	classificationpkg "github.com/boostao/vpro-wails/internal/classification"
	"github.com/boostao/vpro-wails/internal/lmh25rules"
	"github.com/boostao/vpro-wails/internal/plotentry"
	referencespkg "github.com/boostao/vpro-wails/internal/references"
	sivipkg "github.com/boostao/vpro-wails/internal/sivi"
	sourceformpkg "github.com/boostao/vpro-wails/internal/sourceform"
	twopagepkg "github.com/boostao/vpro-wails/internal/twopage"
	vprocorepkg "github.com/boostao/vpro-wails/internal/vprocore"
)

type referenceCatalogBundle struct {
	references     *referencespkg.ReferenceService
	bec            *referencespkg.BECService
	quality        *referencespkg.QualityService
	siteCodes      *referencespkg.SiteCodeService
	siteCodesErr   error
	regionCodes    *referencespkg.RegionCodeService
	soilCodes      *referencespkg.SoilCodeService
	geologyCodes   *referencespkg.GeologyCodeService
	parentCodes    *referencespkg.ParentCodeService
	parentCodesErr error
}

func openReferenceCatalogBundle(dataDir string) *referenceCatalogBundle {
	b := &referenceCatalogBundle{}
	var err error
	if b.references, err = referencespkg.NewReferenceService(dataDir); err != nil {
		log.Printf("Warning: failed to initialize ReferenceService: %v", err)
	}
	if b.bec, err = referencespkg.NewBECService(dataDir); err != nil {
		log.Printf("Warning: failed to initialize BECService: %v", err)
	}
	if b.quality, err = referencespkg.NewQualityService(dataDir); err != nil {
		log.Printf("Warning: failed to initialize QualityService: %v", err)
	}
	if b.siteCodes, b.siteCodesErr = referencespkg.NewSiteCodeService(dataDir); b.siteCodesErr != nil {
		log.Printf("Warning: failed to initialize SiteCodeService: %v", b.siteCodesErr)
	}
	if b.regionCodes, err = referencespkg.NewRegionCodeService(dataDir); err != nil {
		log.Printf("Warning: failed to initialize RegionCodeService: %v", err)
	}
	if b.soilCodes, err = referencespkg.NewSoilCodeService(dataDir); err != nil {
		log.Printf("Warning: failed to initialize SoilCodeService: %v", err)
	}
	if b.geologyCodes, err = referencespkg.NewGeologyCodeService(dataDir); err != nil {
		log.Printf("Warning: failed to initialize GeologyCodeService: %v", err)
	}
	if b.parentCodes, b.parentCodesErr = referencespkg.NewParentCodeService(dataDir); b.parentCodesErr != nil {
		log.Printf("Warning: failed to initialize ParentCodeService: %v", b.parentCodesErr)
	}
	return b
}

func (b *referenceCatalogBundle) Close() {
	if b == nil {
		return
	}
	type namedCloser struct {
		name  string
		close func() error
	}
	var closers []namedCloser
	if b.references != nil {
		closers = append(closers, namedCloser{"ReferenceService", b.references.Close})
	}
	if b.parentCodes != nil {
		closers = append(closers, namedCloser{"ParentCodeService", b.parentCodes.Close})
	}
	if b.geologyCodes != nil {
		closers = append(closers, namedCloser{"GeologyCodeService", b.geologyCodes.Close})
	}
	if b.soilCodes != nil {
		closers = append(closers, namedCloser{"SoilCodeService", b.soilCodes.Close})
	}
	if b.regionCodes != nil {
		closers = append(closers, namedCloser{"RegionCodeService", b.regionCodes.Close})
	}
	if b.siteCodes != nil {
		closers = append(closers, namedCloser{"SiteCodeService", b.siteCodes.Close})
	}
	if b.bec != nil {
		closers = append(closers, namedCloser{"BECService", b.bec.Close})
	}
	for _, item := range closers {
		if err := item.close(); err != nil {
			log.Printf("Warning: failed to close %s: %v", item.name, err)
		}
	}
}

type PlotEntryService struct {
	contexts                     *ContextService
	plots                        *vprocorepkg.PlotService
	siviParentShared             *sourceformpkg.SIVIParentSharedService
	siviCovers                   *sivipkg.SIVICoverService
	siviCombined                 *sivipkg.SIVICombinedService
	siviCollected                *sivipkg.SIVICollectedService
	siviSpecies                  *sourceformpkg.SIVISpeciesService
	siviIdentity                 *sivipkg.SIVIIdentityService
	siviCreation                 *sivipkg.SIVICreationService
	siviCreationUndo             *sivipkg.SIVICreationUndoService
	siviDeletion                 *sivipkg.SIVIDeletionService
	twoPageParentExtra           *twopagepkg.TwoPageParentExtraService
	twoPageParentCommon          *twopagepkg.TwoPageParentCommonService
	twoPageParentEntry           *twopagepkg.TwoPageParentEntryService
	twoPageSoil                  *twopagepkg.TwoPageSoilService
	twoPageVegetation            *twopagepkg.TwoPageVegetationService
	twoPageVegetationSpeciesList *twopagepkg.TwoPageVegetationSpeciesListService
	twoPageSourceAudit           *twopagepkg.TwoPageSourceAuditService
	twoPageProjectMetadata       *classificationpkg.TwoPageProjectMetadataService
	references                   *referencespkg.ReferenceService
	siteCodes                    *referencespkg.SiteCodeService
	soilCodes                    *referencespkg.SoilCodeService
	parentCodes                  *referencespkg.ParentCodeService
	geologyCodes                 *referencespkg.GeologyCodeService
	regionCodes                  *referencespkg.RegionCodeService
	quality                      *referencespkg.QualityService
}

func buildPlotEntryService(contexts *ContextService, plots *vprocorepkg.PlotService, refs *referenceCatalogBundle) (PlotEntryService, error) {
	core := unwrapContextService(contexts)
	siviParentShared, err := sourceformpkg.NewSIVIParentSharedService(core)
	if err != nil {
		return PlotEntryService{}, err
	}
	siviCovers, err := sivipkg.NewSIVICoverService(core)
	if err != nil {
		return PlotEntryService{}, err
	}
	siviCombined, err := sivipkg.NewSIVICombinedService(core)
	if err != nil {
		return PlotEntryService{}, err
	}
	siviCollected, err := sivipkg.NewSIVICollectedService(core)
	if err != nil {
		return PlotEntryService{}, err
	}
	siviSpecies, err := sourceformpkg.NewSIVISpeciesService(core)
	if err != nil {
		return PlotEntryService{}, err
	}
	siviIdentity, err := sivipkg.NewSIVIIdentityService(core)
	if err != nil {
		return PlotEntryService{}, err
	}
	siviCreation, err := sivipkg.NewSIVICreationService(core)
	if err != nil {
		return PlotEntryService{}, err
	}
	siviCreationUndo, err := sivipkg.NewSIVICreationUndoService(core)
	if err != nil {
		return PlotEntryService{}, err
	}
	siviDeletion, err := sivipkg.NewSIVIDeletionService(core)
	if err != nil {
		return PlotEntryService{}, err
	}

	var refBundle referenceCatalogBundle
	if refs != nil {
		refBundle = *refs
	}
	siviParentShared.References = sourceformpkg.SiviParentSharedReferenceReaders{
		Site:    refBundle.siteCodes,
		Region:  refBundle.regionCodes,
		Parent:  refBundle.parentCodes,
		Geology: refBundle.geologyCodes,
		Bec:     refBundle.bec,
	}
	entryReaders := twopagepkg.TwoPageEntryReferenceReaders{}
	if refBundle.siteCodes != nil {
		entryReaders.Shared.Site = refBundle.siteCodes
	}
	if refBundle.regionCodes != nil {
		entryReaders.Shared.Region = refBundle.regionCodes
		entryReaders.Ecosection = refBundle.regionCodes
	}
	if refBundle.parentCodes != nil {
		entryReaders.Shared.Parent = refBundle.parentCodes
	}
	if refBundle.geologyCodes != nil {
		entryReaders.Shared.Geology = refBundle.geologyCodes
	}
	if refBundle.bec != nil {
		entryReaders.Shared.Bec = refBundle.bec
		entryReaders.Series = refBundle.bec
	}
	if refBundle.quality != nil {
		entryReaders.Quality = refBundle.quality
	}
	if refBundle.soilCodes != nil {
		entryReaders.Soil = refBundle.soilCodes
	}

	twoPageParentExtra, err := twopagepkg.NewTwoPageParentExtraService(core)
	if err != nil {
		return PlotEntryService{}, err
	}
	twoPageParentCommon, err := twopagepkg.NewTwoPageParentCommonService(core)
	if err != nil {
		return PlotEntryService{}, err
	}
	twoPageParentEntry, err := twopagepkg.NewTwoPageParentEntryService(core, entryReaders)
	if err != nil {
		return PlotEntryService{}, err
	}
	twoPageSoil, err := twopagepkg.NewTwoPageSoilService(core)
	if err != nil {
		return PlotEntryService{}, err
	}
	twoPageVegetation, err := twopagepkg.NewTwoPageVegetationService(core)
	if err != nil {
		return PlotEntryService{}, err
	}
	twoPageVegetationSpeciesList, err := twopagepkg.NewTwoPageVegetationSpeciesListService(core)
	if err != nil {
		return PlotEntryService{}, err
	}
	twoPageSourceAudit, err := twopagepkg.NewTwoPageSourceAuditService(core)
	if err != nil {
		return PlotEntryService{}, err
	}
	twoPageProjectMetadata, err := classificationpkg.NewTwoPageProjectMetadataService(core)
	if err != nil {
		return PlotEntryService{}, err
	}

	return PlotEntryService{
		contexts:                     contexts,
		plots:                        plots,
		siviParentShared:             siviParentShared,
		siviCovers:                   siviCovers,
		siviCombined:                 siviCombined,
		siviCollected:                siviCollected,
		siviSpecies:                  siviSpecies,
		siviIdentity:                 siviIdentity,
		siviCreation:                 siviCreation,
		siviCreationUndo:             siviCreationUndo,
		siviDeletion:                 siviDeletion,
		twoPageParentExtra:           twoPageParentExtra,
		twoPageParentCommon:          twoPageParentCommon,
		twoPageParentEntry:           twoPageParentEntry,
		twoPageSoil:                  twoPageSoil,
		twoPageVegetation:            twoPageVegetation,
		twoPageVegetationSpeciesList: twoPageVegetationSpeciesList,
		twoPageSourceAudit:           twoPageSourceAudit,
		twoPageProjectMetadata:       twoPageProjectMetadata,
		references:                   refBundle.references,
		siteCodes:                    refBundle.siteCodes,
		soilCodes:                    refBundle.soilCodes,
		parentCodes:                  refBundle.parentCodes,
		geologyCodes:                 refBundle.geologyCodes,
		regionCodes:                  refBundle.regionCodes,
		quality:                      refBundle.quality,
	}, nil
}

// PlotService delegates

func (s *PlotEntryService) CanEditMasterBEC() bool {
	if s == nil || s.plots == nil {
		return false
	}
	return s.plots.CanEditMasterBEC()
}

func (s *PlotEntryService) CreatePlot(h FS882Header) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.CreatePlot(h)
}

func (s *PlotEntryService) CreateSourceVegetation(plot string, request VegetationCreationRequest) (int64, error) {
	if s == nil || s.plots == nil {
		return 0, errors.New("plot service is unavailable")
	}
	return s.plots.CreateSourceVegetation(plot, request)
}

func (s *PlotEntryService) DeleteHumusRecord(plotNumber string, id int64) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.DeleteHumusRecord(plotNumber, id)
}

func (s *PlotEntryService) DeleteMineralRecord(plotNumber string, id int64) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.DeleteMineralRecord(plotNumber, id)
}

func (s *PlotEntryService) DeleteOtherRecord(plotNumber string, id int64) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.DeleteOtherRecord(plotNumber, id)
}

func (s *PlotEntryService) DeleteReviewedVegetation(plot string, request VegetationDeletionRequest) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return plotentry.DeleteReviewedVegetation(s.plots, plot, request)
}

func (s *PlotEntryService) DeleteVegRecord(plotNumber string, id int64) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.DeleteVegRecord(plotNumber, id)
}

func (s *PlotEntryService) GetChildCapabilities(kind string) (map[string]bool, error) {
	if s == nil || s.plots == nil {
		return nil, errors.New("plot service is unavailable")
	}
	return s.plots.GetChildCapabilities(kind)
}

func (s *PlotEntryService) GetHeaderCapabilities() (map[string]bool, error) {
	if s == nil || s.plots == nil {
		return nil, errors.New("plot service is unavailable")
	}
	return s.plots.GetHeaderCapabilities()
}

func (s *PlotEntryService) GetPlot(plotNumber string) (*FS882Header, error) {
	if s == nil || s.plots == nil {
		return nil, errors.New("plot service is unavailable")
	}
	return s.plots.GetPlot(plotNumber)
}

func (s *PlotEntryService) ListAuditEntries(plotNumber string) ([]AuditEntry, error) {
	if s == nil || s.plots == nil {
		return nil, errors.New("plot service is unavailable")
	}
	return s.plots.ListAuditEntries(plotNumber)
}

func (s *PlotEntryService) ListAuditReadEntries(plot string) ([]OrdinaryAuditReadEntry, error) {
	if s == nil || s.plots == nil {
		return nil, errors.New("plot service is unavailable")
	}
	return s.plots.ListAuditReadEntries(plot)
}

func (s *PlotEntryService) ListHumusRecords(plotNumber string) ([]HumusRecord, error) {
	if s == nil || s.plots == nil {
		return nil, errors.New("plot service is unavailable")
	}
	return s.plots.ListHumusRecords(plotNumber)
}

func (s *PlotEntryService) ListMineralRecords(plotNumber string) ([]MineralRecord, error) {
	if s == nil || s.plots == nil {
		return nil, errors.New("plot service is unavailable")
	}
	return s.plots.ListMineralRecords(plotNumber)
}

func (s *PlotEntryService) ListOtherRecords(plotNumber string) ([]OtherRecord, error) {
	if s == nil || s.plots == nil {
		return nil, errors.New("plot service is unavailable")
	}
	return s.plots.ListOtherRecords(plotNumber)
}

func (s *PlotEntryService) ListVegRecords(plotNumber string) ([]VegRecord, error) {
	if s == nil || s.plots == nil {
		return nil, errors.New("plot service is unavailable")
	}
	return s.plots.ListVegRecords(plotNumber)
}

func (s *PlotEntryService) RestoreAuditRecords(plotNumber string, removeAuditRows bool) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.RestoreAuditRecords(plotNumber, removeAuditRows)
}

func (s *PlotEntryService) RestoreSelectedAuditRecords(plotNumber string, rowIDs []string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if s == nil || s.plots == nil {
		return nil, errors.New("plot service is unavailable")
	}
	return s.plots.RestoreSelectedAuditRecords(plotNumber, rowIDs, vprocorepkg.AuditRestoreAction(action))
}

func (s *PlotEntryService) SaveHumusRecord(r HumusRecord) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.SaveHumusRecord(r)
}

func (s *PlotEntryService) SaveMineralRecord(r MineralRecord) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.SaveMineralRecord(r)
}

func (s *PlotEntryService) SaveOtherRecord(r OtherRecord) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.SaveOtherRecord(r)
}

func (s *PlotEntryService) SavePlot(h FS882Header) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.SavePlot(h)
}

func (s *PlotEntryService) SaveVegRecord(r VegRecord) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.SaveVegRecord(r)
}

func (s *PlotEntryService) SetAuditRestoreSelection(plotNumber string, rowIDs []string) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.SetAuditRestoreSelection(plotNumber, rowIDs)
}

func (s *PlotEntryService) SetAuditStrength(strength int) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.SetAuditStrength(strength)
}

func (s *PlotEntryService) SetCurrentUser(user string) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.SetCurrentUser(user)
}

func (s *PlotEntryService) UpdateCollectedRecords(plot string, updates []CollectedRecordUpdate) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.UpdateCollectedRecords(plot, updates)
}

func (s *PlotEntryService) UpdateHeightRecords(plotNumber string, updates []HeightRecordUpdate) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.UpdateHeightRecords(plotNumber, updates)
}

func (s *PlotEntryService) UpdateHumusRecord(r HumusRecord) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.UpdateHumusRecord(r)
}

func (s *PlotEntryService) UpdateMineralRecord(r MineralRecord) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.UpdateMineralRecord(r)
}

func (s *PlotEntryService) UpdateOtherRecord(r OtherRecord) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.UpdateOtherRecord(r)
}

func (s *PlotEntryService) UpdateOtherRecords(plot string, updates []OtherRecordUpdate) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.UpdateOtherRecords(plot, updates)
}

func (s *PlotEntryService) UpdatePlot(h FS882Header) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.UpdatePlot(h)
}

func (s *PlotEntryService) UpdateSoilRecords(plot string, updates []SoilRecordUpdate) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.UpdateSoilRecords(plot, updates)
}

func (s *PlotEntryService) UpdateVegRecord(r VegRecord) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.UpdateVegRecord(r)
}

func (s *PlotEntryService) UpdateVegetationAttributes(plot string, updates []VegetationAttributeUpdate) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return plotentry.UpdateVegetationAttributes(s.plots, plot, updates)
}

func (s *PlotEntryService) UpdateVegetationNumbers(plot string, updates []VegetationNumberUpdate) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.UpdateVegetationNumbers(plot, updates)
}

func (s *PlotEntryService) UpdateVegetationSpecies(plot string, updates []VegetationSpeciesUpdate) error {
	if s == nil || s.plots == nil {
		return errors.New("plot service is unavailable")
	}
	return s.plots.UpdateVegetationSpecies(plot, updates)
}

// SIVI entry/cover/height/species/collected/combined/identity/creation/deletion/undo delegates

func (s *PlotEntryService) GetSIVIParentSharedOriginal(ctx context.Context, contextID, plot string) (*SIVIParentProjection, error) {
	if s == nil || s.siviParentShared == nil {
		return nil, errors.New("SIVI parent shared service is unavailable")
	}
	return s.siviParentShared.GetOriginal(ctx, contextID, plot)
}

func (s *PlotEntryService) GetSIVIParentSharedReferences(ctx context.Context, contextID, plot string, zone ProjectMetadataCell) (*SIVIParentSharedReferences, error) {
	if s == nil || s.siviParentShared == nil {
		return nil, errors.New("SIVI parent shared service is unavailable")
	}
	return s.siviParentShared.GetReferences(ctx, contextID, plot, zone)
}

func (s *PlotEntryService) SaveSIVIParentShared(ctx context.Context, contextID, plot string, request SIVIParentSharedWrite) (*SIVIParentWriteResult, error) {
	if s == nil || s.siviParentShared == nil {
		return nil, errors.New("SIVI parent shared service is unavailable")
	}
	return s.siviParentShared.Save(ctx, contextID, plot, request)
}

func (s *PlotEntryService) RestoreSIVIParentShared(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if s == nil || s.siviParentShared == nil {
		return nil, errors.New("SIVI parent shared service is unavailable")
	}
	return s.siviParentShared.Restore(ctx, contextID, plot, historyID, vprocorepkg.AuditRestoreAction(action))
}

func (s *PlotEntryService) GetSIVICoverOriginal(ctx context.Context, contextID, plot string, extended bool) ([]SIVIVegetationProjection, error) {
	if s == nil || s.siviCovers == nil {
		return nil, errors.New("SIVI cover service is unavailable")
	}
	return s.siviCovers.GetOriginal(ctx, contextID, plot, extended)
}

func (s *PlotEntryService) SaveSIVICoverReviewed(ctx context.Context, contextID, plot string, extended bool, requestJSON string) (*SIVICoverWriteResult, error) {
	if s == nil || s.siviCovers == nil {
		return nil, errors.New("SIVI cover service is unavailable")
	}
	return s.siviCovers.SaveReviewed(ctx, contextID, plot, extended, requestJSON)
}

func (s *PlotEntryService) RestoreSIVICoverReviewed(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if s == nil || s.siviCovers == nil {
		return nil, errors.New("SIVI cover service is unavailable")
	}
	return s.siviCovers.RestoreReviewed(ctx, contextID, plot, historyID, vprocorepkg.AuditRestoreAction(action))
}

func (s *PlotEntryService) GetSIVICombinedOriginal(ctx context.Context, contextID, plot string, extended bool) ([]SIVIVegetationProjection, error) {
	if s == nil || s.siviCombined == nil {
		return nil, errors.New("SIVI combined service is unavailable")
	}
	return s.siviCombined.GetOriginal(ctx, contextID, plot, extended)
}

func (s *PlotEntryService) SaveSIVICombinedReviewed(ctx context.Context, contextID, plot string, extended bool, requestJSON string) (*SIVIHeightWriteResult, error) {
	if s == nil || s.siviCombined == nil {
		return nil, errors.New("SIVI combined service is unavailable")
	}
	return s.siviCombined.SaveReviewed(ctx, contextID, plot, extended, requestJSON)
}

func (s *PlotEntryService) RestoreSIVICombinedReviewed(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if s == nil || s.siviCombined == nil {
		return nil, errors.New("SIVI combined service is unavailable")
	}
	return s.siviCombined.RestoreReviewed(ctx, contextID, plot, historyID, vprocorepkg.AuditRestoreAction(action))
}

func (s *PlotEntryService) GetSIVICollectedOriginal(ctx context.Context, contextID, plot string, extended bool) ([]SIVIVegetationProjection, error) {
	if s == nil || s.siviCollected == nil {
		return nil, errors.New("SIVI collected service is unavailable")
	}
	return s.siviCollected.GetOriginal(ctx, contextID, plot, extended)
}

func (s *PlotEntryService) SaveSIVICollectedReviewed(ctx context.Context, contextID, plot string, extended bool, requestJSON string) (*SIVIHeightWriteResult, error) {
	if s == nil || s.siviCollected == nil {
		return nil, errors.New("SIVI collected service is unavailable")
	}
	return s.siviCollected.SaveReviewed(ctx, contextID, plot, extended, requestJSON)
}

func (s *PlotEntryService) RestoreSIVICollectedReviewed(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if s == nil || s.siviCollected == nil {
		return nil, errors.New("SIVI collected service is unavailable")
	}
	return s.siviCollected.RestoreReviewed(ctx, contextID, plot, historyID, vprocorepkg.AuditRestoreAction(action))
}

func (s *PlotEntryService) GetSIVISpeciesOriginal(ctx context.Context, contextID, plot string, extended bool) ([]SIVIVegetationProjection, error) {
	if s == nil || s.siviSpecies == nil {
		return nil, errors.New("SIVI species service is unavailable")
	}
	return s.siviSpecies.GetOriginal(ctx, contextID, plot, extended)
}

func (s *PlotEntryService) GetSIVISpeciesReferences(ctx context.Context, contextID, plot string) (SIVISpeciesReferences, error) {
	if s == nil || s.siviSpecies == nil {
		return SIVISpeciesReferences{}, errors.New("SIVI species service is unavailable")
	}
	return s.siviSpecies.GetReferences(ctx, contextID, plot)
}

func (s *PlotEntryService) SaveSIVISpeciesReviewed(ctx context.Context, contextID, plot string, extended bool, requestJSON string) (*SIVIHeightWriteResult, error) {
	if s == nil || s.siviSpecies == nil {
		return nil, errors.New("SIVI species service is unavailable")
	}
	return s.siviSpecies.SaveReviewed(ctx, contextID, plot, extended, requestJSON)
}

func (s *PlotEntryService) RestoreSIVISpeciesReviewed(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if s == nil || s.siviSpecies == nil {
		return nil, errors.New("SIVI species service is unavailable")
	}
	return s.siviSpecies.RestoreReviewed(ctx, contextID, plot, historyID, vprocorepkg.AuditRestoreAction(action))
}

func (s *PlotEntryService) GetSIVIIdentityOriginal(ctx context.Context, contextID, plot string, extended bool) ([]SIVIVegetationProjection, error) {
	if s == nil || s.siviIdentity == nil {
		return nil, errors.New("SIVI identity service is unavailable")
	}
	return s.siviIdentity.GetOriginal(ctx, contextID, plot, extended)
}

func (s *PlotEntryService) SaveSIVIIdentityReviewed(ctx context.Context, contextID, plot string, extended bool, requestJSON string) (*SIVIHeightWriteResult, error) {
	if s == nil || s.siviIdentity == nil {
		return nil, errors.New("SIVI identity service is unavailable")
	}
	return s.siviIdentity.SaveReviewed(ctx, contextID, plot, extended, requestJSON)
}

func (s *PlotEntryService) RestoreSIVIIdentityReviewed(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if s == nil || s.siviIdentity == nil {
		return nil, errors.New("SIVI identity service is unavailable")
	}
	return s.siviIdentity.RestoreReviewed(ctx, contextID, plot, historyID, vprocorepkg.AuditRestoreAction(action))
}

func (s *PlotEntryService) CreateSIVIVegetation(ctx context.Context, contextID, requestJSON string) (*SIVICreationResult, error) {
	if s == nil || s.siviCreation == nil {
		return nil, errors.New("SIVI creation service is unavailable")
	}
	return s.siviCreation.Create(ctx, contextID, requestJSON)
}

func (s *PlotEntryService) GetSIVICreationReferences(ctx context.Context, contextID, plot string) (SIVISpeciesReferences, error) {
	if s == nil || s.siviCreation == nil {
		return SIVISpeciesReferences{}, errors.New("SIVI creation service is unavailable")
	}
	return s.siviCreation.GetReferences(ctx, contextID, plot)
}

func (s *PlotEntryService) LookupSIVICreationReceipt(ctx context.Context, contextID, requestJSON string) (*SIVICreationResult, error) {
	if s == nil || s.siviCreation == nil {
		return nil, errors.New("SIVI creation service is unavailable")
	}
	return s.siviCreation.LookupReceipt(ctx, contextID, requestJSON)
}

func (s *PlotEntryService) GetSIVICreationUndoHistory(ctx context.Context, contextID, plot string) (*SIVICreationUndoHistoryList, error) {
	if s == nil || s.siviCreationUndo == nil {
		return nil, errors.New("SIVI creation undo service is unavailable")
	}
	return s.siviCreationUndo.GetHistory(ctx, contextID, plot)
}

func (s *PlotEntryService) LookupSIVICreationUndoReceipt(ctx context.Context, contextID, requestJSON string) (*SIVICreationUndoResult, error) {
	if s == nil || s.siviCreationUndo == nil {
		return nil, errors.New("SIVI creation undo service is unavailable")
	}
	return s.siviCreationUndo.LookupReceipt(ctx, contextID, requestJSON)
}

func (s *PlotEntryService) ReviewSIVICreationUndo(ctx context.Context, contextID, plot, historyID string) (*SIVICreationUndoReview, error) {
	if s == nil || s.siviCreationUndo == nil {
		return nil, errors.New("SIVI creation undo service is unavailable")
	}
	return s.siviCreationUndo.Review(ctx, contextID, plot, historyID)
}

func (s *PlotEntryService) UndoSIVICreation(ctx context.Context, contextID, requestJSON string) (*SIVICreationUndoResult, error) {
	if s == nil || s.siviCreationUndo == nil {
		return nil, errors.New("SIVI creation undo service is unavailable")
	}
	return s.siviCreationUndo.Undo(ctx, contextID, requestJSON)
}

func (s *PlotEntryService) DeleteSIVIVegetation(ctx context.Context, contextID, requestJSON string) (*SIVIDeletionResult, error) {
	if s == nil || s.siviDeletion == nil {
		return nil, errors.New("SIVI deletion service is unavailable")
	}
	return s.siviDeletion.Delete(ctx, contextID, requestJSON)
}

func (s *PlotEntryService) GetSIVIDeletionHistory(ctx context.Context, contextID, plot string) (*SIVIDeletionHistoryList, error) {
	if s == nil || s.siviDeletion == nil {
		return nil, errors.New("SIVI deletion service is unavailable")
	}
	return s.siviDeletion.GetHistory(ctx, contextID, plot)
}

func (s *PlotEntryService) GetSIVIDeletionOriginal(ctx context.Context, contextID, plot, form, rowID string) (*SIVIDeletionOriginal, error) {
	if s == nil || s.siviDeletion == nil {
		return nil, errors.New("SIVI deletion service is unavailable")
	}
	return s.siviDeletion.GetOriginal(ctx, contextID, plot, form, rowID)
}

func (s *PlotEntryService) GetSIVIDeletionTargets(ctx context.Context, contextID, plot string) (*SIVIDeletionTargets, error) {
	if s == nil || s.siviDeletion == nil {
		return nil, errors.New("SIVI deletion service is unavailable")
	}
	return s.siviDeletion.GetTargets(ctx, contextID, plot)
}

func (s *PlotEntryService) LookupSIVIDeletionReceipt(ctx context.Context, contextID, requestJSON string) (*SIVIDeletionResult, error) {
	if s == nil || s.siviDeletion == nil {
		return nil, errors.New("SIVI deletion service is unavailable")
	}
	return s.siviDeletion.LookupReceipt(ctx, contextID, requestJSON)
}

func (s *PlotEntryService) LookupSIVIDeletionRestorationReceipt(ctx context.Context, contextID, requestJSON string) (*SIVIDeletionRestorationResult, error) {
	if s == nil || s.siviDeletion == nil {
		return nil, errors.New("SIVI deletion service is unavailable")
	}
	return s.siviDeletion.LookupRestorationReceipt(ctx, contextID, requestJSON)
}

func (s *PlotEntryService) RestoreSIVIDeletion(ctx context.Context, contextID, requestJSON string) (*SIVIDeletionRestorationResult, error) {
	if s == nil || s.siviDeletion == nil {
		return nil, errors.New("SIVI deletion service is unavailable")
	}
	return s.siviDeletion.Restore(ctx, contextID, requestJSON)
}

func (s *PlotEntryService) ReviewSIVIDeletionRestoration(ctx context.Context, contextID, plot, historyID string) (*SIVIDeletionRestorationReview, error) {
	if s == nil || s.siviDeletion == nil {
		return nil, errors.New("SIVI deletion service is unavailable")
	}
	return s.siviDeletion.ReviewRestoration(ctx, contextID, plot, historyID)
}

// Two-Page entry, soil, vegetation, audit & metadata delegates

func (s *PlotEntryService) GetTwoPageParentExtraOriginal(ctx context.Context, contextID, plot, form string) (*SIVIParentProjection, error) {
	if s == nil || s.twoPageParentExtra == nil {
		return nil, errors.New("two-page parent extra service is unavailable")
	}
	return s.twoPageParentExtra.GetOriginal(ctx, contextID, plot, form)
}

func (s *PlotEntryService) SaveTwoPageParentExtra(ctx context.Context, contextID, plot, form string, request TwoPageParentExtraWrite) (*SIVIParentWriteResult, error) {
	if s == nil || s.twoPageParentExtra == nil {
		return nil, errors.New("two-page parent extra service is unavailable")
	}
	return s.twoPageParentExtra.Save(ctx, contextID, plot, form, request)
}

func (s *PlotEntryService) RestoreTwoPageParentExtra(ctx context.Context, contextID, plot, form, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if s == nil || s.twoPageParentExtra == nil {
		return nil, errors.New("two-page parent extra service is unavailable")
	}
	return s.twoPageParentExtra.Restore(ctx, contextID, plot, form, historyID, vprocorepkg.AuditRestoreAction(action))
}

func (s *PlotEntryService) GetTwoPageParentCommonOriginal(ctx context.Context, contextID, plot, form string) (*SIVIParentProjection, error) {
	if s == nil || s.twoPageParentCommon == nil {
		return nil, errors.New("two-page parent common service is unavailable")
	}
	return s.twoPageParentCommon.GetOriginal(ctx, contextID, plot, form)
}

func (s *PlotEntryService) SaveTwoPageParentCommon(ctx context.Context, contextID, plot, form string, request TwoPageParentCommonWrite) (*SIVIParentWriteResult, error) {
	if s == nil || s.twoPageParentCommon == nil {
		return nil, errors.New("two-page parent common service is unavailable")
	}
	return s.twoPageParentCommon.Save(ctx, contextID, plot, form, request)
}

func (s *PlotEntryService) RestoreTwoPageParentCommon(ctx context.Context, contextID, plot, form, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if s == nil || s.twoPageParentCommon == nil {
		return nil, errors.New("two-page parent common service is unavailable")
	}
	return s.twoPageParentCommon.Restore(ctx, contextID, plot, form, historyID, vprocorepkg.AuditRestoreAction(action))
}

func (s *PlotEntryService) GetTwoPageParentEntryOriginal(ctx context.Context, contextID, plot, form string) (*TwoPageEntryReferenceSnapshot, error) {
	if s == nil || s.twoPageParentEntry == nil {
		return nil, errors.New("two-page parent entry service is unavailable")
	}
	return s.twoPageParentEntry.GetOriginal(ctx, contextID, plot, form)
}

func (s *PlotEntryService) GetTwoPageParentEntryReferences(ctx context.Context, contextID, plot, form string, filters TwoPageEntryReferenceFilters) (*TwoPageEntryReferenceSnapshot, error) {
	if s == nil || s.twoPageParentEntry == nil {
		return nil, errors.New("two-page parent entry service is unavailable")
	}
	return s.twoPageParentEntry.GetReferences(ctx, contextID, plot, form, filters)
}

func (s *PlotEntryService) SaveTwoPageParentEntry(ctx context.Context, contextID, plot, form string, request TwoPageParentEntryWrite) (*SIVIParentWriteResult, error) {
	if s == nil || s.twoPageParentEntry == nil {
		return nil, errors.New("two-page parent entry service is unavailable")
	}
	return s.twoPageParentEntry.Save(ctx, contextID, plot, form, request)
}

func (s *PlotEntryService) RestoreTwoPageParentEntry(ctx context.Context, contextID, plot, form, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if s == nil || s.twoPageParentEntry == nil {
		return nil, errors.New("two-page parent entry service is unavailable")
	}
	return s.twoPageParentEntry.Restore(ctx, contextID, plot, form, historyID, vprocorepkg.AuditRestoreAction(action))
}

func (s *PlotEntryService) CreateTwoPageSoilHorizon(ctx context.Context, contextID, requestJSON string) (*TwoPageSoilWriteResult, error) {
	if s == nil || s.twoPageSoil == nil {
		return nil, errors.New("two-page soil service is unavailable")
	}
	return s.twoPageSoil.CreateHorizon(ctx, contextID, requestJSON)
}

func (s *PlotEntryService) GetTwoPageSoilOriginal(ctx context.Context, contextID, plot, form string) (*TwoPageSoilOriginal, error) {
	if s == nil || s.twoPageSoil == nil {
		return nil, errors.New("two-page soil service is unavailable")
	}
	return s.twoPageSoil.GetOriginal(ctx, contextID, plot, form)
}

func (s *PlotEntryService) GetTwoPageSoilReferences(ctx context.Context, contextID, plot, form string) (*TwoPageSoilReferences, error) {
	if s == nil || s.twoPageSoil == nil {
		return nil, errors.New("two-page soil service is unavailable")
	}
	return s.twoPageSoil.GetReferences(ctx, contextID, plot, form)
}

func (s *PlotEntryService) RecoverTwoPageSoilOutcome(ctx context.Context, contextID, requestJSON string) (*TwoPageSoilAttemptOutcome, error) {
	if s == nil || s.twoPageSoil == nil {
		return nil, errors.New("two-page soil service is unavailable")
	}
	return s.twoPageSoil.RecoverOutcome(ctx, contextID, requestJSON)
}

func (s *PlotEntryService) RecoverTwoPageSoilReceipt(ctx context.Context, contextID, requestJSON string) (*TwoPageSoilWriteResult, error) {
	if s == nil || s.twoPageSoil == nil {
		return nil, errors.New("two-page soil service is unavailable")
	}
	return s.twoPageSoil.RecoverReceipt(ctx, contextID, requestJSON)
}

func (s *PlotEntryService) SaveTwoPageSoil(ctx context.Context, contextID, requestJSON string) (*TwoPageSoilWriteResult, error) {
	if s == nil || s.twoPageSoil == nil {
		return nil, errors.New("two-page soil service is unavailable")
	}
	return s.twoPageSoil.Save(ctx, contextID, requestJSON)
}

func (s *PlotEntryService) GetTwoPageVegetationOriginal(ctx context.Context, contextID, plot, form string) (*TwoPageVegetationOriginal, error) {
	if s == nil || s.twoPageVegetation == nil {
		return nil, errors.New("two-page vegetation service is unavailable")
	}
	return s.twoPageVegetation.GetOriginal(ctx, contextID, plot, form)
}

func (s *PlotEntryService) GetTwoPageVegetationSpeciesReferences(ctx context.Context, contextID, plot, form string) (*TwoPageVegetationSpeciesReferences, error) {
	if s == nil || s.twoPageVegetation == nil {
		return nil, errors.New("two-page vegetation service is unavailable")
	}
	return s.twoPageVegetation.GetSpeciesReferences(ctx, contextID, plot, form)
}

func (s *PlotEntryService) RecoverTwoPageVegetationOutcome(ctx context.Context, contextID, requestJSON string) (*TwoPageVegetationAttemptOutcome, error) {
	if s == nil || s.twoPageVegetation == nil {
		return nil, errors.New("two-page vegetation service is unavailable")
	}
	return s.twoPageVegetation.RecoverOutcome(ctx, contextID, requestJSON)
}

func (s *PlotEntryService) RecoverTwoPageVegetationReceipt(ctx context.Context, contextID, requestJSON string) (*TwoPageVegetationWriteResult, error) {
	if s == nil || s.twoPageVegetation == nil {
		return nil, errors.New("two-page vegetation service is unavailable")
	}
	return s.twoPageVegetation.RecoverReceipt(ctx, contextID, requestJSON)
}

func (s *PlotEntryService) SaveTwoPageVegetation(ctx context.Context, contextID, requestJSON string) (*TwoPageVegetationWriteResult, error) {
	if s == nil || s.twoPageVegetation == nil {
		return nil, errors.New("two-page vegetation service is unavailable")
	}
	return s.twoPageVegetation.Save(ctx, contextID, requestJSON)
}

func (s *PlotEntryService) SaveTwoPageVegetationSpeciesList(ctx context.Context, contextID, requestJSON string) (*TwoPageVegetationWriteResult, error) {
	if s == nil || s.twoPageVegetationSpeciesList == nil {
		return nil, errors.New("two-page vegetation species list service is unavailable")
	}
	return s.twoPageVegetationSpeciesList.Save(ctx, contextID, requestJSON)
}

func (s *PlotEntryService) GetTwoPageSourceAuditOriginal(ctx context.Context, contextID, plot, form string) (*TwoPageSourceAuditOriginal, error) {
	if s == nil || s.twoPageSourceAudit == nil {
		return nil, errors.New("two-page source audit service is unavailable")
	}
	return s.twoPageSourceAudit.GetOriginal(ctx, contextID, plot, form)
}

func (s *PlotEntryService) GetTwoPageProjectMetadataReview(ctx context.Context, contextID, plot string, offset, limit int) (*TwoPageProjectMetadataReview, error) {
	if s == nil || s.twoPageProjectMetadata == nil {
		return nil, errors.New("two-page project metadata service is unavailable")
	}
	return s.twoPageProjectMetadata.GetReview(ctx, contextID, plot, offset, limit)
}

// Reference picklist lookup delegates

func (s *PlotEntryService) GetAllLists(ctx context.Context) (map[string][]ListItem, error) {
	if s == nil || s.references == nil {
		return nil, errors.New("reference catalogue is unavailable")
	}
	return s.references.GetAllLists(ctx)
}

func (s *PlotEntryService) GetListItems(ctx context.Context, listName string) ([]ListItem, error) {
	if s == nil || s.references == nil {
		return nil, errors.New("reference catalogue is unavailable")
	}
	return s.references.GetListItems(ctx, listName)
}

func (s *PlotEntryService) GetSpecies(ctx context.Context, code string) (*SpeciesItem, error) {
	if s == nil || s.references == nil {
		return nil, errors.New("reference catalogue is unavailable")
	}
	return s.references.GetSpecies(ctx, code)
}

func (s *PlotEntryService) SearchSpecies(ctx context.Context, query string, limit int) ([]SpeciesItem, error) {
	if s == nil || s.references == nil {
		return nil, errors.New("reference catalogue is unavailable")
	}
	return s.references.SearchSpecies(ctx, query, limit)
}

func (s *PlotEntryService) ListExposureChoices(ctx context.Context) ([]SiteCodeChoice, error) {
	if s == nil || s.siteCodes == nil {
		return nil, errors.New("site code catalogue is unavailable")
	}
	return s.siteCodes.ListExposureChoices(ctx)
}

func (s *PlotEntryService) ListSiteDisturbanceChoices(ctx context.Context) ([]SiteCodeChoice, error) {
	if s == nil || s.siteCodes == nil {
		return nil, errors.New("site code catalogue is unavailable")
	}
	return s.siteCodes.ListSiteDisturbanceChoices(ctx)
}

func (s *PlotEntryService) ReloadSiteCodeCatalogue(ctx context.Context) error {
	if s == nil || s.siteCodes == nil {
		return errors.New("site code catalogue is unavailable")
	}
	return s.siteCodes.ReloadCatalogue(ctx)
}

func (s *PlotEntryService) ListGreatGroupChoices(ctx context.Context) ([]SoilCodeChoice, error) {
	if s == nil || s.soilCodes == nil {
		return nil, errors.New("soil code catalogue is unavailable")
	}
	return s.soilCodes.ListGreatGroupChoices(ctx)
}

func (s *PlotEntryService) ListSubgroupChoices(ctx context.Context) ([]SoilCodeChoice, error) {
	if s == nil || s.soilCodes == nil {
		return nil, errors.New("soil code catalogue is unavailable")
	}
	return s.soilCodes.ListSubgroupChoices(ctx)
}

func (s *PlotEntryService) ReloadSoilCodeCatalogue(ctx context.Context) error {
	if s == nil || s.soilCodes == nil {
		return errors.New("soil code catalogue is unavailable")
	}
	return s.soilCodes.ReloadCatalogue(ctx)
}

func (s *PlotEntryService) ListParentCodeChoices(ctx context.Context, listName string) ([]ParentCodeChoice, error) {
	if s == nil || s.parentCodes == nil {
		return nil, errors.New("parent code catalogue is unavailable")
	}
	return s.parentCodes.ListChoices(ctx, listName)
}

func (s *PlotEntryService) ReloadParentCodeCatalogue(ctx context.Context) error {
	if s == nil || s.parentCodes == nil {
		return errors.New("parent code catalogue is unavailable")
	}
	return s.parentCodes.ReloadCatalogue(ctx)
}

func (s *PlotEntryService) ListBedrockChoices(ctx context.Context) ([]GeologyCodeChoice, error) {
	if s == nil || s.geologyCodes == nil {
		return nil, errors.New("geology code catalogue is unavailable")
	}
	return s.geologyCodes.ListBedrockChoices(ctx)
}

func (s *PlotEntryService) ReloadGeologyCodeCatalogue(ctx context.Context) error {
	if s == nil || s.geologyCodes == nil {
		return errors.New("geology code catalogue is unavailable")
	}
	return s.geologyCodes.ReloadCatalogue(ctx)
}

func (s *PlotEntryService) ListEcosectionChoices(ctx context.Context) ([]RegionCodeChoice, error) {
	if s == nil || s.regionCodes == nil {
		return nil, errors.New("region code catalogue is unavailable")
	}
	return s.regionCodes.ListEcosectionChoices(ctx)
}

func (s *PlotEntryService) ListRegionChoices(ctx context.Context) ([]RegionCodeChoice, error) {
	if s == nil || s.regionCodes == nil {
		return nil, errors.New("region code catalogue is unavailable")
	}
	return s.regionCodes.ListRegionChoices(ctx)
}

func (s *PlotEntryService) ReloadRegionCodeCatalogue(ctx context.Context) error {
	if s == nil || s.regionCodes == nil {
		return errors.New("region code catalogue is unavailable")
	}
	return s.regionCodes.ReloadCatalogue(ctx)
}

func (s *PlotEntryService) ListPlotQualityChoices(ctx context.Context) ([]PlotQualityChoice, error) {
	if s == nil || s.quality == nil {
		return nil, errors.New("plot quality catalogue is unavailable")
	}
	return s.quality.ListPlotQualityChoices(ctx)
}

// LMH25 (2023) Pedon, Substrate, HumusThickness & rSMR/rSNR Rule Engine (Auto-2)

func (s *PlotEntryService) EvaluatePlotDiagnostics(header FS882Header, humus []HumusRecord, mineral []MineralRecord) lmh25rules.PlotDiagnosticsResult {
	return lmh25rules.EvaluatePlotDiagnostics(header, humus, mineral)
}

func (s *PlotEntryService) EvaluateStoredPlotDiagnostics(ctx context.Context, contextID, plotNumber string) (lmh25rules.PlotDiagnosticsResult, error) {
	if s == nil || unwrapContextService(s.contexts) == nil {
		return lmh25rules.PlotDiagnosticsResult{}, errors.New("plot entry context is unavailable")
	}
	return lmh25rules.EvaluateStoredPlotDiagnostics(unwrapContextService(s.contexts), ctx, contextID, plotNumber)
}
