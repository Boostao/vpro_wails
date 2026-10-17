<script lang="ts">
  import { onDestroy } from 'svelte';
  import { SummaryVegetationService, SummaryVegetationPreferencesService, SummaryVegetationQualityPreferencesService, SummaryVegetationPresentationPreferencesService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { summaryVegetationSession, type SummaryVegetationPort } from './summaryVegetation';
  import { summaryVegetationPreferenceSession } from './summaryVegetationPreferenceSessions';
  import { summaryVegetationQualityPreferenceSession } from './summaryVegetationQualityPreferenceSessions';
  import SummaryVegetationQualityPreferences from './SummaryVegetationQualityPreferences.svelte';
  import { summaryVegetationPresentationPreferenceSession } from './summaryVegetationPresentationPreferenceSessions';
  import SummaryVegetationPresentationPreferences from './SummaryVegetationPresentationPreferences.svelte';
  import SummaryVegetationFieldList from './SummaryVegetationFieldList.svelte';
  import { summaryVegetationFieldListDesktop } from './summaryVegetationFieldListDesktop';
  import SummaryVegetationEnvironmental from './SummaryVegetationEnvironmental.svelte';
  import SummaryVegetationWorkbookPanel from './SummaryVegetationWorkbookPanel.svelte';
  import { summaryEnvironmentalDesktop, summaryEnvironmentalEnabled, summaryEnvironmentalNumericEnabled, summaryEnvironmentalLifeformEnabled, summaryEnvironmentalPort } from './summaryVegetationEnvironmentalDesktop';
  import type { SummaryEnvironmentalSession, SummaryEnvironmentalNumericSession, SummaryEnvironmentalLifeformSession } from './summaryVegetationEnvironmental';

  let { contextId, project, projectPath, su, suPath, operationHeld: operationHeldProp = false, onBusyChange }: {
    contextId: string; project: string; projectPath: string; su: string; suPath: string;
    operationHeld?: boolean; onBusyChange: (busy: boolean) => void;
  } = $props();
  const enabled = import.meta.env.VITE_SUMMARY_VEGETATION === 'true';
  const groupedWorkbookEnabled = import.meta.env.VITE_SUMMARY_VEGETATION_GROUPED_WORKBOOK === 'true';
  const reads = new ReadRequests();
  function ownedContext() { return { contextId, project, projectPath, su, suPath }; }
  const session = summaryVegetationSession(ownedContext());
  let environmentalHeld = $state(false);
  let workbookHeld = $state(false);
  const operationHeld = $derived(operationHeldProp || environmentalHeld || workbookHeld);
  let environmentalSession = $state.raw<SummaryEnvironmentalSession | SummaryEnvironmentalNumericSession | SummaryEnvironmentalLifeformSession | null>(null);
  let environmentalError = $state('');
  if (summaryEnvironmentalEnabled) {
    try { environmentalSession = summaryEnvironmentalDesktop.session(ownedContext()); }
    catch (cause) { environmentalError = `Combined environmental inspection unavailable: ${String(cause)}`; }
  }
  const preferences = summaryVegetationPreferenceSession?.(ownedContext()) ?? null;
  const quality = summaryVegetationQualityPreferenceSession?.(ownedContext()) ?? null;
  const presentation = summaryVegetationPresentationPreferenceSession?.(ownedContext()) ?? null;
  const fieldList = summaryVegetationFieldListDesktop;
  const fieldListScopeAvailable = fieldList.contextAvailable(ownedContext());
  let fieldListState = $state.raw(fieldListScopeAvailable ? fieldList.state(ownedContext()) : null);
  let fieldListHeld = $state(false);
  let fieldListBusy = $state(false);
  let fieldListError = $state('');
  let presentationView = $state.raw(presentation?.view() ?? null);
  let presentationReadBusy = $state(false);
  let qualityView = $state.raw(quality?.view() ?? null);
  let qualityReadBusy = $state(false);
  let preferenceView = $state.raw(preferences?.view() ?? null);
  let preferenceReadBusy = $state(false);
  let preferenceReadError = $state('');
  let preferenceGeneration = 0;
  const preferenceReads = new ReadRequests();
  const port: SummaryVegetationPort = {
    preview: (average, display) => reads.track(SummaryVegetationService.Preview(contextId, average, display)),
    previewClass: (average, display, showClass) => reads.track(SummaryVegetationService.PreviewClass(contextId, average, display, showClass)),
    previewGrouped: (average, groupAndOrder) => reads.track(SummaryVegetationService.PreviewGrouped(contextId, average, groupAndOrder)),
    previewGroupedDisplay: (average, groupAndOrder, display, showClass) => reads.track(SummaryVegetationService.PreviewGroupedDisplay(contextId, average, groupAndOrder, display, showClass)),
    cancel: () => reads.cancelAll(),
    validationYield: () => new Promise(resolve => setTimeout(resolve, 0)),
  };
  let viewState = $state.raw(session.view());
  function reportBusy() {
    onBusyChange(viewState.busy || (!preferences && !!viewState.draftError) || preferenceReadBusy ||
      !!preferenceView && (preferenceView.busy || preferenceView.blocked || !!preferenceView.draftError) ||
      qualityReadBusy || !!qualityView && (qualityView.busy || qualityView.blocked || !!qualityView.draftError) ||
      presentationReadBusy || !!presentationView && (presentationView.busy || presentationView.blocked || !!presentationView.draftError) || fieldListHeld || environmentalHeld || workbookHeld);
  }
  const unsubscribe = session.subscribe(() => { viewState = session.view(); reportBusy(); });
  const unsubscribeEnvironment = summaryEnvironmentalEnabled ? summaryEnvironmentalDesktop.subscribe(() => {
    environmentalHeld = summaryEnvironmentalDesktop.activity().busy;
    reportBusy();
  }) : undefined;
  function syncPreferenceDraft() {
    if (!preferenceView?.initialized || viewState.busy) return;
    const draft = preferenceView.draft;
    const group = Number(draft.groupBy) + 10, average = Number(draft.average),
      display = Number(draft.display), show = Number(draft.showClass);
    if (viewState.groupAndOrder !== group || viewState.average !== average || viewState.display !== display || viewState.showClass !== show)
      session.edit(average, display, show, group);
  }
  const unsubscribePreferences = preferences?.subscribe(() => {
    preferenceView = preferences.view(); syncPreferenceDraft(); reportBusy();
  });
  const unsubscribeQuality = quality?.subscribe(() => { qualityView = quality.view(); reportBusy(); });
  const unsubscribePresentation = presentation?.subscribe(() => { presentationView = presentation.view(); reportBusy(); });
  const unsubscribeFieldList = fieldListScopeAvailable ? fieldList.subscribe(() => {
    fieldListState = fieldList.state(ownedContext());
    const activity = fieldList.activity();
    fieldListBusy = activity.busy;
    fieldListHeld = activity.busy || activity.blocked;
    reportBusy();
  }) : undefined;
  async function loadFieldList() {
    if (!fieldListScopeAvailable || fieldListPeerDisabled || fieldListPeerValidation || fieldListState?.readBusy) return;
    fieldListError = '';
    try { await fieldList.load(ownedContext()); } catch (cause) { fieldListError = String(cause); }
  }
  function dismissInitialFieldListReadError() {
    if (!fieldListScopeAvailable || !fieldListState?.canDismissInitialReadError) return;
    fieldListError = '';
    try { fieldList.dismissInitialReadError(ownedContext()); } catch (cause) { fieldListError = String(cause); }
  }
  const presentationMutationHeld = $derived(presentationReadBusy || !!presentationView && (presentationView.busy || presentationView.blocked));
  const presentationInvalid = $derived(!!presentationView?.draftError || !!presentationView?.authorityUnknown || !!presentationView?.readError);
  const qualityMutationHeld = $derived(qualityReadBusy || !!qualityView && (qualityView.busy || qualityView.blocked));
  const qualityInvalid = $derived(!!qualityView?.draftError || !!qualityView?.authorityUnknown || !!qualityView?.readError);
  const qualityUnavailable = $derived(!!quality && (!qualityView?.saved || qualityView.saved.enforce || qualityInvalid));
  const unavailableScope = $derived(!contextId || su === 'None' || su === 'USysSuTableDynamic');
  const preferenceHeld = $derived(preferenceReadBusy || !!preferenceView && (preferenceView.busy || preferenceView.blocked));
  const preferenceInvalid = $derived(!!preferenceView?.draftError || !!preferenceView?.authorityUnknown || !!preferenceView?.readError);
  const fieldListPeerDisabled = $derived(operationHeld || viewState.busy || preferenceHeld || qualityMutationHeld || presentationMutationHeld);
  const fieldListPeerValidation = $derived(preferenceInvalid ? 'Resolve Summary options validation.'
    : qualityInvalid ? 'Resolve Summary quality validation.'
    : presentationInvalid ? 'Resolve Summary presentation validation.' : '');
  const fieldListEnabledFromPresentation = $derived(!!presentationView?.saved && !presentationInvalid &&
    presentationView.draft?.includeEnv === true);
  const editHeld = $derived(!enabled || viewState.busy || operationHeld || fieldListBusy || preferenceHeld || qualityMutationHeld || presentationMutationHeld ||
    !!preferences && !preferenceView?.saved);
  function cancelPreferenceRead() {
    preferenceGeneration++; preferenceReads.cancelAll(); preferences?.cancelLoad();
    preferenceReadBusy = false; reportBusy();
  }
  async function loadPreferences() {
    if (!preferences || viewState.busy || operationHeld || fieldListHeld || preferenceHeld || qualityMutationHeld || qualityInvalid || presentationMutationHeld || presentationInvalid) return;
    cancelPreferenceRead();
    const request = preferenceGeneration, token = preferences.beginLoad();
    preferenceReadError = ''; preferenceReadBusy = true; reportBusy();
    try {
      const value = await preferenceReads.track(SummaryVegetationPreferencesService.GetSummaryVegetationPreferences(contextId));
      if (request !== preferenceGeneration) return;
      preferences.applyLoaded(value, token);
    } catch (cause) {
      if (request === preferenceGeneration) preferences.rejectRead(cause);
    } finally {
      if (request === preferenceGeneration) { preferenceReadBusy = false; reportBusy(); }
    }
  }
  async function savePreferences() {
    if (!preferences || viewState.busy || operationHeld || fieldListHeld || preferenceHeld || preferenceInvalid || qualityMutationHeld || qualityInvalid || presentationMutationHeld || presentationInvalid) return;
    cancelPreferenceRead(); preferenceReadError = '';
    try {
      await preferences.save(value => SummaryVegetationPreferencesService.SaveSummaryVegetationPreferences(contextId, JSON.stringify(value)));
    } catch (cause) {
      preferenceReadError = String(cause);
    }
  }
  function editPreference(field: 'groupBy' | 'average' | 'display' | 'showClass', raw: string) {
    if (!preferences || editHeld) return;
    cancelPreferenceRead(); preferenceReadError = ''; preferences.editOption(field, raw);
  }
  function changeAverage(event: Event) {
    const raw = (event.currentTarget as HTMLSelectElement).value;
    if (preferences) { editPreference('average', raw); return; }
    const current = Number(raw);
    session.edit(current, viewState.display, viewState.showClass || 1, viewState.groupAndOrder);
  }
  function changeDisplay(event: Event) {
    const raw = (event.currentTarget as HTMLSelectElement).value;
    if (preferences) { editPreference('display', raw); return; }
    const current = Number(raw);
    session.edit(viewState.average, current, viewState.showClass || 1, viewState.groupAndOrder);
  }
  function changeShowClass(event: Event) {
    const raw = (event.currentTarget as HTMLSelectElement).value;
    if (preferences) { editPreference('showClass', raw); return; }
    const current = Number(raw);
    session.edit(viewState.average, viewState.display, current, viewState.groupAndOrder);
  }
  function changeGroup(event: Event) {
    const raw = (event.currentTarget as HTMLSelectElement).value;
    if (preferences) { editPreference('groupBy', String(Number(raw) - 10)); return; }
    const current = Number(raw);
    session.edit(viewState.average, viewState.display, viewState.showClass || 1, current);
  }
  onDestroy(() => {
    session.cancel(port); unsubscribe(); unsubscribePreferences?.(); unsubscribeQuality?.(); unsubscribePresentation?.();
    unsubscribeFieldList?.(); unsubscribeEnvironment?.(); if (fieldListScopeAvailable) fieldList.detach(ownedContext());
    cancelPreferenceRead(); onBusyChange(false);
  });
</script>

<section data-summary-vegetation aria-label="Summary Vegetation" class="summary-vegetation">
  <h1>Summary Vegetation</h1>
  {#if viewState.error}<p role="alert" class="error">{viewState.error}</p>{/if}
  {#if viewState.draftError}<p role="alert" class="error">{viewState.draftError}</p>{/if}
  {#if preferenceReadError}<p role="alert" class="error">{preferenceReadError}</p>{/if}
  {#if preferenceView?.readError}<p role="alert" class="error">{preferenceView.readError}</p>{/if}
  {#if preferenceView?.draftError}<p role="alert" class="error">{preferenceView.draftError}</p>{/if}
  {#if preferenceView?.error}<p role="alert" class="error">{preferenceView.error}</p>{/if}
  <p class="owner">Project: {project}<br />{projectPath}<br />Current Site Unit Table: {su}<br />{suPath}</p>
  {#if !enabled}<p>This preview is disabled in this build.</p>{/if}
  {#if unavailableScope}<p>A selected normal Site Unit Table is required. Hierarchy and dynamic breaks are unavailable.</p>{/if}
  <div class="options">
    <label>Summary Report Units
      <select disabled aria-label="Summary Report Units"><option>Current Site Unit Table</option></select>
    </label>
    <label>Group by
      <select value={viewState.groupAndOrder} onchange={changeGroup} disabled={editHeld} data-summary-group data-source-control="rptSvGroupBy">
        <option value={11}>Layer</option><option value={12} disabled={viewState.display === 2}>Strata</option>
        <option value={13} disabled={viewState.display === 2}>Lifeform</option><option value={14}>None</option>
      </select>
    </label>
    <label>Display Values
      <select value={viewState.display} onchange={changeDisplay} disabled={editHeld} data-summary-display data-source-control="rptSvValueEq">
        <option value={1}>% Presence and % Cover</option>
        <option value={2} disabled={viewState.groupAndOrder === 12 || viewState.groupAndOrder === 13}>Presence Class and Significance Class</option>
        <option value={3}>Presence Class and % Cover</option>
        <option value={6}>% Cover</option>
        <option value={4}>Prominence Class</option>
        <option value={5}>Goldstream Class</option>
      </select>
    </label>
    <label>Show
      <select value={viewState.showClass} onchange={changeShowClass}
        disabled={editHeld || viewState.display <= 3} data-summary-show data-source-control="OptShowClassValue">
        <option value={1}>Class (historical default 1)</option>
        <option value={10}>Class</option><option value={20}>Value</option>
      </select>
    </label>
    <label>Calculate
      <select value={viewState.average} onchange={changeAverage} disabled={editHeld} data-summary-average data-source-control="rptSvAvgType">
        <option value={1}>By n Plots</option><option value={2}>Characteristic</option>
      </select>
    </label>
  </div>
  {#if preferences && preferenceView}
    <section data-summary-preferences aria-label="Saved Summary options">
      {#if preferenceView.busy}<p role="status">Saving Summary options; Save cannot be cancelled.</p>{/if}
      {#each preferenceView.receipts as receipt}
        <p data-summary-preference-receipt role={receipt.kind === 'known' ? 'status' : 'alert'}>
          {#if receipt.kind === 'known'}Preference receipt: changed {String(receipt.outcome.changed)};
            committed {String(receipt.outcome.committed)}. {receipt.outcome.errorMessage}
          {:else}Preference outcome unknown; do not replay. {receipt.errorMessage}{/if}
        </p>
      {/each}
      <div class="actions">
        <button data-summary-preferences-load disabled={viewState.busy || operationHeld || fieldListHeld || preferenceHeld || qualityMutationHeld || qualityInvalid || presentationMutationHeld || presentationInvalid} onclick={loadPreferences}>Load saved Summary options</button>
        <button data-summary-preferences-save disabled={viewState.busy || operationHeld || fieldListHeld || preferenceHeld || preferenceInvalid || qualityMutationHeld || qualityInvalid || presentationMutationHeld || presentationInvalid || !preferenceView.saved} onclick={savePreferences}>Save Summary options</button>
        <button data-summary-preferences-undo disabled={viewState.busy || operationHeld || fieldListBusy || preferenceHeld || qualityMutationHeld || presentationMutationHeld || !preferenceView.saved}
          onclick={() => { cancelPreferenceRead(); preferences.undo(); }}>Undo Summary option draft</button>
        {#if preferenceView.blocked && !preferenceView.busy}
          <button data-summary-preferences-acknowledge disabled={operationHeld || fieldListBusy} onclick={() => preferences.acknowledge()}>Acknowledge Summary preference outcome</button>
        {/if}
        {#if preferenceReadBusy}<button data-summary-preferences-cancel onclick={cancelPreferenceRead}>Cancel saved options read</button>{/if}
      </div>
      <p>Explicit Load uses saved options in the controls above. Save changes only the five reviewed YAML preferences, never project data or source audits.
        Display changes outside modes 1/3 clear the historical paired limit; other edits retain it.
        Acknowledge retains receipts. Unknown outcomes require explicit Load, not replay.</p>
    </section>
  {/if}
  {#if quality}
    <SummaryVegetationQualityPreferences contextId={contextId} preferences={quality}
      port={{ read: id => SummaryVegetationQualityPreferencesService.GetSummaryVegetationQualityPreferences(id),
        save: (id, request) => SummaryVegetationQualityPreferencesService.SaveSummaryVegetationQualityPreferences(id, request) }}
      operationHeld={operationHeld || fieldListHeld || viewState.busy || preferenceHeld || preferenceInvalid || presentationMutationHeld || presentationInvalid}
      mutationHeld={operationHeld || fieldListBusy || viewState.busy || preferenceHeld || presentationMutationHeld}
      onReadBusyChange={value => { qualityReadBusy = value; reportBusy(); }} />
    {#if !qualityView?.saved}<p>Explicitly Load saved quality options before preview; no quality defaults are assumed.</p>{/if}
    {#if qualityView?.saved?.enforce}<p role="status">Saved QC enforcement is active. Qualified Summary preview is unavailable; no unfiltered report is returned.</p>{/if}
  {/if}
  {#if presentation}
    <SummaryVegetationPresentationPreferences contextId={contextId} preferences={presentation}
      port={{ read: id => SummaryVegetationPresentationPreferencesService.GetSummaryVegetationPresentationPreferences(id),
        save: (id, request) => SummaryVegetationPresentationPreferencesService.SaveSummaryVegetationPresentationPreferences(id, request) }}
      operationHeld={operationHeld || fieldListHeld || viewState.busy || preferenceHeld || preferenceInvalid || qualityMutationHeld || qualityInvalid}
      mutationHeld={operationHeld || fieldListBusy || viewState.busy || preferenceHeld || qualityMutationHeld}
      onReadBusyChange={value => { presentationReadBusy = value; reportBusy(); }} />
  {/if}
  {#if fieldList.enabled && !fieldListScopeAvailable}
    <p role="status">Environmental field-list unavailable: a complete literal owned project/current table context and retained session capacity are required. None may have an empty table path; no retained state is evicted.</p>
  {/if}
  {#if fieldList.enabled && fieldListState}
    <section class="linked-field-list" data-summary-field-list-desktop aria-label="Linked Summary environmental field list">
      {#if fieldListError}<p role="alert">{fieldListError}</p>{/if}
      {#if fieldListState.readError}<p role="alert">{fieldListState.readError}</p>{/if}
      {#if fieldListState.readError && !fieldListState.session}
        <p>No field-list authority has been loaded. After all initial read dispatches and validation settle,
          explicitly acknowledge this failed read to release its close/navigation hold. This does not enable
          field editing or assume defaults; a fresh successful Load is mandatory. Established drafts and Save receipts cannot be dismissed here.</p>
        <button data-summary-field-list-bootstrap-dismiss onclick={dismissInitialFieldListReadError}
          disabled={!fieldListState.canDismissInitialReadError}>Acknowledge failed initial environmental field read</button>
      {/if}
      {#if fieldListState.bootstrapPending && !fieldListState.readBusy}
        <p role="status">Initial environmental field read dispatch/validation is still settling.
          Acknowledgement and fresh Load must wait; no stale response will become authority.</p>
      {/if}
      {#if !presentationView?.saved || presentationInvalid}
        <p role="status">Environmental field mutation unavailable: explicitly Load valid reviewed presentation authority. Load and recovery remain available when peers are not held.</p>
      {/if}
      {#if fieldListState.session && fieldListState.scope}
        {#key fieldListState.session}
          <SummaryVegetationFieldList scope={fieldListState.scope} session={fieldListState.session}
            enabledFromPresentationDraft={fieldListEnabledFromPresentation}
            peerDisabled={fieldListPeerDisabled} peerValidation={fieldListPeerValidation}
            onStateChange={() => { reportBusy(); }} />
        {/key}
      {:else}
        <p>Explicit Load learns the exact owned VPro64 source path; no default path or physical selection is assumed.</p>
        <button data-summary-field-list-bootstrap-load onclick={loadFieldList}
          disabled={fieldListPeerDisabled || !!fieldListPeerValidation || fieldListState.readBusy || fieldListState.bootstrapPending}>Load environmental fields</button>
      {/if}
      {#if fieldListState.readBusy}
        <p role="status">Reading complete owned field-list authority…</p>
        <button data-summary-field-list-bootstrap-cancel onclick={() => fieldList.cancel(ownedContext())}>Cancel environmental field read</button>
      {/if}
    </section>
  {/if}
  {#if environmentalError}<p role="alert">{environmentalError}</p>{/if}
  {#if environmentalSession}
    <SummaryVegetationEnvironmental session={environmentalSession} port={summaryEnvironmentalPort} numeric={summaryEnvironmentalNumericEnabled} lifeform={summaryEnvironmentalLifeformEnabled}
      operationHeld={operationHeldProp || workbookHeld || unavailableScope || viewState.busy || fieldListHeld ||
        preferenceHeld || preferenceInvalid || !!viewState.draftError || qualityMutationHeld || qualityUnavailable ||
        presentationMutationHeld || presentationInvalid}
      onBusyChange={() => { environmentalHeld = summaryEnvironmentalDesktop.activity().busy; reportBusy(); }} />
  {/if}
  {#if import.meta.env.VITE_SUMMARY_VEGETATION_WORKBOOK === 'true'}
    <SummaryVegetationWorkbookPanel owner={{ contextId, project, projectPath, su, suPath }}
      calculation={{ average: viewState.average, display: viewState.display, showClass: viewState.showClass,
        ...(groupedWorkbookEnabled && viewState.groupAndOrder !== 11 ? { groupAndOrder: viewState.groupAndOrder } : {}) }}
      disabled={operationHeldProp || environmentalHeld || unavailableScope || viewState.groupAndOrder !== 11 && !groupedWorkbookEnabled ||
        viewState.busy || fieldListHeld || preferenceHeld || preferenceInvalid || !!viewState.draftError ||
        qualityMutationHeld || qualityUnavailable || presentationMutationHeld || presentationInvalid}
      onBusyChange={(held) => { workbookHeld = held; reportBusy(); }} />
  {/if}
  <div class="actions">
    <button onclick={() => session.show(port)} disabled={!enabled || unavailableScope || viewState.busy || operationHeld || fieldListHeld || preferenceHeld || preferenceInvalid ||
      qualityMutationHeld || qualityUnavailable || presentationMutationHeld || presentationInvalid || !!viewState.draftError || !!preferences && !preferenceView?.initialized} data-summary-preview>
      Preview Summary Vegetation
    </button>
    {#if viewState.busy}<button onclick={() => session.cancel(port)} data-summary-cancel>Cancel preview</button><p role="status">Reading owned vegetation and Site Unit Table…</p>{/if}
  </div>
  <p>Preview uses retained drafts{preferences ? '; preferences change only through explicit Save' : ', not saved preferences'}. Original Access Load/View Report setters are not run.
    Stored presentation drafts are separate and are not consumed by this raw crosstab preview.
    Unlumped, no quality filtering, English/guide names, environment headers or constancy.
    Strata and Lifeform Presence Class and Significance Class remain unavailable because the original grouped query has a criteria type mismatch.
    Other groupings, report units, original formatted workbook output and printing are unavailable.
    A separately gated draft Layer inspection workbook is distinct from that Report action.</p>
  <p>The source “% Presence” display uses Count/nPlots ratios (0.75), not ratio × 100. Cover cells preserve source formatted text and NULL blanks.
    Profile navigation does not filter the current physical Site Unit Table.</p>
  {#if viewState.preview && !qualityUnavailable}
    <article data-summary-result>
      <h2>Current Site Unit Table — {viewState.preview.groupAndOrder === 12 ? 'Strata' : viewState.preview.groupAndOrder === 13 ? 'Lifeform' : viewState.preview.groupAndOrder === 14 ? 'None' : 'Layer'}</h2>
      <p>Calculate: {viewState.preview.average === 1 ? 'By n Plots' : 'Characteristic'}.
        Display Values: {viewState.preview.display === 1 ? '% Presence and % Cover' : viewState.preview.display === 2 ? 'Presence Class and Significance Class' : viewState.preview.display === 3 ? 'Presence Class and % Cover' : viewState.preview.display === 4 ? 'Prominence Class' : viewState.preview.display === 5 ? 'Goldstream Class' : '% Cover'}.
        {#if viewState.preview.showClass}Show: {viewState.preview.showClass === 20 ? 'Value' : viewState.preview.showClass === 1 ? 'Class (historical default 1)' : 'Class'}.
          Calculate is retained but ignored by these original class formulas.
        {/if}</p>
      <div class="table-scroll">
        <table>
          <caption>Summary Vegetation — {project}, {su}</caption>
          <thead>
            <tr><th scope="col">{viewState.preview.groupField?.name ?? 'Layer'}</th><th scope="col">Spp</th><th scope="col">Order</th>{#each viewState.preview.columns as unit}<th scope="col">{unit || '(empty unit)'}</th>{/each}</tr>
            <tr><th scope="row" colspan="3">n Plots</th>{#each viewState.preview.columns as code}<td>{viewState.preview.units.find(unit => unit.code === code)?.numPlots}</td>{/each}</tr>
          </thead>
          <tbody>{#each viewState.preview.rows as row}<tr>
            <td>{row.layer.text ?? `(NULL ${viewState.preview.groupField?.name ?? 'Layer'})`}</td><th scope="row">{row.species.text ?? '(NULL Spp)'}</th><td>{row.order.storage === 'real' ? row.order.real : row.order.text ?? ''}</td>
            {#each row.covers as cover, i}<td aria-label={`${row.species.text ?? 'NULL Spp'} — ${viewState.preview.columns[i] || 'empty unit'}`}>{cover.text ?? ''}</td>{/each}
          </tr>{/each}</tbody>
        </table>
      </div>
      {#if viewState.preview.rows.length === 0}<p>No final species rows. Admitted units, NULL cells and absent pivot columns are not interchangeable; no workbook was created.</p>{/if}
      <details>
        <summary>Physical admission and calculation provenance</summary>
        {#each viewState.preview.units as unit}
          <p>Unit {unit.code || '(empty)'}: {unit.numPlots} non-NULL PlotNumber memberships.
            {unit.membershipIds.length} physical memberships; {unit.rawVegRowIds.length} raw vegetation admission witnesses.
            {viewState.preview.columns.includes(unit.code) ? 'Present in pivot columns.' : 'No staged pivot column.'}</p>
        {/each}
        {#each viewState.preview.rows as row}
          <p>{row.layer.text ?? 'NULL Layer'} — {row.species.text ?? 'NULL Spp'}:
            {row.witnesses.length} weighted stage/membership/reference observations.</p>
        {/each}
      </details>
    </article>
  {/if}
</section>

<style>
  .summary-vegetation { display: grid; grid-template-columns: minmax(0, 1fr); gap: 1rem; padding: 1rem; }
  .options { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 18rem), 1fr)); gap: 1rem; }
  label { display: flex; flex-direction: column; gap: .35rem; min-width: 0; }
  select { box-sizing: border-box; width: 100%; min-width: 0; padding: .5rem; }
  .owner { overflow-wrap: anywhere; }
  .linked-field-list { display: grid; grid-template-columns: minmax(0, 1fr); gap: .75rem; min-width: 0; overflow-wrap: anywhere; }
  .actions { display: flex; flex-wrap: wrap; gap: .75rem; align-items: center; }
  button, summary { min-height: 2.5rem; padding: .5rem .75rem; }
  .error { color: var(--color-red-700, #b91c1c); }
  .table-scroll { overflow-x: auto; max-width: 100%; }
  table { border-collapse: collapse; width: 100%; }
  th, td { padding: .5rem; border: 1px solid #d1d5db; text-align: left; vertical-align: top; }
  article, details { min-width: 0; }
</style>
