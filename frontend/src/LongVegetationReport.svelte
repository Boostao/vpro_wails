<script module lang="ts">
  import { createLongVegetationGroupPreferencesRegistry } from './longVegetationGroupPreferences';
  const groupingSession = import.meta.env.VITE_LONG_VEGETATION_GROUP_PREFERENCES === 'true'
    ? createLongVegetationGroupPreferencesRegistry({
      strata: import.meta.env.VITE_LONG_VEGETATION_STRATA === 'true',
      lifeform: import.meta.env.VITE_LONG_VEGETATION_LIFEFORM === 'true',
    }) : null;
</script>

<script lang="ts">
  import { onMount, onDestroy, untrack } from 'svelte';
  import { ContextService, LongVegetationPreferencesService, LongVegetationStatisticsPreferencesService, VegetationWorkbookService, type LongVegetationSettings } from '../bindings/github.com/boostao/vpro-wails';
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

  let { contextId, project, projectPath, su, suPath, operationHeld = false, onBusyChange }: {
    contextId: string; project: string; projectPath: string; su: string; suPath: string;
    operationHeld?: boolean;
    onBusyChange: (busy: boolean) => void;
  } = $props();
  let settings = $state<LongVegetationSettings | null>(null);
  let busy = $state(false);
  let error = $state('');
  let preview = $state<ValidatedLongVegetationPreview | null>(null);
  let workbookReview = $state.raw<ValidatedVegetationWorkbookReview | null>(null);
  let destination = $state('');
  const reads = new ReadRequests();
  let generation = 0;
  const lifeformEnabled = import.meta.env.VITE_LONG_VEGETATION_LIFEFORM === 'true';
  const strataEnabled = import.meta.env.VITE_LONG_VEGETATION_STRATA === 'true';
  const codeEnabled = import.meta.env.VITE_LONG_VEGETATION_CODE === 'true';
  const workbookEnabled = import.meta.env.VITE_LONG_VEGETATION_WORKBOOK === 'true';
  const preferencesEnabled = import.meta.env.VITE_LONG_VEGETATION_PREFERENCES === 'true';
  const statisticsEnabled = import.meta.env.VITE_LONG_VEGETATION_STATISTICS_PREFERENCES === 'true';
  const switchesEnabled = import.meta.env.VITE_LONG_VEGETATION_SWITCH_PREFERENCES === 'true';
  const thresholdsEnabled = import.meta.env.VITE_LONG_VEGETATION_THRESHOLD_PREFERENCES === 'true';
  const qualityNullsEnabled = import.meta.env.VITE_LONG_VEGETATION_QUALITY_NULL_PREFERENCES === 'true';
  const workbookOwner = untrack(() => ({ contextId, project, projectPath, su, suPath }));
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
  const preferenceBarrier = $derived(titleBarrier || statisticsBarrier || switchBarrier || thresholdBarrier || groupBarrier || qualityNullBarrier);
  const workbookSession = workbookEnabled ? vegetationWorkbookPublicationSession(workbookOwner) : new VegetationWorkbookPublicationSession(workbookOwner);
  let publication = $state(workbookSession.view());
  const workbookBarrier = $derived(publication.busy || publication.blocked);
  const groupLabel = $derived(settings?.grouping === 'lifeform' ? 'Lifeform' : settings?.grouping === 'strata' ? 'Strata' : 'Layer');
  function reportBusy() { onBusyChange(busy || workbookBarrier || preferenceBarrier); }
  const unsubscribeWorkbook = workbookSession.subscribe(() => {
    publication = workbookSession.view();
    if (publication.blocked) workbookReview = null;
    reportBusy();
  });
  const unsubscribePreferences = preferences?.subscribe(() => {
    preferenceView = preferences.view(); preview = null; workbookReview = null;
    if (preferenceView.authorityUnknown || settings && preferenceView.saved && settings.title !== preferenceView.saved.title) settings = null;
    reportBusy();
  });
  const unsubscribeStatistics = statistics?.subscribe(() => {
    statisticsView = statistics.view(); preview = null; workbookReview = null;
    if (statisticsView.authorityUnknown || settings && statisticsView.saved &&
        (settings.average !== (statisticsView.saved.avgType === 10 ? 'all-plots' : 'observations') ||
         settings.order !== (statisticsView.saved.orderBy === 10 ? 'species' : 'presence'))) settings = null;
    reportBusy();
  });
  const unsubscribeSwitches = switches?.subscribe(() => {
    switchView = switches.view(); preview = null; workbookReview = null;
    if (switchView.authorityUnknown || settings && switchView.saved &&
        settings.constantSpeciesList !== switchView.saved.constantSpeciesList) settings = null;
    reportBusy();
  });
  const unsubscribeThresholds = thresholds?.subscribe(() => {
    thresholdView = thresholds.view(); preview = null; workbookReview = null;
    if (thresholdView.authorityUnknown || settings && thresholdView.saved &&
        (settings.presenceGreaterThan !== thresholdView.saved.presenceGreaterThan ||
         settings.meanCoverGreaterThan !== thresholdView.saved.meanCoverGreaterThan)) settings = null;
    reportBusy();
  });
  const unsubscribeGroups = groups?.subscribe(() => {
    groupView = groups.view(); preview = null; workbookReview = null;
    if (groupView.authorityUnknown || settings && groupView.saved &&
        settings.grouping !== (groupView.saved.groupBy === 1 ? 'layer' : groupView.saved.groupBy === 2 ? 'strata' :
          groupView.saved.groupBy === 3 ? 'lifeform' : 'none')) settings = null;
    reportBusy();
  });
  const unsubscribeQualityNulls = qualityNulls?.subscribe(() => {
    qualityNullView = qualityNulls.view(); preview = null; workbookReview = null;
    if (qualityNullView.authorityUnknown || settings?.quality && qualityNullView.saved &&
        (settings.quality.site.includeNull !== qualityNullView.saved.siteIncludeNulls ||
         settings.quality.veg.includeNull !== qualityNullView.saved.vegIncludeNulls ||
         settings.quality.soil.includeNull !== qualityNullView.saved.soilIncludeNulls)) settings = null;
    reportBusy();
  });

  function cancel() {
    generation++; reads.cancelAll(); preferences?.cancelLoad(); statistics?.cancelLoad(); busy = false; reportBusy();
  }
  async function options() {
    if (busy || operationHeld || workbookBarrier || preferenceBarrier) return;
    cancel(); const request = generation; settings = null; error = ''; preview = null; workbookReview = null;
    busy = true; reportBusy();
    try {
      const value = await reads.track(ContextService.GetLongVegetationOptions(contextId));
      if (request !== generation) return;
      const candidate = validateLongVegetationOptions(value, contextId, project, projectPath, su, suPath).settings;
      requireLongVegetationGrouping(candidate, lifeformEnabled, strataEnabled, codeEnabled);
      settings = candidate;
    } catch (cause) {
      if (request === generation) error = `Report options unavailable: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; reportBusy(); }
    }
  }
  async function show() {
    if (busy || operationHeld || workbookBarrier || preferenceBarrier || !settings || su === 'None') return;
    cancel(); const request = generation; error = ''; preview = null; workbookReview = null;
    busy = true; reportBusy();
    try {
      const value = await reads.track(ContextService.PreviewLongVegetation(contextId));
      if (request !== generation) return;
      const candidate = validateLongVegetationPreview(value, contextId, project, projectPath, su, suPath);
      requireLongVegetationGrouping(candidate.settings, lifeformEnabled, strataEnabled, codeEnabled);
      preview = candidate;
      settings = candidate.settings;
    } catch (cause) {
      if (request === generation) error = `Report unavailable; no data or configuration written: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; reportBusy(); }
    }
  }
  async function reviewWorkbook() {
    if (!workbookEnabled || busy || operationHeld || workbookBarrier || preferenceBarrier || !settings || su === 'None' || su === 'USysSuTableDynamic') return;
    cancel(); const request = generation;
    error = ''; preview = null; workbookReview = null; busy = true; reportBusy();
    try {
      const value = await reads.track(VegetationWorkbookService.GetReview(contextId, JSON.stringify({ scope: 'unlumped' })));
      if (request !== generation) return;
      if (!value) throw new Error('No owned workbook review returned.');
      const reviewed = workbookSession.prepare(value);
      requireLongVegetationGrouping(reviewed.preview.settings, lifeformEnabled, strataEnabled, codeEnabled);
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
    if (!preferences || busy || operationHeld || workbookBarrier || statisticsBarrier || switchBarrier || thresholdBarrier || groupBarrier || qualityNullBarrier || preferenceView?.busy || preferenceView?.blocked) return;
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
    if (!preferences) return;
    cancel(); error = ''; preferences.edit(value);
  }
  async function loadSavedStatistics() {
    if (!statistics || busy || operationHeld || workbookBarrier || titleBarrier || switchBarrier || thresholdBarrier || groupBarrier || qualityNullBarrier || statisticsView?.busy || statisticsView?.blocked) return;
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
    if (!statistics) return;
    cancel(); error = ''; statistics.editAverage(value);
  }
  function editOrder(value: string) {
    if (!statistics) return;
    cancel(); error = ''; statistics.editOrder(value);
  }
  const numberText = (value: number | null) => value === null ? 'NULL' : String(value);
  const presenceText = (value: number | null) => value === null ? 'NULL' : `${value * 100}%`;
  onMount(() => { void options(); });
  onDestroy(() => { unsubscribeWorkbook(); unsubscribePreferences?.(); unsubscribeStatistics?.(); unsubscribeSwitches?.(); unsubscribeThresholds?.(); unsubscribeGroups?.(); unsubscribeQualityNulls?.(); cancel(); });
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
  {#if preferences && preferenceView}
    <section class="preferences" aria-label="Saved Long Vegetation title">
      <label for="long-vegetation-title">Report Title</label>
      <textarea id="long-vegetation-title" data-source-control="rptLvTitle" rows="2"
        value={preferenceView.draft.title} aria-invalid={!!preferenceView.draftError}
        disabled={busy || operationHeld || workbookBarrier || switchMutationHeld || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld || statisticsView?.busy || statisticsView?.blocked || preferenceView.busy || preferenceView.blocked || !preferenceView.initialized}
        oninput={event => { const value = event.currentTarget.value; editTitle(value); }}></textarea>
      <div class="actions">
        <button data-long-vegetation-preferences-load
          disabled={busy || operationHeld || workbookBarrier || statisticsBarrier || switchBarrier || thresholdBarrier || groupBarrier || qualityNullBarrier || preferenceView.busy || preferenceView.blocked}
          onclick={loadSavedTitle}>Load saved title</button>
        <button data-long-vegetation-preferences-save
          disabled={busy || operationHeld || workbookBarrier || preferenceBarrier || !preferenceView.saved}
          onclick={saveTitle}>Save title preferences</button>
        <button data-long-vegetation-preferences-undo
          disabled={busy || operationHeld || workbookBarrier || switchMutationHeld || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld || statisticsView?.busy || statisticsView?.blocked || preferenceView.busy || preferenceView.blocked || !preferenceView.saved}
          onclick={() => { cancel(); preferences.undo(); }}>Undo title draft</button>
        {#if preferenceView.blocked && !preferenceView.busy}
          <button data-long-vegetation-preferences-acknowledge disabled={busy || operationHeld || workbookBarrier || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld}
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
        disabled={busy || operationHeld || workbookBarrier || switchMutationHeld || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld || preferenceView?.busy || preferenceView?.blocked || statisticsView.busy || statisticsView.blocked || !statisticsView.initialized}
        onchange={event => { const value = event.currentTarget.value; editAverage(value); }}>
        <option value="" disabled>Load saved Average</option>
        <option value="10">By n Plots</option><option value="20">Characteristic</option>
      </select>
      <label for="long-vegetation-order">Order by</label>
      <select id="long-vegetation-order" data-source-control="rptLvOrderBy" value={statisticsView.draft.orderBy}
        aria-invalid={!!statisticsView.draftError}
        disabled={busy || operationHeld || workbookBarrier || switchMutationHeld || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld || preferenceView?.busy || preferenceView?.blocked || statisticsView.busy || statisticsView.blocked || !statisticsView.initialized}
        onchange={event => { const value = event.currentTarget.value; editOrder(value); }}>
        <option value="" disabled>Load saved Order by</option>
        <option value="10">Species</option><option value="20">Presence</option>
      </select>
      <div class="actions">
        <button data-long-vegetation-statistics-load
          disabled={busy || operationHeld || workbookBarrier || titleBarrier || switchBarrier || thresholdBarrier || groupBarrier || qualityNullBarrier || statisticsView.busy || statisticsView.blocked}
          onclick={loadSavedStatistics}>Load saved Average/Order</button>
        <button data-long-vegetation-statistics-save
          disabled={busy || operationHeld || workbookBarrier || preferenceBarrier || !statisticsView.saved}
          onclick={saveStatistics}>Save Average/Order preferences</button>
        <button data-long-vegetation-statistics-undo
          disabled={busy || operationHeld || workbookBarrier || switchMutationHeld || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld || preferenceView?.busy || preferenceView?.blocked || statisticsView.busy || statisticsView.blocked || !statisticsView.saved}
          onclick={() => { cancel(); statistics.undo(); }}>Undo Average/Order draft</button>
        {#if statisticsView.blocked && !statisticsView.busy}
          <button data-long-vegetation-statistics-acknowledge disabled={busy || operationHeld || workbookBarrier || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld}
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
      operationHeld={busy || operationHeld || workbookBarrier || titleBarrier || statisticsBarrier || thresholdBarrier || groupBarrier || qualityNullBarrier}
      mutationHeld={busy || operationHeld || workbookBarrier || !!preferenceView?.busy || !!preferenceView?.blocked ||
        !!statisticsView?.busy || !!statisticsView?.blocked || thresholdMutationHeld || groupMutationHeld || qualityNullMutationHeld}
      onReadBusyChange={value => { switchReadBusy = value; reportBusy(); }} />
  {/if}
  {#if thresholds}
    <LongVegetationThresholdPreferences contextId={contextId} preferences={thresholds}
      operationHeld={busy || operationHeld || workbookBarrier || titleBarrier || statisticsBarrier || switchBarrier || groupBarrier || qualityNullBarrier}
      mutationHeld={busy || operationHeld || workbookBarrier || !!preferenceView?.busy || !!preferenceView?.blocked ||
        !!statisticsView?.busy || !!statisticsView?.blocked || switchMutationHeld || groupMutationHeld || qualityNullMutationHeld}
      onReadBusyChange={value => { thresholdReadBusy = value; reportBusy(); }} />
  {/if}
  {#if groups}
    <LongVegetationGroupPreferences contextId={contextId} preferences={groups}
      operationHeld={busy || operationHeld || workbookBarrier || titleBarrier || statisticsBarrier || switchBarrier || thresholdBarrier || qualityNullBarrier}
      mutationHeld={busy || operationHeld || workbookBarrier || !!preferenceView?.busy || !!preferenceView?.blocked ||
        !!statisticsView?.busy || !!statisticsView?.blocked || switchMutationHeld || thresholdMutationHeld || qualityNullMutationHeld}
      onReadBusyChange={value => { groupReadBusy = value; reportBusy(); }} />
  {/if}
  {#if qualityNulls}
    <LongVegetationQualityNullPreferences contextId={contextId} preferences={qualityNulls}
      operationHeld={busy || operationHeld || workbookBarrier || titleBarrier || statisticsBarrier || switchBarrier || thresholdBarrier || groupBarrier}
      mutationHeld={busy || operationHeld || workbookBarrier || !!preferenceView?.busy || !!preferenceView?.blocked ||
        !!statisticsView?.busy || !!statisticsView?.blocked || switchMutationHeld || thresholdMutationHeld || groupMutationHeld}
      onReadBusyChange={value => { qualityNullReadBusy = value; reportBusy(); }} />
  {/if}
  <p>Project {project}; selected SU {su}. Whole selected-SU scope; the current plot and profile navigation do not filter this report.</p>
  <dl class="scope">
    <div><dt>Project database</dt><dd>{projectPath}</dd></div>
    <div><dt>Selected SU database</dt><dd>{suPath || 'No selected SU'}</dd></div>
  </dl>
  {#if settings}
    <dl class="settings" aria-label="Read-only report options">
      <div><dt>Report Title</dt><dd data-source-control="rptLvTitle">{settings.title === '' ? '"" (empty title)' : settings.title}</dd></div>
      <div><dt>Group by</dt><dd>{settings.grouping === 'none' ? 'None (source layer observations retained)' : groupLabel}</dd></div>
      <div><dt>Average</dt><dd>{settings.average === 'all-plots' ? (settings.quality ? 'By n Plots — joined sum / qualified SU denominator' : 'By n Plots — joined sum / physical SU denominator') : 'Characteristic — mean of joined non-NULL observations'}</dd></div>
      <div><dt>Constant species list</dt><dd>{settings.constantSpeciesList ? 'On — same selected-SU list; thresholds bypassed' : 'Off — strict presence and mean-cover thresholds apply'}</dd></div>
      <div><dt>Show species with presence &gt; than</dt><dd>{settings.presenceGreaterThan}%{settings.constantSpeciesList ? ' (not applied)' : ''}</dd></div>
      <div><dt>Show species with mean cover &gt; than</dt><dd>{settings.meanCoverGreaterThan}%{settings.constantSpeciesList ? ' (not applied)' : ''}</dd></div>
      <div><dt>Order by</dt><dd>{settings.constantSpeciesList ? `${groupLabel}, then species (constant-list source order)` :
        settings.grouping === 'none' ? (settings.order === 'presence' ? 'Descending presence, then species' : 'Species') :
        settings.order === 'presence' ? `${groupLabel}, then descending presence` : `${groupLabel}, then species`}</dd></div>
      <div><dt>{settings.showSpeciesCode ? 'Show 8 char. code' : 'English names'}</dt><dd>{settings.showSpeciesCode ?
        'List and matched master codes shown separately; literal values are not truncated' :
        settings.showEnglishName ? 'List and matched report names shown separately' : 'Not shown'}</dd></div>
      <div><dt>Plot-quality filter</dt><dd data-source-control="optEnforceQC">{settings.quality ? 'On — selected-SU qualification' : 'Off — all physical selected-SU memberships'}</dd></div>
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
    <button data-source-control="btnViewReport" disabled={busy || operationHeld || workbookBarrier || preferenceBarrier || !settings || su === 'None'} onclick={show}>View Report</button>
    <button disabled={busy || operationHeld || workbookBarrier || preferenceBarrier} onclick={options}>Reload report options</button>
    {#if workbookEnabled}
      <button disabled={busy || operationHeld || workbookBarrier || preferenceBarrier || !settings || su === 'None' || su === 'USysSuTableDynamic'} onclick={reviewWorkbook}>Review unlumped XLSX workbook (no file yet)</button>
    {/if}
    {#if busy}<button onclick={cancel}>Cancel report read</button><p role="status">Reading owned report snapshots...</p>{/if}
  </div>
  {#if workbookReview}
    <section class="workbook" aria-label="Reviewed unlumped workbook">
      <h2>Reviewed workbook — explicit unlumped scope</h2>
      <dl class="settings">
        <div><dt>Reviewed creation date</dt><dd>{workbookReview.createdDate} (reused at publication)</dd></div>
        <div><dt>Quick report</dt><dd>{workbookReview.options.quickReport ? 'On' : 'Off — source print layout retained'}</dd></div>
        <div><dt>Space between groups</dt><dd>{workbookReview.options.spaceBetweenGroups ? 'On' : 'Off'}</dd></div>
        <div><dt>Report summary</dt><dd>{workbookReview.options.reportSummary ? 'On' : 'Off'}</dd></div>
        <div><dt>Reviewed bytes</dt><dd>{workbookReview.bytes}; SHA256 {workbookReview.workbookSHA256}</dd></div>
        <div><dt>Source approval</dt><dd>{workbookReview.approvalHash}</dd></div>
      </dl>
      <ul>{#each workbookReview.sheets as sheet}<li>Unit {reportCellText(sheet.unit)} → worksheet “{sheet.name}”</li>{/each}</ul>
      {#each workbookReview.skippedUnits as skipped}<p>Skipped unit {reportCellText(skipped.unit)}: {skipped.reason}.</p>{/each}
      {#if workbookReview.summary}
        <p>ReportSummary: original Env rows {workbookReview.summary.environmentRows};
          {workbookReview.preview.report.quality ? 'qualified weighted join occurrences' : 'non-NULL selected-SU PlotNumber rows'} {workbookReview.summary.selectedPlotRows}.
          AllSpecs version {reportCellText(workbookReview.summary.speciesVersion)}; metadata status {workbookReview.summary.versionStatus}.</p>
        <details><summary>Original AllSpecs Description definitions</summary>
          {#each workbookReview.summary.versionDefinitions.rows ?? [] as row}
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
  {#if su === 'None'}<p>Select an SU in Projects &amp; context before viewing the report.</p>{/if}
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
        Reference fanout weights the denominator and joined cover without inventing physical SU row IDs.</p>
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
        <p>{preview.settings.quality ? 'Qualified denominator' : 'Physical denominator'}: {unit.numPlots}; retained physical membership rows: {unit.membershipIds.length}.</p>
        <details><summary>Physical membership row IDs ({unit.membershipIds.length})</summary><p>{unit.membershipIds.join(', ')}</p></details>
        {#if unit.rows.length === 0}
          <p>No {groupLabel.toLowerCase()}/species rows satisfy the current report settings.</p>
        {:else}
          <div class="table-scroll" role="region" aria-label={`Vegetation table for unit ${reportCellText(unit.code)}`}>
            <table>
              <caption>Vegetation Table — Site Unit {reportCellText(unit.code)}</caption>
              <thead><tr>
                <th scope="col">{groupLabel}</th><th scope="col">Species</th>
                {#if preview.settings.showSpeciesCode}<th scope="col">List Code</th><th scope="col">Matched report Code</th>
                {:else if preview.settings.showEnglishName}<th scope="col">List English name</th><th scope="col">Matched report English name</th>{/if}
                <th scope="col">Presence (%)</th><th scope="col">Mean cover (%)</th>
                {#each unit.rows[0].plots as plot}<th scope="col">Plot {JSON.stringify(plot.plotNumber)}</th>{/each}
              </tr></thead>
              <tbody>{#each unit.rows as row}
                <tr>
                  <td>{reportCellText(row.layer)}</td><th scope="row">{reportCellText(row.species)}</th>
                  {#if preview.settings.showEnglishName || preview.settings.showSpeciesCode}<td>{reportCellText(row.englishName)}</td><td>{reportCellText(row.matchedName)}</td>{/if}
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
