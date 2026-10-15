import { LongVegetationProjectWorkbookService, LongVegetationReportScopeService } from '../bindings/github.com/boostao/vpro-wails';
import { ReadRequests } from './readRequests';
import { createProjectReportWorkspaceRegistry, type ProjectWorkspacePort } from './longVegetationProjectWorkspace.svelte';
import type { ProjectReportOwner } from './longVegetationProjectReport';

const capabilities = Object.freeze({
  enabled: import.meta.env.VITE_LONG_VEGETATION_PROJECT_SCOPE === 'true',
  workbook: import.meta.env.VITE_LONG_VEGETATION_WORKBOOK === 'true',
  lifeform: import.meta.env.VITE_LONG_VEGETATION_LIFEFORM === 'true',
  strata: import.meta.env.VITE_LONG_VEGETATION_STRATA === 'true',
  code: import.meta.env.VITE_LONG_VEGETATION_CODE === 'true',
  selectedField: import.meta.env.VITE_LONG_VEGETATION_SELECTED_FIELD === 'true',
});

function ownedPorts(owner: ProjectReportOwner): ProjectWorkspacePort {
  const scopeReads = new ReadRequests(), unitReads = new ReadRequests(), reportReads = new ReadRequests();
  const context = owner.contextId;
  const nonnull = <T>(value: T | null, operation: string): T => {
    if (value === null) throw new Error(`No owned project ${operation} acknowledgement returned.`);
    return value;
  };
  return {
    scope: {
      scope: async () => nonnull(await scopeReads.track(LongVegetationReportScopeService.GetLongVegetationReportScope(context)), 'scope'),
      catalogue: async () => nonnull(await scopeReads.track(LongVegetationReportScopeService.GetLongVegetationProjectFieldDefinitions(context)), 'field catalogue'),
      switchScope: async request => nonnull(await LongVegetationReportScopeService.SetLongVegetationReportScope(context, JSON.stringify(request)), 'scope switch'),
      fields: async request => nonnull(await LongVegetationReportScopeService.SetLongVegetationProjectFields(context, JSON.stringify(request)), 'field draft'),
      build: async revision => nonnull(await LongVegetationReportScopeService.BuildLongVegetationProjectScope(context, revision), 'Build'),
    },
    options: request => reportReads.track(LongVegetationReportScopeService.GetLongVegetationProjectOptions(context, JSON.stringify(request))),
    preview: request => reportReads.track(LongVegetationReportScopeService.PreviewLongVegetationProject(context, JSON.stringify(request))),
    unitPreferences: request => unitReads.track(LongVegetationReportScopeService.GetLongVegetationProjectUnitPreferences(context, JSON.stringify(request))),
    saveUnits: request => LongVegetationReportScopeService.SaveLongVegetationProjectUnits(context, JSON.stringify(request)),
    workbookReview: request => reportReads.track(LongVegetationProjectWorkbookService.GetReview(context, JSON.stringify(request))),
    publishWorkbook: request => LongVegetationProjectWorkbookService.ExportReviewed(context, JSON.stringify(request)),
    cancelScopeReads: () => scopeReads.cancelAll(),
    cancelUnitReads: () => unitReads.cancelAll(),
    cancelReportReads: () => reportReads.cancelAll(),
  };
}

export const projectReportWorkspace = capabilities.enabled ? createProjectReportWorkspaceRegistry(ownedPorts, capabilities) : null;
