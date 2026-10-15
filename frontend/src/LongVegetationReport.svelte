<script module lang="ts">
  import { createLongVegetationGroupPreferencesRegistry } from './longVegetationGroupPreferences';
  import { createLongVegetationNamingPreferencesRegistry } from './longVegetationNamingPreferences';
  import { createLongVegetationSelectedFieldPreferencesRegistry } from './longVegetationSelectedFieldPreferences';
  import { createLongVegetationEnforcementPreferencesRegistry } from './longVegetationEnforcementPreferences';
  import { createLongVegetationUnitPreferencesRegistry } from './longVegetationUnitPreferences';
  import { createSelectedUnitSelectionRegistry } from './longVegetationSelectedUnitSelection';
  const selectedUnitSession = import.meta.env.VITE_LONG_VEGETATION_SELECTED_UNIT_SELECTION === 'true'
    ? createSelectedUnitSelectionRegistry() : null;
  const groupingSession = import.meta.env.VITE_LONG_VEGETATION_GROUP_PREFERENCES === 'true'
    ? createLongVegetationGroupPreferencesRegistry({
      strata: import.meta.env.VITE_LONG_VEGETATION_STRATA === 'true',
      lifeform: import.meta.env.VITE_LONG_VEGETATION_LIFEFORM === 'true',
    }) : null;
  const namingSession = import.meta.env.VITE_LONG_VEGETATION_NAMING_PREFERENCES === 'true'
    ? createLongVegetationNamingPreferencesRegistry({ code: import.meta.env.VITE_LONG_VEGETATION_CODE === 'true' }) : null;
  const selectedFieldSession = import.meta.env.VITE_LONG_VEGETATION_SELECTED_FIELD_PREFERENCES === 'true'
    ? createLongVegetationSelectedFieldPreferencesRegistry() : null;
  const enforcementSession = import.meta.env.VITE_LONG_VEGETATION_ENFORCEMENT_PREFERENCES === 'true'
    ? createLongVegetationEnforcementPreferencesRegistry() : null;
  const unitSession = import.meta.env.VITE_LONG_VEGETATION_UNIT_PREFERENCES === 'true'
    ? createLongVegetationUnitPreferencesRegistry(import.meta.env.VITE_LONG_VEGETATION_NONE_UNITS === 'true',
      import.meta.env.VITE_LONG_VEGETATION_SELECTED_UNITS === 'true') : null;
</script>

<script lang="ts">
  import { onMount, onDestroy, untrack } from 'svelte';
  import { ContextService, LongVegetationPreferencesService, LongVegetationStatisticsPreferencesService, VegetationWorkbookService, type LongVegetationSettings, type ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { reportCellText } from './longEnvironmentReport';
  import { requireLongVegetationGrouping, validateLongVegetationOptions, validateLongVegetationPreview, type ValidatedLongVegetationPreview } from './longVegetationReport';
  import { vegetationWorkbookPublicationSession, VegetationWorkbookPublicationSession, type ValidatedVegetationWorkbookReview } from './vegetationWorkbook';
  import { longVegetationPreferencesSession } from './longVegetationPreferences';
  import { longVegetationStatisticsPreferencesSession } from './longVegetationStatisticsPreferences';
  import { longVegetationSwitchPreferencesSession } from './longVegetationSwitchPreferences';
  import LongVegetationSwitchPreferences from './LongVegetationSwitchPreferences.svelte';
  import { longVegetationThresholdPreferencesSession } from './longVegetationThresholdPreferences';
  import LongVegetationThresholdPreferences from './LongVegetationThresholdPreferences.svelte';
  import LongVegetationGroupPreferences from './LongVegetationGroupPreferences.svelte';
  import { longVegetationQualityNullPreferencesSession } from './longVegetationQualityNullPreferences';
  import LongVegetationQualityNullPreferences from './LongVegetationQualityNullPreferences.svelte';
  import { longVegetationQualityMinimumPreferencesSession } from './longVegetationQualityMinimumPreferences';
  import LongVegetationQualityMinimumPreferences from './LongVegetationQualityMinimumPreferences.svelte';
  import LongVegetationNamingPreferences from './LongVegetationNamingPreferences.svelte';
  import LongVegetationSelectedFieldPreferences from './LongVegetationSelectedFieldPreferences.svelte';
  import LongVegetationEnforcementPreferences from './LongVegetationEnforcementPreferences.svelte';
  import LongVegetationUnitPreferences from './LongVegetationUnitPreferences.svelte';
  import LongVegetationSelectedUnitSelection from './LongVegetationSelectedUnitSelection.svelte';
  import LongVegetationProjectScopeSelection from './LongVegetationProjectScopeSelection.svelte';
  import LongVegetationProjectUnitPreferences from './LongVegetationProjectUnitPreferences.svelte';
  import { projectReportWorkspace } from './longVegetationProjectWorkspaces';

  let { contextId, project, projectPath, su, suPath, operationHeld = false, onBusyChange }: {
    contextId: string; project: string; projectPath: string; su: string; suPath: string;
    operationHeld?: boolean;
    onBusyChange: (busy: boolean) => void;
  } = $props();
  let settings = $state.raw<LongVegetationSettings | null>(null);
  let busy = $state(false);
  let error = $state('');
  let preview = $state.raw<ValidatedLongVegetationPreview | null>(null);
  let workbookReview = $state.raw<ValidatedVegetationWorkbookReview | null>(null);
  let destination = $state('');
  const reads = new ReadRequests();
  let generation = 0;
  let disposed = false;
  const lifeformEnabled = import.meta.env.VITE_LONG_VEGETATION_LIFEFORM === 'true';
  const strataEnabled = import.meta.env.VITE_LONG_VEGETATION_STRATA === 'true';
  const codeEnabled = import.meta.env.VITE_LONG_VEGETATION_CODE === 'true';
  const selectedFieldEnabled = import.meta.env.VITE_LONG_VEGETATION_SELECTED_FIELD === 'true';
  const noneUnitsEnabled = import.meta.env.VITE_LONG_VEGETATION_NONE_UNITS === 'true';
  const selectedUnitsEnabled = import.meta.env.VITE_LONG_VEGETATION_SELECTED_UNITS === 'true';
  const workbookEnabled = import.meta.env.VITE_LONG_VEGETATION_WORKBOOK === 'true';
  const preferencesEnabled = import.meta.env.VITE_LONG_VEGETATION_PREFERENCES === 'true';
  const statisticsEnabled = import.meta.env.VITE_LONG_VEGETATION_STATISTICS_PREFERENCES === 'true';
  const switchesEnabled = import.meta.env.VITE_LONG_VEGETATION_SWITCH_PREFERENCES === 'true';
  const thresholdsEnabled = import.meta.env.VITE_LONG_VEGETATION_THRESHOLD_PREFERENCES === 'true';
  const qualityNullsEnabled = import.meta.env.VITE_LONG_VEGETATION_QUALITY_NULL_PREFERENCES === 'true';
  const qualityMinimaEnabled = import.meta.env.VITE_LONG_VEGETATION_QUALITY_MINIMUM_PREFERENCES === 'true';
  const workbookOwner = untrack(() => ({ contextId, project, projectPath, su, suPath }));
  const projectWorkspace = untrack(() => projectReportWorkspace?.(workbookOwner) ?? null);
  let projectView = $state.raw(untrack(() => projectWorkspace?.view() ?? null));
  const projectActive = $derived(!!projectView?.projectActive);
  const projectScopeKnown = $derived(!projectWorkspace || !!projectView?.scope.scope && !projectView.scope.authorityUnknown);
  const projectBarrier = $derived(!!projectWorkspace && (!projectScopeKnown || !!projectView &&
    (projectView.readBusy || projectView.scope.busy ||
      projectView.scope.scope?.sourceKind === 'project-selected-fields' && projectView.scope.dirty ||
      projectView.units.closeBlocked)));
  const projectMutationHeld = $derived(!!projectView && (projectView.readBusy || projectView.scope.busy ||
    projectView.units.readBusy || projectView.units.busy || projectView.units.blocked));
  const preferences = preferencesEnabled ? longVegetationPreferencesSession(workbookOwner) : null;
  let preferenceView = $state(preferences?.view() ?? null);
  const statistics = statisticsEnabled ? longVegetationStatisticsPreferencesSession(workbookOwner) : null;
  let statisticsView = $state(statistics?.view() ?? null);
  const titleBarrier = $derived(!!preferenceView && (preferenceView.busy || preferenceView.blocked ||
    preferenceView.authorityUnknown || !!preferenceView.draftError));
  const statisticsBarrier = $derived(!!statisticsView && (statisticsView.busy || statisticsView.blocked ||
    statisticsView.authorityUnknown || !!statisticsView.draftError));
  const switches = switchesEnabled ? longVegetationSwitchPreferencesSession(workbookOwner) : null;
  let switchView = $state(switches?.view() ?? null);
  let switchReadBusy = $state(false);
  const switchBarrier = $derived(switchReadBusy || !!switchView && (switchView.busy || switchView.blocked ||
    switchView.authorityUnknown || !!switchView.draftError));
  const switchMutationHeld = $derived(switchReadBusy || !!switchView && (switchView.busy || switchView.blocked));
  const thresholds = thresholdsEnabled ? longVegetationThresholdPreferencesSession(workbookOwner) : null;
  let thresholdView = $state(thresholds?.view() ?? null);
  let thresholdReadBusy = $state(false);
  const thresholdBarrier = $derived(thresholdReadBusy || !!thresholdView && (thresholdView.busy || thresholdView.blocked ||
    thresholdView.authorityUnknown || !!thresholdView.draftError));
  const thresholdMutationHeld = $derived(thresholdReadBusy || !!thresholdView && (thresholdView.busy || thresholdView.blocked));
  const groups = groupingSession ? groupingSession(workbookOwner) : null;
  let groupView = $state(groups?.view() ?? null);
  let groupReadBusy = $state(false);
  const groupBarrier = $derived(groupReadBusy || !!groupView && (groupView.busy || groupView.blocked ||
    groupView.authorityUnknown || !!groupView.draftError));
  const groupMutationHeld = $derived(groupReadBusy || !!groupView && (groupView.busy || groupView.blocked));
  const qualityNulls = qualityNullsEnabled ? longVegetationQualityNullPreferencesSession(workbookOwner) : null;
  let qualityNullView = $state(qualityNulls?.view() ?? null);
  let qualityNullReadBusy = $state(false);
  const qualityNullBarrier = $derived(qualityNullReadBusy || !!qualityNullView && (qualityNullView.busy || qualityNullView.blocked ||
    qualityNullView.authorityUnknown || !!qualityNullView.draftError));
  const qualityNullMutationHeld = $derived(qualityNullReadBusy || !!qualityNullView && (qualityNullView.busy || qualityNullView.blocked));
  const qualityMinima = qualityMinimaEnabled ? longVegetationQualityMinimumPreferencesSession(workbookOwner) : null;
  let qualityMinimumView = $state(qualityMinima?.view() ?? null);
  let qualityMinimumReadBusy = $state(false);
  const qualityMinimumBarrier = $derived(qualityMinimumReadBusy || !!qualityMinimumView && (qualityMinimumView.busy || qualityMinimumView.blocked ||
    qualityMinimumView.authorityUnknown || !!qualityMinimumView.draftError));
  const qualityMinimumMutationHeld = $derived(qualityMinimumReadBusy || !!qualityMinimumView && (qualityMinimumView.busy || qualityMinimumView.blocked));
  const naming = namingSession ? namingSession(workbookOwner) : null;
  let namingView = $state(naming?.view() ?? null);
  let namingReadBusy = $state(false);
  const namingBarrier = $derived(namingReadBusy || !!namingView && (namingView.busy || namingView.blocked ||
    namingView.authorityUnknown || !!namingView.draftError));
  const namingMutationHeld = $derived(namingReadBusy || !!namingView && (namingView.busy || namingView.blocked));
  const selectedField = selectedFieldSession ? selectedFieldSession(workbookOwner) : null;
  let selectedFieldView = $state(selectedField?.view() ?? null);
  let selectedFieldReadBusy = $state(false);
  const selectedFieldBarrier = $derived(selectedFieldReadBusy || !!selectedFieldView && (selectedFieldView.busy || selectedFieldView.blocked ||
    selectedFieldView.authorityUnknown || !!selectedFieldView.draftError));
  const selectedFieldMutationHeld = $derived(selectedFieldReadBusy || !!selectedFieldView && (selectedFieldView.busy || selectedFieldView.blocked));
  const enforcement = enforcementSession ? enforcementSession(workbookOwner) : null;
  let enforcementView = $state(enforcement?.view() ?? null);
  let enforcementReadBusy = $state(false);
  const enforcementBarrier = $derived(enforcementReadBusy || !!enforcementView && (enforcementView.busy || enforcementView.blocked ||
    enforcementView.authorityUnknown || !!enforcementView.draftError));
  const enforcementMutationHeld = $derived(enforcementReadBusy || !!enforcementView && (enforcementView.busy || enforcementView.blocked));
  const units = unitSession ? unitSession(workbookOwner) : null;
  let unitView = $state(units?.view() ?? null);
  let unitReadBusy = $state(false);
  const selectedUnitSelection = selectedUnitSession ? selectedUnitSession(workbookOwner) : null;
  let selectedUnitView = $state.raw(selectedUnitSelection?.view() ?? null);
  const selectedUnitBarrier = $derived(!!selectedUnitView && (selectedUnitView.busy ||
    selectedUnitView.dirty || selectedUnitView.authorityUnknown));
  const selectedUnitMutationHeld = $derived(!!selectedUnitView?.busy);
  const ordinaryUnitBarrier = $derived(selectedUnitBarrier || unitReadBusy || !!unitView && (unitView.busy || unitView.blocked ||
    unitView.authorityUnknown || !!unitView.draftError));
  const unitBarrier = $derived(ordinaryUnitBarrier || projectBarrier);
  const ordinaryUnitMutationHeld = $derived(selectedUnitMutationHeld || unitReadBusy || !!unitView && (unitView.busy || unitView.blocked));
  const unitMutationHeld = $derived(ordinaryUnitMutationHeld || projectMutationHeld);
  const sharedPreferenceBarrier = $derived(titleBarrier || statisticsBarrier || switchBarrier || thresholdBarrier || groupBarrier || qualityNullBarrier || qualityMinimumBarrier || namingBarrier || selectedFieldBarrier || enforcementBarrier || ordinaryUnitBarrier);
  const sharedMutationHeld = $derived(ordinaryUnitMutationHeld || !!preferenceView?.busy || !!preferenceView?.blocked ||
    !!statisticsView?.busy || !!statisticsView?.blocked || switchMutationHeld || thresholdMutationHeld ||
    groupMutationHeld || qualityNullMutationHeld || qualityMinimumMutationHeld || namingMutationHeld ||
    selectedFieldMutationHeld || enforcementMutationHeld);
  const preferenceBarrier = $derived(sharedPreferenceBarrier || projectBarrier);
  const workbookSession = workbookEnabled ? vegetationWorkbookPublicationSession(workbookOwner, selectedUnitsEnabled) :
    new VegetationWorkbookPublicationSession(workbookOwner, selectedUnitsEnabled);
  let publication = $state(workbookSession.view());
  const workbookBarrier = $derived(publication.busy || publication.blocked ||
    !!projectView?.publication && (projectView.publication.busy || projectView.publication.blocked));
  const activeWorkbookReview = $derived(projectActive ? projectView?.workbookReview ?? null : workbookReview);
  const groupLabel = $derived(settings?.grouping === 'lifeform' ? 'Lifeform' : settings?.grouping === 'strata' ? 'Strata' : 'Layer');
  function reportBusy() { onBusyChange(busy || workbookBarrier || sharedPreferenceBarrier || !!projectView?.closeBlocked); }
  function invalidateReports() {
    preview = null; workbookReview = null;
    projectWorkspace?.invalidate();
  }
  const unsubscribeWorkbook = workbookSession.subscribe(() => {
    publication = workbookSession.view();
    if (publication.blocked) workbookReview = null;
    reportBusy();
  });
  const unsubscribePreferences = preferences?.subscribe(() => {
    preferenceView = preferences.view(); invalidateReports();
    if (preferenceView.authorityUnknown || settings && preferenceView.saved && settings.title !== preferenceView.saved.title) settings = null;
    reportBusy();
  });
  const unsubscribeStatistics = statistics?.subscribe(() => {
    statisticsView = statistics.view(); invalidateReports();
    if (statisticsView.authorityUnknown || settings && statisticsView.saved &&
        (settings.average !== (statisticsView.saved.avgType === 10 ? 'all-plots' : 'observations') ||
         settings.order !== (statisticsView.saved.orderBy === 10 ? 'species' : 'presence'))) settings = null;
    reportBusy();
  });
  const unsubscribeSwitches = switches?.subscribe(() => {
    switchView = switches.view(); invalidateReports();
    if (switchView.authorityUnknown || settings && switchView.saved &&
        settings.constantSpeciesList !== switchView.saved.constantSpeciesList) settings = null;
    reportBusy();
  });
  const unsubscribeThresholds = thresholds?.subscribe(() => {
    thresholdView = thresholds.view(); invalidateReports();
    if (thresholdView.authorityUnknown || settings && thresholdView.saved &&
        (settings.presenceGreaterThan !== thresholdView.saved.presenceGreaterThan ||
         settings.meanCoverGreaterThan !== thresholdView.saved.meanCoverGreaterThan)) settings = null;
    reportBusy();
  });
  const unsubscribeGroups = groups?.subscribe(() => {
    groupView = groups.view(); invalidateReports();
    if (groupView.authorityUnknown || settings && groupView.saved &&
        settings.grouping !== (groupView.saved.groupBy === 1 ? 'layer' : groupView.saved.groupBy === 2 ? 'strata' :
          groupView.saved.groupBy === 3 ? 'lifeform' : 'none')) settings = null;
    reportBusy();
  });
  const unsubscribeQualityNulls = qualityNulls?.subscribe(() => {
    qualityNullView = qualityNulls.view(); invalidateReports();
    if (qualityNullView.authorityUnknown || settings?.quality && qualityNullView.saved &&
        (settings.quality.site.includeNull !== qualityNullView.saved.siteIncludeNulls ||
         settings.quality.veg.includeNull !== qualityNullView.saved.vegIncludeNulls ||
         settings.quality.soil.includeNull !== qualityNullView.saved.soilIncludeNulls)) settings = null;
    reportBusy();
  });
  const unsubscribeQualityMinima = qualityMinima?.subscribe(() => {
    qualityMinimumView = qualityMinima.view(); invalidateReports();
    if (qualityMinimumView.authorityUnknown || settings?.quality && qualityMinimumView.saved &&
        (settings.quality.site.minimum !== qualityMinimumView.saved.siteMinimum ||
         settings.quality.veg.minimum !== qualityMinimumView.saved.vegMinimum ||
         settings.quality.soil.minimum !== qualityMinimumView.saved.soilMinimum)) settings = null;
    reportBusy();
  });
  const unsubscribeNaming = naming?.subscribe(() => {
    namingView = naming.view(); invalidateReports();
    if (namingView.authorityUnknown || settings && namingView.saved &&
        (settings.showEnglishName !== (namingView.saved.mode === 1) ||
         settings.showSpeciesCode !== (namingView.saved.mode === 2) ||
         settings.selectedField !== (namingView.saved.mode === 4 ? namingView.saved.selectedField : undefined))) settings = null;
    reportBusy();
  });
  const unsubscribeSelectedField = selectedField?.subscribe(() => {
    selectedFieldView = selectedField.view(); invalidateReports();
    if (selectedFieldView.authorityUnknown || settings && selectedFieldView.saved &&
        (settings.showEnglishName !== (selectedFieldView.saved.mode === 1) ||
         settings.showSpeciesCode !== (selectedFieldView.saved.mode === 2) ||
         settings.selectedField !== (selectedFieldView.saved.mode === 4 ? selectedFieldView.saved.selectedField : undefined))) settings = null;
    reportBusy();
  });
  const unsubscribeEnforcement = enforcement?.subscribe(() => {
    enforcementView = enforcement.view(); invalidateReports();
    if (enforcementView.authorityUnknown || settings && enforcementView.saved &&
        (!!settings.quality !== enforcementView.saved.enforce ||
         (settings.unitGrouping === 'all-plots' ? 3 : settings.unitGrouping === 'selected-fields' ? 2 : 1) !== enforcementView.saved.unitGroups)) settings = null;
    reportBusy();
  });
  const unsubscribeUnits = units?.subscribe(() => {
    unitView = units.view(); invalidateReports();
    if (unitView.authorityUnknown || settings && unitView.saved &&
        (settings.unitGrouping === 'all-plots' ? 3 : settings.unitGrouping === 'selected-fields' ? 2 : 1) !== unitView.saved.unitGroups) settings = null;
    reportBusy();
  });
  function projectChanged() {
    generation++; reads.cancelAll(); busy = false;
    projectView = projectWorkspace?.view() ?? null;
    workbookReview = null;
    settings = projectView?.projectActive ? projectView.settings : null;
    preview = projectView?.projectActive ? projectView.preview : null;
    reportBusy();
  }
  const unsubscribeProject = projectWorkspace?.subscribe(projectChanged);

  function selectedUnitChanged() {
    generation++; reads.cancelAll(); busy = false;
    selectedUnitView = selectedUnitSelection?.view() ?? null;
    preview = null; workbookReview = null; settings = null;
    reportBusy();
  }

  function cancel() {
    generation++; reads.cancelAll(); projectWorkspace?.cancelReportRead(); preferences?.cancelLoad(); statistics?.cancelLoad(); busy = false; reportBusy();
  }
  async function readProject(operation: () => Promise<void>) {
    if (busy || operationHeld || workbookBarrier || sharedPreferenceBarrier || !projectView?.outputReady) return;
    error = '';
    try { await operation(); }
    catch (cause) { error = `Original project report unavailable: ${String(cause)}`; }
  }
  async function options() {
    if (projectActive && projectWorkspace) { await readProject(() => projectWorkspace.loadOptions()); return; }
    if (busy || operationHeld || workbookBarrier || preferenceBarrier) return;
    cancel(); const request = generation; settings = null; error = ''; preview = null; workbookReview = null;
    busy = true; reportBusy();
    try {
      const value = await reads.track(ContextService.GetLongVegetationOptions(contextId));
      if (request !== generation) return;
      const candidate = validateLongVegetationOptions(value, contextId, project, projectPath, su, suPath, selectedUnitsEnabled).settings;
      requireLongVegetationGrouping(candidate, lifeformEnabled, strataEnabled, codeEnabled, selectedFieldEnabled, noneUnitsEnabled, selectedUnitsEnabled);
      settings = candidate;
    } catch (cause) {
      if (request === generation) error = `Report options unavailable: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; reportBusy(); }
    }
  }
  async function show() {
    if (projectActive && projectWorkspace) { await readProject(() => projectWorkspace.showPreview()); return; }
    if (busy || operationHeld || workbookBarrier || preferenceBarrier || !settings || su === 'None') return;
    cancel(); const request = generation; error = ''; preview = null; workbookReview = null;
    busy = true; reportBusy();
    try {
      const value = await reads.track(ContextService.PreviewLongVegetation(contextId));
      if (request !== generation) return;
      const candidate = validateLongVegetationPreview(value, contextId, project, projectPath, su, suPath, selectedUnitsEnabled);
      requireLongVegetationGrouping(candidate.settings, lifeformEnabled, strataEnabled, codeEnabled, selectedFieldEnabled, noneUnitsEnabled, selectedUnitsEnabled);
      preview = candidate;
      settings = candidate.settings;
    } catch (cause) {
      if (request === generation) error = `Report unavailable; no data or configuration written: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; reportBusy(); }
    }
  }
  async function reviewWorkbook() {
    if (projectActive && projectWorkspace) {
      if (workbookEnabled) await readProject(() => projectWorkspace.reviewWorkbook());
      return;
    }
    if (!workbookEnabled || busy || operationHeld || workbookBarrier || preferenceBarrier || !settings || su === 'None' || su === 'USysSuTableDynamic') return;
    cancel(); const request = generation;
    error = ''; preview = null; workbookReview = null; busy = true; reportBusy();
    try {
      const value = await reads.track(VegetationWorkbookService.GetReview(contextId, JSON.stringify({ scope: 'unlumped' })));
      if (request !== generation) return;
      if (!value) throw new Error('No owned workbook review returned.');
      const reviewed = workbookSession.prepare(value);
      requireLongVegetationGrouping(reviewed.preview.settings, lifeformEnabled, strataEnabled, codeEnabled, selectedFieldEnabled, noneUnitsEnabled, selectedUnitsEnabled);
      workbookReview = reviewed;
      preview = reviewed.preview;
      settings = reviewed.preview.settings;
    } catch (cause) {
      if (request === generation) error = `Workbook review unavailable; no file published: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; reportBusy(); }
    }
  }
  async function publishWorkbook() {
    if (projectActive && projectWorkspace) {
      if (!workbookEnabled || busy || operationHeld || workbookBarrier || sharedPreferenceBarrier || !projectView?.workbookReview) return;
      error = '';
      try { await projectWorkspace.publishWorkbook(destination); }
      catch (cause) { error = String(cause); }
      return;
    }
    if (!workbookEnabled || busy || operationHeld || workbookBarrier || preferenceBarrier || !workbookReview) return;
    cancel(); error = '';
    try {
      await workbookSession.publish(workbookReview, destination, request =>
        VegetationWorkbookService.ExportReviewed(contextId, JSON.stringify(request)));
    } catch (cause) { error = String(cause); }
  }
  function acknowledgeWorkbook() {
    workbookSession.acknowledge();
    workbookReview = null;
  }
  async function loadSavedTitle() {
    if (!preferences || unitBarrier || busy || operationHeld || workbookBarrier || statisticsBarrier || switchBarrier || thresholdBarrier || groupBarrier || qualityNullBarrier || qualityMinimumBarrier || namingBarrier || selectedFieldBarrier || enforcementBarrier || preferenceView?.busy || preferenceView?.blocked) return;
    cancel(); const request = generation, token = preferences.beginLoad();
    error = ''; busy = true; reportBusy();
    try {
      const value = await reads.track(LongVegetationPreferencesService.GetLongVegetationPreferences(contextId));
      if (request !== generation) return;
      preferences.applyLoaded(value, token);
    } catch (cause) {
      if (request === generation) error = `Saved title unavailable; no defaults applied: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; reportBusy(); }
    }
  }
  async function saveTitle() {
    if (!preferences || busy || operationHeld || workbookBarrier || preferenceBarrier) return;
    cancel(); const request = generation; error = '';
    try {
      await preferences.save(value => LongVegetationPreferencesService.SaveLongVegetationPreferences(contextId, JSON.stringify(value)));
    } catch (cause) {
      if (request === generation) error = String(cause);
    }
  }
  function editTitle(value: string) {
    if (!preferences || unitMutationHeld) return;
    cancel(); error = ''; preferences.edit(value);
  }
  async function loadSavedStatistics() {
    if (!statistics || unitBarrier || busy || operationHeld || workbookBarrier || titleBarrier || switchBarrier || thresholdBarrier || groupBarrier || qualityNullBarrier || qualityMinimumBarrier || namingBarrier || selectedFieldBarrier || enforcementBarrier || statisticsView?.busy || statisticsView?.blocked) return;
    cancel(); const request = generation, token = statistics.beginLoad();
    error = ''; busy = true; reportBusy();
    try {
      const value = await reads.track(LongVegetationStatisticsPreferencesService.GetLongVegetationStatisticsPreferences(contextId));
      if (request !== generation) return;
      statistics.applyLoaded(value, token);
    } catch (cause) {
      if (request === generation) error = `Saved Average/Order choices unavailable; no defaults applied: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; reportBusy(); }
    }
  }
  async function saveStatistics() {
    if (!statistics || busy || operationHeld || workbookBarrier || preferenceBarrier) return;
    cancel(); const request = generation; error = '';
    try {
      await statistics.save(value => LongVegetationStatisticsPreferencesService.SaveLongVegetationStatisticsPreferences(contextId, JSON.stringify(value)));
    } catch (cause) {
      if (request === generation) error = String(cause);
    }
  }
  function editAverage(value: string) {
    if (!statistics || unitMutationHeld) return;
    cancel(); error = ''; statistics.editAverage(value);
  }
  function editOrder(value: string) {
    if (!statistics || unitMutationHeld) return;
    cancel(); error = ''; statistics.editOrder(value);
  }
  const numberText = (value: number | null) => value === null ? 'NULL' : String(value);
  const presenceText = (value: number | null) => value === null ? 'NULL' : `${value * 100}%`;
  const supplementaryText = (cell: ProjectMetadataCell) => preview?.report.selectedFieldType === 'boolean' && cell.storage !== 'null'
    ? cell.integer === '-1' ? 'TRUE' : 'FALSE' : reportCellText(cell);
  onMount(() => {
    if (projectWorkspace) void projectWorkspace.selection.load().then(() => {
      if (disposed) return;
      if (projectWorkspace.view().scope.scope?.sourceKind === 'selected-list') void options();
    }).catch(cause => { error = `Owned report scope unavailable: ${String(cause)}`; });
    else void options();
  });
  onDestroy(() => {
    disposed = true;
    unsubscribeWorkbook(); unsubscribePreferences?.(); unsubscribeStatistics?.(); unsubscribeSwitches?.();
    unsubscribeThresholds?.(); unsubscribeGroups?.(); unsubscribeQualityNulls?.(); unsubscribeQualityMinima?.();
    unsubscribeNaming?.(); unsubscribeSelectedField?.(); unsubscribeEnforcement?.(); unsubscribeUnits?.(); unsubscribeProject?.();
    projectWorkspace?.cancelScopeReads(); projectWorkspace?.selection.cancel();
    projectWorkspace?.cancelUnitReads(); projectWorkspace?.units.cancelRead(); cancel();
  });
</script>

<section class="report" data-long-vegetation-report aria-label="Long Vegetation report">
  <h1>Long Vegetation</h1>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if preferenceView?.draftError}<p class="error" role="alert">{preferenceView.draftError}</p>{/if}
  {#if preferenceView?.error}<p class="error" role="alert">{preferenceView.error}</p>{/if}
  {#if statisticsView?.draftError}<p class="error" role="alert">{statisticsView.draftError}</p>{/if}
  {#if statisticsView?.error}<p class="error" role="alert">{statisticsView.error}</p>{/if}
  {#if publication.error}<p class="error" role="alert">{publication.error}</p>{/if}
  {#if publication.busy}<p role="status">Publishing the reviewed unlumped workbook; publication cannot be cancelled.</p>{/if}
  {#if publication.outcome}
    <p data-vegetation-workbook-receipt role="status">Workbook receipt: {publication.outcome.status}.
      Requested {publication.requestedDestination}; observed {publication.outcome.path || '(no committed file)'}.
      SHA256 {publication.outcome.sha256 || '(none)'}.</p>
  {:else if publication.requestedDestination && !publication.busy}
    <p data-vegetation-workbook-unknown role="alert">Workbook outcome unknown. Retain and inspect {publication.requestedDestination}; never replay blindly.</p>
  {/if}
  {#if publication.blocked && !publication.busy}
    <button onclick={acknowledgeWorkbook}>Acknowledge workbook outcome</button>
  {/if}
  {#if projectWorkspace && projectView}
    {#if projectView.error}<p role="alert" class="error">{projectView.error}</p>{/if}
    {#if projectView.publication?.error}<p role="alert" class="error">{projectView.publication.error}</p>{/if}
    {#if projectView.publication?.busy}<p role="status">Publishing the original project workbook; publication cannot be cancelled or replayed.</p>{/if}
    {#if projectView.publication?.outcome}
      <p data-project-workbook-receipt role="status">Original project workbook receipt: {projectView.publication.outcome.status}.
        Requested {projectView.publication.requestedDestination}; observed {projectView.publication.outcome.path || '(no committed file)'}.
        SHA256 {projectView.publication.outcome.sha256 || '(none)'}.</p>
    {:else if projectView.publication?.requestedDestination && !projectView.publication.busy}
      <p data-project-workbook-unknown role="alert">Original project workbook outcome unknown.
        Retain and inspect {projectView.publication.requestedDestination}; never replay blindly.</p>
    {/if}
    {#if projectView.publication?.blocked && !projectView.publication.busy}
      <button data-project-workbook-acknowledge disabled={busy || operationHeld || sharedMutationHeld || publication.busy || publication.blocked}
        onclick={() => projectWorkspace.workbook?.acknowledge()}>Acknowledge original project workbook outcome</button>
    {/if}
    <LongVegetationProjectScopeSelection selection={projectWorkspace.selection} allPlotsAvailable={su === 'None'}
      cancelReads={() => projectWorkspace.cancelScopeReads()} onChange={projectChanged}
      operationHeld={busy || operationHeld || workbookBarrier || sharedPreferenceBarrier || projectView.scopeOperationHeld}
      mutationHeld={busy || operationHeld || workbookBarrier || sharedMutationHeld || projectView.scopeMutationHeld} />
    {#if projectActive}
      <LongVegetationProjectUnitPreferences preferences={projectWorkspace.units}
        fetchPreferences={projectWorkspace.fetchUnitPreferences} savePreferences={projectWorkspace.saveUnitPreferences}
        cancelReads={() => projectWorkspace.cancelUnitReads()} onChange={projectChanged}
        operationHeld={busy || operationHeld || workbookBarrier || sharedPreferenceBarrier || projectView.unitOperationHeld}
        mutationHeld={busy || operationHeld || workbookBarrier || sharedMutationHeld || projectView.unitMutationHeld} />
    {/if}
  {/if}
  {#if preferences && preferenceView}
    <section class="preferences" aria-label="Saved Long Vegetation title">
      <label for="long-vegetation-title">Report Title</label>
      <textarea id="long-vegetation-title" data-source-control="rptLvTitle" rows="2"
        value={preferenceView.draft.title} aria-invalid={!!preferenceView.draftError}
        disabled={unitMutationHeld || busy || operationHeld || workbookBarrier || switchMutationHeld || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld || qualityMinimumMutationHeld || namingMutationHeld || selectedFieldMutationHeld || enforcementMutationHeld || statisticsView?.busy || statisticsView?.blocked || preferenceView.busy || preferenceView.blocked || !preferenceView.initialized}
        oninput={event => { const value = event.currentTarget.value; editTitle(value); }}></textarea>
      <div class="actions">
        <button data-long-vegetation-preferences-load
          disabled={unitBarrier || busy || operationHeld || workbookBarrier || statisticsBarrier || switchBarrier || thresholdBarrier || groupBarrier || qualityNullBarrier || qualityMinimumBarrier || namingBarrier || selectedFieldBarrier || enforcementBarrier || preferenceView.busy || preferenceView.blocked}
          onclick={loadSavedTitle}>Load saved title</button>
        <button data-long-vegetation-preferences-save
          disabled={busy || operationHeld || workbookBarrier || preferenceBarrier || !preferenceView.saved}
          onclick={saveTitle}>Save title preferences</button>
        <button data-long-vegetation-preferences-undo
          disabled={unitMutationHeld || busy || operationHeld || workbookBarrier || switchMutationHeld || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld || qualityMinimumMutationHeld || namingMutationHeld || selectedFieldMutationHeld || enforcementMutationHeld || statisticsView?.busy || statisticsView?.blocked || preferenceView.busy || preferenceView.blocked || !preferenceView.saved}
          onclick={() => { cancel(); preferences.undo(); }}>Undo title draft</button>
        {#if preferenceView.blocked && !preferenceView.busy}
          <button data-long-vegetation-preferences-acknowledge disabled={unitMutationHeld || busy || operationHeld || workbookBarrier || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld || qualityMinimumMutationHeld || namingMutationHeld || selectedFieldMutationHeld || enforcementMutationHeld}
            onclick={() => preferences.acknowledge()}>Acknowledge title preference outcome</button>
        {/if}
      </div>
      {#if preferenceView.busy}<p role="status">Saving title preferences; Save cannot be cancelled.</p>{/if}
      {#each preferenceView.receipts as receipt}
        {#if receipt.kind === 'known'}
          <p data-long-vegetation-preferences-receipt role="status">Preference receipt: changed {String(receipt.outcome.changed)};
            committed {String(receipt.outcome.committed)}. {receipt.outcome.errorMessage}</p>
        {:else}
          <p data-long-vegetation-preferences-receipt role="alert">Preference receipt: outcome unknown; changed unknown; committed unknown.
            {receipt.errorMessage}</p>
        {/if}
      {/each}
      <p class="guidance">Drafts do not change report output. Save explicitly, acknowledge the outcome, then reload report options.
        Acknowledgement retains receipts; an unknown outcome requires an explicit owned Load, never blind Save replay.
        No registry, project data or source audits are written.</p>
    </section>
  {/if}
  {#if statistics && statisticsView}
    <section class="preferences" aria-label="Saved Long Vegetation Average and Order">
      <label for="long-vegetation-average">Calculate</label>
      <select id="long-vegetation-average" data-source-control="rptLvAvgType" value={statisticsView.draft.avgType}
        aria-invalid={!!statisticsView.draftError}
        disabled={unitMutationHeld || busy || operationHeld || workbookBarrier || switchMutationHeld || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld || qualityMinimumMutationHeld || namingMutationHeld || selectedFieldMutationHeld || enforcementMutationHeld || preferenceView?.busy || preferenceView?.blocked || statisticsView.busy || statisticsView.blocked || !statisticsView.initialized}
        onchange={event => { const value = event.currentTarget.value; editAverage(value); }}>
        <option value="" disabled>Load saved Average</option>
        <option value="10">By n Plots</option><option value="20">Characteristic</option>
      </select>
      <label for="long-vegetation-order">Order by</label>
      <select id="long-vegetation-order" data-source-control="rptLvOrderBy" value={statisticsView.draft.orderBy}
        aria-invalid={!!statisticsView.draftError}
        disabled={unitMutationHeld || busy || operationHeld || workbookBarrier || switchMutationHeld || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld || qualityMinimumMutationHeld || namingMutationHeld || selectedFieldMutationHeld || enforcementMutationHeld || preferenceView?.busy || preferenceView?.blocked || statisticsView.busy || statisticsView.blocked || !statisticsView.initialized}
        onchange={event => { const value = event.currentTarget.value; editOrder(value); }}>
        <option value="" disabled>Load saved Order by</option>
        <option value="10">Species</option><option value="20">Presence</option>
      </select>
      <div class="actions">
        <button data-long-vegetation-statistics-load
          disabled={unitBarrier || busy || operationHeld || workbookBarrier || titleBarrier || switchBarrier || thresholdBarrier || groupBarrier || qualityNullBarrier || qualityMinimumBarrier || namingBarrier || selectedFieldBarrier || enforcementBarrier || statisticsView.busy || statisticsView.blocked}
          onclick={loadSavedStatistics}>Load saved Average/Order</button>
        <button data-long-vegetation-statistics-save
          disabled={busy || operationHeld || workbookBarrier || preferenceBarrier || !statisticsView.saved}
          onclick={saveStatistics}>Save Average/Order preferences</button>
        <button data-long-vegetation-statistics-undo
          disabled={unitMutationHeld || busy || operationHeld || workbookBarrier || switchMutationHeld || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld || qualityMinimumMutationHeld || namingMutationHeld || selectedFieldMutationHeld || enforcementMutationHeld || preferenceView?.busy || preferenceView?.blocked || statisticsView.busy || statisticsView.blocked || !statisticsView.saved}
          onclick={() => { cancel(); statistics.undo(); }}>Undo Average/Order draft</button>
        {#if statisticsView.blocked && !statisticsView.busy}
          <button data-long-vegetation-statistics-acknowledge disabled={unitMutationHeld || busy || operationHeld || workbookBarrier || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld || qualityMinimumMutationHeld || namingMutationHeld || selectedFieldMutationHeld || enforcementMutationHeld}
            onclick={() => statistics.acknowledge()}>Acknowledge Average/Order outcome</button>
        {/if}
      </div>
      {#if statisticsView.busy}<p role="status">Saving Average/Order preferences; Save cannot be cancelled.</p>{/if}
      {#each statisticsView.receipts as receipt}
        <p data-long-vegetation-statistics-receipt role={receipt.kind === 'known' ? 'status' : 'alert'}>
          {#if receipt.kind === 'known'}Preference receipt: changed {String(receipt.outcome.changed)};
            committed {String(receipt.outcome.committed)}. {receipt.outcome.errorMessage}
          {:else}Preference receipt: outcome unknown; changed unknown; committed unknown. {receipt.errorMessage}{/if}
        </p>
      {/each}
      <p class="guidance">Drafts do not change report output. Save explicitly, acknowledge the outcome, then reload report options.
        Only the two saved source choices are changed; no registry, project data or source audits are written.</p>
    </section>
  {/if}
  {#if switches}
    <LongVegetationSwitchPreferences contextId={contextId} preferences={switches}
      operationHeld={unitBarrier || busy || operationHeld || workbookBarrier || titleBarrier || statisticsBarrier || thresholdBarrier || groupBarrier || qualityNullBarrier || qualityMinimumBarrier || namingBarrier || selectedFieldBarrier || enforcementBarrier}
      mutationHeld={unitMutationHeld || busy || operationHeld || workbookBarrier || !!preferenceView?.busy || !!preferenceView?.blocked ||
        !!statisticsView?.busy || !!statisticsView?.blocked || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld || qualityMinimumMutationHeld || namingMutationHeld || selectedFieldMutationHeld || enforcementMutationHeld}
      onReadBusyChange={value => { switchReadBusy = value; reportBusy(); }} />
  {/if}
  {#if thresholds}
    <LongVegetationThresholdPreferences contextId={contextId} preferences={thresholds}
      operationHeld={unitBarrier || busy || operationHeld || workbookBarrier || titleBarrier || statisticsBarrier || switchBarrier || groupBarrier || qualityNullBarrier || qualityMinimumBarrier || namingBarrier || selectedFieldBarrier || enforcementBarrier}
      mutationHeld={unitMutationHeld || busy || operationHeld || workbookBarrier || !!preferenceView?.busy || !!preferenceView?.blocked ||
        !!statisticsView?.busy || !!statisticsView?.blocked || switchMutationHeld || groupMutationHeld || qualityNullMutationHeld || qualityMinimumMutationHeld || namingMutationHeld || selectedFieldMutationHeld || enforcementMutationHeld}
      onReadBusyChange={value => { thresholdReadBusy = value; reportBusy(); }} />
  {/if}
  {#if groups}
    <LongVegetationGroupPreferences contextId={contextId} preferences={groups}
      operationHeld={unitBarrier || busy || operationHeld || workbookBarrier || titleBarrier || statisticsBarrier || switchBarrier || thresholdBarrier || qualityNullBarrier || qualityMinimumBarrier || namingBarrier || selectedFieldBarrier || enforcementBarrier}
      mutationHeld={unitMutationHeld || busy || operationHeld || workbookBarrier || !!preferenceView?.busy || !!preferenceView?.blocked ||
        !!statisticsView?.busy || !!statisticsView?.blocked || switchMutationHeld || thresholdMutationHeld || qualityNullMutationHeld || qualityMinimumMutationHeld || namingMutationHeld || selectedFieldMutationHeld || enforcementMutationHeld}
      onReadBusyChange={value => { groupReadBusy = value; reportBusy(); }} />
  {/if}
  {#if qualityNulls}
    <LongVegetationQualityNullPreferences contextId={contextId} preferences={qualityNulls}
      operationHeld={unitBarrier || busy || operationHeld || workbookBarrier || titleBarrier || statisticsBarrier || switchBarrier || thresholdBarrier || groupBarrier || qualityMinimumBarrier || namingBarrier || selectedFieldBarrier || enforcementBarrier}
      mutationHeld={unitMutationHeld || busy || operationHeld || workbookBarrier || !!preferenceView?.busy || !!preferenceView?.blocked ||
        !!statisticsView?.busy || !!statisticsView?.blocked || switchMutationHeld || thresholdMutationHeld || groupMutationHeld || qualityMinimumMutationHeld || namingMutationHeld || selectedFieldMutationHeld || enforcementMutationHeld}
      onReadBusyChange={value => { qualityNullReadBusy = value; reportBusy(); }} />
  {/if}
  {#if qualityMinima}
    <LongVegetationQualityMinimumPreferences contextId={contextId} preferences={qualityMinima}
      operationHeld={unitBarrier || busy || operationHeld || workbookBarrier || titleBarrier || statisticsBarrier || switchBarrier || thresholdBarrier || groupBarrier || qualityNullBarrier || namingBarrier || selectedFieldBarrier || enforcementBarrier}
      mutationHeld={unitMutationHeld || busy || operationHeld || workbookBarrier || !!preferenceView?.busy || !!preferenceView?.blocked ||
        !!statisticsView?.busy || !!statisticsView?.blocked || switchMutationHeld || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld || namingMutationHeld || selectedFieldMutationHeld || enforcementMutationHeld}
      onReadBusyChange={value => { qualityMinimumReadBusy = value; reportBusy(); }} />
  {/if}
  {#if naming}
    <LongVegetationNamingPreferences contextId={contextId} preferences={naming} selectedFieldEnabled={selectedFieldEnabled}
      operationHeld={unitBarrier || busy || operationHeld || workbookBarrier || titleBarrier || statisticsBarrier || switchBarrier || thresholdBarrier || groupBarrier || qualityNullBarrier || qualityMinimumBarrier || selectedFieldBarrier || enforcementBarrier}
      mutationHeld={unitMutationHeld || busy || operationHeld || workbookBarrier || !!preferenceView?.busy || !!preferenceView?.blocked ||
        !!statisticsView?.busy || !!statisticsView?.blocked || switchMutationHeld || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld || qualityMinimumMutationHeld || selectedFieldMutationHeld || enforcementMutationHeld}
      onReadBusyChange={value => { namingReadBusy = value; reportBusy(); }} />
  {/if}
  {#if selectedField}
    <LongVegetationSelectedFieldPreferences contextId={contextId} preferences={selectedField}
      operationHeld={unitBarrier || busy || operationHeld || workbookBarrier || titleBarrier || statisticsBarrier || switchBarrier || thresholdBarrier || groupBarrier || qualityNullBarrier || qualityMinimumBarrier || namingBarrier || enforcementBarrier}
      mutationHeld={unitMutationHeld || busy || operationHeld || workbookBarrier || !!preferenceView?.busy || !!preferenceView?.blocked ||
        !!statisticsView?.busy || !!statisticsView?.blocked || switchMutationHeld || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld || qualityMinimumMutationHeld || namingMutationHeld || enforcementMutationHeld}
      onReadBusyChange={value => { selectedFieldReadBusy = value; reportBusy(); }} />
  {/if}
  {#if enforcement}
    <LongVegetationEnforcementPreferences contextId={contextId} preferences={enforcement}
      operationHeld={unitBarrier || busy || operationHeld || workbookBarrier || titleBarrier || statisticsBarrier || switchBarrier || thresholdBarrier || groupBarrier || qualityNullBarrier || qualityMinimumBarrier || namingBarrier || selectedFieldBarrier}
      mutationHeld={unitMutationHeld || busy || operationHeld || workbookBarrier || !!preferenceView?.busy || !!preferenceView?.blocked ||
        !!statisticsView?.busy || !!statisticsView?.blocked || switchMutationHeld || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld || qualityMinimumMutationHeld || namingMutationHeld || selectedFieldMutationHeld}
      onReadBusyChange={value => { enforcementReadBusy = value; reportBusy(); }} />
  {/if}
  {#if units && (!projectActive && projectScopeKnown || unitReadBusy || unitView?.busy || unitView?.blocked || unitView?.authorityUnknown)}
    <LongVegetationUnitPreferences contextId={contextId} preferences={units}
      operationHeld={projectBarrier || selectedUnitBarrier || busy || operationHeld || workbookBarrier || titleBarrier || statisticsBarrier || switchBarrier || thresholdBarrier || groupBarrier || qualityNullBarrier || qualityMinimumBarrier || namingBarrier || selectedFieldBarrier || enforcementBarrier}
      mutationHeld={projectMutationHeld || selectedUnitMutationHeld || busy || operationHeld || workbookBarrier || !!preferenceView?.busy || !!preferenceView?.blocked ||
        !!statisticsView?.busy || !!statisticsView?.blocked || switchMutationHeld || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld || qualityMinimumMutationHeld || namingMutationHeld || selectedFieldMutationHeld || enforcementMutationHeld}
      onReadBusyChange={value => { unitReadBusy = value; reportBusy(); }} />
  {/if}
  {#if selectedUnitSelection && (!projectActive && projectScopeKnown || selectedUnitBarrier)}
    <LongVegetationSelectedUnitSelection contextId={contextId} selection={selectedUnitSelection}
      operationHeld={projectBarrier || unitReadBusy || !!unitView && (unitView.busy || unitView.blocked || !!unitView.draftError) ||
        busy || operationHeld || workbookBarrier || titleBarrier || statisticsBarrier || switchBarrier || thresholdBarrier ||
        groupBarrier || qualityNullBarrier || qualityMinimumBarrier || namingBarrier || selectedFieldBarrier || enforcementBarrier}
      mutationHeld={projectMutationHeld || unitReadBusy || !!unitView && (unitView.busy || unitView.blocked) ||
        busy || operationHeld || workbookBarrier || !!preferenceView?.busy || !!preferenceView?.blocked ||
        !!statisticsView?.busy || !!statisticsView?.blocked || switchMutationHeld || thresholdMutationHeld ||
        groupMutationHeld || qualityNullMutationHeld || qualityMinimumMutationHeld || namingMutationHeld ||
        selectedFieldMutationHeld || enforcementMutationHeld}
      onChange={selectedUnitChanged} />
  {/if}
  <p>Project {project}; selected SU {su}.
    {projectActive ? 'Explicit original whole-project scope; current SU is retained only for original quality authority when requested.' :
      'Whole selected-SU scope; the current plot and profile navigation do not filter this report.'}</p>
  <dl class="scope">
    <div><dt>Project database</dt><dd>{projectPath}</dd></div>
    <div><dt>Selected SU database</dt><dd>{suPath || 'No selected SU'}</dd></div>
  </dl>
  {#if settings}
    <dl class="settings" aria-label="Read-only report options">
      <div><dt>Report Title</dt><dd data-source-control="rptLvTitle">{settings.title === '' ? '"" (empty title)' : settings.title}</dd></div>
      <div><dt>Group by</dt><dd>{settings.grouping === 'none' ? 'None (source layer observations retained)' : groupLabel}</dd></div>
      <div><dt>Average</dt><dd>{settings.average === 'all-plots' ? (projectActive ? 'By n Plots — original project tuple denominator; quality filters by plot existence' : settings.unitGrouping === 'all-plots' ? 'By n Plots — joined sum / derived Env/current-list denominator' : settings.unitGrouping === 'selected-fields' ? 'By n Plots — joined sum / derived selected-field denominator' : settings.quality ? 'By n Plots — joined sum / qualified SU denominator' : 'By n Plots — joined sum / physical SU denominator') : 'Characteristic — mean of joined non-NULL observations'}</dd></div>
      <div><dt>Report units</dt><dd data-source-control="rptLvUnitGroups">{settings.unitGrouping === 'all-plots' ? (projectActive ? 'None — All plots from original Env rows' : 'None — All plots from Env/current selected list joins') : settings.unitGrouping === 'selected-fields' ? `Selected fields — ${settings.unitFields!.join(' & ')} (no separator)` : 'Site unit table — selected SU'}</dd></div>
      <div><dt>Constant species list</dt><dd>{settings.constantSpeciesList ? (projectActive ? 'On — same original project list; thresholds bypassed' : 'On — same selected-SU list; thresholds bypassed') : 'Off — strict presence and mean-cover thresholds apply'}</dd></div>
      <div><dt>Show species with presence &gt; than</dt><dd>{settings.presenceGreaterThan}%{settings.constantSpeciesList ? ' (not applied)' : ''}</dd></div>
      <div><dt>Show species with mean cover &gt; than</dt><dd>{settings.meanCoverGreaterThan}%{settings.constantSpeciesList ? ' (not applied)' : ''}</dd></div>
      <div><dt>Order by</dt><dd>{settings.constantSpeciesList ? `${groupLabel}, then species (constant-list source order)` :
        settings.grouping === 'none' ? (settings.order === 'presence' ? 'Descending presence, then species' : 'Species') :
        settings.order === 'presence' ? `${groupLabel}, then descending presence` : `${groupLabel}, then species`}</dd></div>
      <div><dt>{settings.selectedField !== undefined ? 'Show selected field' : settings.showSpeciesCode ? 'Show 8 char. code' : 'English names'}</dt><dd>{settings.selectedField !== undefined ?
        `${JSON.stringify(settings.selectedField)} — typed list and matched report scalars shown separately` : settings.showSpeciesCode ?
        'List and matched master codes shown separately; literal values are not truncated' :
        settings.showEnglishName ? 'List and matched report names shown separately' : 'Not shown'}</dd></div>
      <div><dt>Plot-quality filter</dt><dd data-source-control="optEnforceQC">{settings.quality ? (projectActive ? 'On — original-SU qualification; original project tuples retained by plot existence' : 'On — selected-SU qualification') : projectActive ? 'Off — original project tuples' : 'Off — all physical selected-SU memberships'}</dd></div>
      {#if settings.quality}
        {#each [{label:'Site',field:'SitePlotQuality',value:settings.quality.site},
          {label:'Vegetation',field:'VegPlotQuality',value:settings.quality.veg},
          {label:'Soil',field:'SoilPlotQuality',value:settings.quality.soil}] as criterion}
          <div><dt>{criterion.label} quality minimum</dt><dd data-source-control={criterion.field}>{JSON.stringify(criterion.value.minimum)}; include NULL/missing rank: {criterion.value.includeNull ? 'Yes' : 'No'}</dd></div>
        {/each}
      {/if}
    </dl>
  {/if}
  <div class="actions">
    <button data-source-control="btnViewReport" disabled={busy || operationHeld || workbookBarrier || preferenceBarrier || (projectActive ? !projectView?.outputReady : !settings || su === 'None')} onclick={show}>View Report</button>
    <button disabled={busy || operationHeld || workbookBarrier || preferenceBarrier || projectActive && !projectView?.outputReady} onclick={options}>Reload report options</button>
    {#if workbookEnabled}
      <button disabled={busy || operationHeld || workbookBarrier || preferenceBarrier || (projectActive ? !projectView?.outputReady : !settings || su === 'None' || su === 'USysSuTableDynamic')} onclick={reviewWorkbook}>Review unlumped XLSX workbook (no file yet)</button>
    {/if}
    {#if busy || projectView?.readBusy}<button onclick={cancel}>Cancel report read</button><p role="status">Reading owned report snapshots...</p>{/if}
    {#if projectView?.readBusy}<button data-project-report-cancel onclick={() => projectWorkspace?.cancelReportRead()}>Cancel original project report read</button><p role="status">Reading original project report snapshots...</p>{/if}
  </div>
  {#if activeWorkbookReview}
    {@const reviewed = activeWorkbookReview}
    <section class="workbook" aria-label="Reviewed unlumped workbook">
      <h2>Reviewed workbook — explicit unlumped scope</h2>
      <dl class="settings">
        <div><dt>Reviewed creation date</dt><dd>{reviewed.createdDate} (reused at publication)</dd></div>
        <div><dt>Quick report</dt><dd>{reviewed.options.quickReport ? 'On' : 'Off — source print layout retained'}</dd></div>
        <div><dt>Space between groups</dt><dd>{reviewed.options.spaceBetweenGroups ? 'On' : 'Off'}</dd></div>
        <div><dt>Report summary</dt><dd>{reviewed.options.reportSummary ? 'On' : 'Off'}</dd></div>
        <div><dt>Reviewed bytes</dt><dd>{reviewed.bytes}; SHA256 {reviewed.workbookSHA256}</dd></div>
        <div><dt>Source approval</dt><dd>{reviewed.approvalHash}</dd></div>
      </dl>
      <ul>{#each reviewed.sheets as sheet}<li>Unit {reportCellText(sheet.unit)} → worksheet “{sheet.name}”</li>{/each}</ul>
      {#each reviewed.skippedUnits as skipped}<p>Skipped unit {reportCellText(skipped.unit)}: {skipped.reason}.</p>{/each}
      {#if reviewed.summary}
        <p>ReportSummary: original Env rows {reviewed.summary.environmentRows};
          {#if 'sourcePlotRows' in reviewed.summary}
            original project non-NULL plot tuples {reviewed.summary.sourcePlotRows}.
            D:E original project tuples are an explicit desktop adaptation of the source's separate current-SU summary.
          {:else}
            {reviewed.preview.settings.unitGrouping === 'selected-fields' ? 'derived selected-field join rows (quality filters by plot existence)' :
              reviewed.preview.settings.unitGrouping === 'all-plots' ? 'derived Env/current-list join rows' :
              reviewed.preview.report.quality ? 'qualified weighted join occurrences' : 'non-NULL selected-SU PlotNumber rows'} {reviewed.summary.selectedPlotRows}.
          {/if}
          AllSpecs version {reportCellText(reviewed.summary.speciesVersion)}; metadata status {reviewed.summary.versionStatus}.</p>
        <details><summary>Original AllSpecs Description definitions</summary>
          {#each reviewed.summary.versionDefinitions.rows ?? [] as row}
            <p>Physical row {row.rowId}: {row.cells?.map(reportCellText).join('; ')}</p>
          {:else}<p>No Description definitions; absence remains distinct from NULL or empty text.</p>{/each}
        </details>
      {/if}
      <label for="vegetation-workbook-destination">New XLSX output file (existing files are never replaced)</label>
      <textarea id="vegetation-workbook-destination" rows="2" disabled={busy || operationHeld || workbookBarrier || preferenceBarrier} bind:value={destination}></textarea>
      <button disabled={busy || operationHeld || workbookBarrier || preferenceBarrier || !destination || !/\.xlsx$/i.test(destination)} onclick={publishWorkbook}>Publish reviewed unlumped XLSX workbook</button>
      <p class="guidance">Source settings/layout are read-only. Hidden lossless source metadata retains typed identities, reference candidates and diagnostics.
        Publication never replaces an existing file. Every returned or unknown outcome clears preparation; acknowledge it and explicitly obtain a new review before retrying.</p>
    </section>
  {/if}
  {#if su === 'None' && !projectActive}
    <p>{projectWorkspace ? 'Select an SU in Projects & context, or choose an explicit available whole-project scope before viewing the report.' :
      'Select an SU in Projects & context before viewing the report.'}</p>
  {/if}
  {#if settings?.constantSpeciesList}<p class="notice">Constant species list is enabled in retained YAML. Presence and mean-cover thresholds are bypassed, as in the source report; missing combinations have NULL statistics, not zero.</p>{/if}
  {#if settings?.grouping === 'none'}<p class="notice">The source's None mode changes row ordering, not cover aggregation.
    Layer observations remain separate and labelled; equal source sort keys use a deterministic layer/name tie-break.
    A constant species list retains its source layer-first order.</p>{/if}
  {#if settings?.grouping === 'lifeform'}<p class="notice">Lifeform conversion uses the original master/personal UNION,
    extended covers and the source's 99.9 cap before grouped insertion. Cover10 is not part of this conversion.
    Later master definitions and physical memberships retain their separate join weights; this is not the standalone Lifeform Summary.</p>{/if}
  {#if settings?.grouping === 'strata'}<p class="notice">Strata A/B use retained TotalA/TotalB when present;
    otherwise source cover sums are capped at 99. Extended B covers participate.
    Only positive A/B results are inserted; C/D retain every non-NULL cover, including zero or negative values.
    Physical Strata definitions retain their original join weights; Cover8/9/10 do not enter this conversion.</p>{/if}
  {#if preview}
    <h2>{preview.report.title === '' ? '"" (empty title)' : preview.report.title}</h2>
    <p role="status">{preview.report.units.length} units. No data, audits or configuration written.</p>
    {#if preview.report.quality}
      <p class="notice">Quality ranks use original DataQuality definitions, not the editor dropdown ordering.
        NULL flags apply to missing/NULL joined ranks; matches only in another list are excluded.
        {preview.report.sourceKind ?
          'Original project tuples are retained by qualified plot existence; reference fanout does not multiply their denominator or cover.' :
          preview.settings.unitGrouping === 'selected-fields' ?
          'For selected-field units, quality keeps derived rows by qualified plot existence; reference fanout does not multiply their denominator or cover.' :
          'Reference fanout weights the denominator and joined cover without inventing physical SU row IDs.'}</p>
      <details class="quality-provenance"><summary>Quality qualification provenance ({preview.report.quality.occurrences.length} occurrences)</summary>
        <p>Threshold reference row IDs — Site: {preview.report.quality.thresholdRowIds[0].join(', ')};
          Vegetation: {preview.report.quality.thresholdRowIds[1].join(', ')};
          Soil: {preview.report.quality.thresholdRowIds[2].join(', ')}.</p>
        <p>Excluded physical membership IDs: {preview.report.quality.excludedMembershipIds.length ? preview.report.quality.excludedMembershipIds.join(', ') : 'None'}.</p>
        {#each preview.report.quality.references as reference}
          <p class="quality-occurrence">Reference row {reference.rowId}: item {reportCellText(reference.item)};
            list {reportCellText(reference.listName)}; rank {reportCellText(reference.itemOrder)}.</p>
        {/each}
        {#each preview.report.quality.occurrences as occurrence}
          <p class="quality-occurrence">SU row {occurrence.membershipId}; plot {JSON.stringify(occurrence.plotNumber.text)};
            unit {reportCellText(occurrence.siteUnit)}; ENV row {occurrence.envRowId}; Admin row {occurrence.adminRowId};
            quality values (Site, Vegetation, Soil): {occurrence.values.map(reportCellText).join(', ')};
            reference rows (Site, Vegetation, Soil): {occurrence.listRowIds.map(id => id === null ? 'NULL join' : id).join(', ')}.</p>
        {/each}
      </details>
    {/if}
    {#if preview.report.sourceKind && preview.report.projectUnits}
      <p class="notice">Original project scope: {preview.report.sourceKind}. Complete original Build remains separate from emitted tuples.
        No synthetic SU row IDs or memberships are created.</p>
      <details class="quality-provenance"><summary>Original project tuple provenance ({preview.report.projectUnits.length} tuples)</summary>
        {#each preview.report.projectUnits as tuple}
          <p class="quality-occurrence">Original ENV row {tuple.envRowId}; Admin row {tuple.adminRowId ?? 'not used by All plots'};
            plot {reportCellText(tuple.plotNumber)}; derived unit {reportCellText(tuple.siteUnit)}.</p>
        {/each}
      </details>
    {/if}
    {#if preview.settings.unitGrouping === 'all-plots' && preview.report.noneUnits}
      <p class="notice">None report units join physical Env rows to the current selected list.
        Quality qualification runs first when enabled. The derived All plots unit does not replace original SU identities
        or expand to all-project scope.</p>
      <details class="quality-provenance"><summary>None unit join provenance ({preview.report.noneUnits.length} occurrences)</summary>
        {#each preview.report.noneUnits as occurrence}
          <p class="quality-occurrence">Outer ENV row {occurrence.envRowId}; original SU row {occurrence.membershipId};
            plot {JSON.stringify(occurrence.plotNumber.text)}; derived unit {reportCellText(occurrence.siteUnit)};
            {occurrence.qualityOccurrence === null ? 'unfiltered current list' : `quality occurrence index ${occurrence.qualityOccurrence}`}.</p>
        {/each}
      </details>
    {/if}
    {#if preview.settings.unitGrouping === 'selected-fields' && preview.report.selectedUnits}
      <p class="notice">Selected report units concatenate the ordered fields with no separator.
        Original selected-SU/Env/Admin joins define the derived denominator and cover weights.
        Quality, when enabled, filters by qualified plot existence rather than multiplying those joins.
        This is not all-project scope and original physical identities remain unchanged.</p>
      <details class="quality-provenance"><summary>Selected field unit join provenance ({preview.report.selectedUnits.length} occurrences)</summary>
        {#each preview.report.selectedUnits as occurrence}
          <p class="quality-occurrence">Original SU row {occurrence.membershipId};
            ENV row {occurrence.envRowId === null ? 'NULL join' : occurrence.envRowId};
            Admin row {occurrence.adminRowId === null ? 'NULL join' : occurrence.adminRowId};
            plot {reportCellText(occurrence.plotNumber)}; derived unit {reportCellText(occurrence.siteUnit)}.</p>
        {/each}
      </details>
    {/if}
    {#each preview.report.diagnostics as diagnostic}
      <p class="diagnostic">{diagnostic.code}: {JSON.stringify(diagnostic.identity)}; count {diagnostic.count}.</p>
    {/each}
    {#each preview.report.units as unit}
      <section class="unit">
        <h3>Site Unit — {unit.code.storage === 'null' ? 'Unassigned (NULL)' : reportCellText(unit.code)}</h3>
        <p class="unit-name">Long name: {unit.longName === null ? unit.nameStatus : unit.longName === '' ? '"" (empty name)' : unit.longName};
          reference status: {unit.nameStatus}.</p>
        <details><summary>Reference name candidates ({unit.nameCandidates.length})</summary>
          {#each unit.nameCandidates as candidate}<p class="name-candidate">Reference row {candidate.rowId}: {reportCellText(candidate.value)}</p>{/each}
        </details>
        {#if preview.report.sourceKind}
          <p>Original project tuple denominator: {unit.numPlots}; emitted tuple indexes: {unit.projectUnitIndexes?.length ?? 0}.</p>
          <details><summary>Original project tuple indexes</summary><p>{unit.projectUnitIndexes?.join(', ')}</p></details>
        {:else}
          <p>{preview.settings.unitGrouping === 'all-plots' ? 'Derived Env/current-list denominator' : preview.settings.unitGrouping === 'selected-fields' ? 'Derived selected-field denominator' : preview.settings.quality ? 'Qualified denominator' : 'Physical denominator'}: {unit.numPlots}; retained physical membership rows: {unit.membershipIds.length}.</p>
          <details><summary>Physical membership row IDs ({unit.membershipIds.length})</summary><p>{unit.membershipIds.join(', ')}</p></details>
        {/if}
        {#if unit.rows.length === 0}
          <p>No {groupLabel.toLowerCase()}/species rows satisfy the current report settings.</p>
        {:else}
          <div class="table-scroll" role="region" aria-label={`Vegetation table for unit ${reportCellText(unit.code)}`}>
            <table>
              <caption>Vegetation Table — Site Unit {reportCellText(unit.code)}</caption>
              <thead><tr>
                <th scope="col">{groupLabel}</th><th scope="col">Species</th>
                {#if preview.settings.showSpeciesCode}<th scope="col">List Code</th><th scope="col">Matched report Code</th>
                {:else if preview.settings.showEnglishName}<th scope="col">List English name</th><th scope="col">Matched report English name</th>
                {:else if preview.settings.selectedField !== undefined}<th scope="col">List {preview.settings.selectedField}</th><th scope="col">Matched report {preview.settings.selectedField}</th>{/if}
                <th scope="col">Presence (%)</th><th scope="col">Mean cover (%)</th>
                {#each unit.rows[0].plots as plot}<th scope="col">Plot {JSON.stringify(plot.plotNumber)}</th>{/each}
              </tr></thead>
              <tbody>{#each unit.rows as row}
                <tr>
                  <td>{reportCellText(row.layer)}</td><th scope="row">{reportCellText(row.species)}</th>
                  {#if preview.settings.showEnglishName || preview.settings.showSpeciesCode || preview.settings.selectedField !== undefined}<td>{supplementaryText(row.englishName)}</td><td>{supplementaryText(row.matchedName)}</td>{/if}
                  <td>{presenceText(row.presence)}</td><td>{numberText(row.meanCover)}</td>
                  {#each row.plots as plot}<td>{numberText(plot.cover)}</td>{/each}
                </tr>
              {/each}</tbody>
            </table>
          </div>
        {/if}
      </section>
    {/each}
  {/if}
  <p class="guidance">Read-only experimental vegetation preview. Options are loaded from YAML; only title persistence has a separate reviewed default-off gate.
    NULL, empty text, zero and absent statistics remain distinct. Source reference/membership multiplicities and English-name fanout are reported, not deduplicated.
    Unit-name candidates preserve original reference rows; conflicting or unsupported names are not chosen arbitrarily.
    Taxon lumping, combined variants, Source4 attributes and Excel automation remain unavailable.
    Workbook review and explicit no-replace XLSX publication require their separate default-off gate.</p>
</section>

<style>
  .report { display: grid; grid-template-columns: minmax(0, 1fr); gap: .75rem; min-width: 0; overflow-wrap: anywhere; }
  .workbook, .preferences { display: grid; grid-template-columns: minmax(0, 1fr); gap: .75rem; min-width: 0; }
  textarea, select { box-sizing: border-box; min-width: 0; border: 1px solid #cbd5e1; border-radius: .4rem; padding: .6rem; width: 100%; }
  h1 { font-size: 1.5rem; } h2 { font-size: 1.25rem; } h3, dt { font-weight: 600; }
  .scope, .settings { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 19rem), 1fr)); gap: .75rem 1.5rem; }
  .scope > div, .settings > div { min-width: 0; display: grid; gap: .25rem; align-content: start; }
  dd { white-space: pre-wrap; overflow-wrap: anywhere; }
  .actions { display: flex; flex-wrap: wrap; gap: .5rem; align-items: center; }
  button { border: 1px solid #cbd5e1; border-radius: .4rem; padding: .5rem .75rem; background: white; }
  button:disabled { opacity: .5; }
  .unit { min-width: 0; display: grid; gap: .5rem; }
  .unit-name, .name-candidate, .quality-occurrence { white-space: pre-wrap; overflow-wrap: anywhere; }
  .table-scroll { overflow-x: auto; max-width: 100%; border: 1px solid #cbd5e1; }
  table { border-collapse: collapse; width: 100%; }
  th, td { border: 1px solid #e2e8f0; padding: .5rem; text-align: left; min-width: 8rem; white-space: pre-wrap; overflow-wrap: anywhere; }
  .error { color: #b91c1c; } .diagnostic, .notice { color: #92400e; }
  .guidance { color: #57534e; font-size: .875rem; }
</style>
