<script lang="ts">
  import { onMount, onDestroy, untrack } from 'svelte';
  import { ContextService, LongEnvironmentPreferencesService, EnvironmentWorkbookService, type LongEnvironmentPreview } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { reportTitleError, reportCellText, validateLongEnvironmentPreview } from './longEnvironmentReport';
  import { longEnvironmentPreferencesSession } from './longEnvironmentPreferences';
  import { environmentWorkbookPublicationSession, validateEnvironmentWorkbookReview, type ValidatedEnvironmentWorkbookReview } from './environmentWorkbook';

  let { contextId, project, projectPath, su, suPath, onBusyChange }: {
    contextId: string; project: string; projectPath: string; su: string; suPath: string;
    onBusyChange: (busy: boolean) => void;
  } = $props();
  let title = $state('');
  let ready = $state(false);
  let busy = $state(false);
  let error = $state('');
  let preview = $state<LongEnvironmentPreview | null>(null);
  let workbookReview = $state.raw<ValidatedEnvironmentWorkbookReview | null>(null);
  let destination = $state('');
  const titleError = $derived(reportTitleError(title));
  const reads = new ReadRequests();
  let generation = 0;
  const preferencesEnabled = import.meta.env.VITE_LONG_ENVIRONMENT_PREFERENCES === 'true';
  const workbookEnabled = import.meta.env.VITE_LONG_ENVIRONMENT_WORKBOOK === 'true';
  const workbookOwner = untrack(() => ({ contextId, project, projectPath, su, suPath }));
  const workbookSession = environmentWorkbookPublicationSession(workbookOwner);
  let publication = $state(workbookSession.view());
  const workbookBarrier = $derived(publication.busy || publication.blocked);
  const ownedPreferences = () => longEnvironmentPreferencesSession({ contextId, project, projectPath, su, suPath });
  const preferences = preferencesEnabled ? ownedPreferences() : null;
  let preferenceView = $state(preferences?.view() ?? null);
  const preferenceBarrier = $derived(!!preferenceView && (preferenceView.busy || preferenceView.blocked ||
    preferenceView.authorityUnknown || !!preferenceView.draftError));
  function reportBusy() { onBusyChange(busy || preferenceBarrier || workbookBarrier); }
  const unsubscribeWorkbook = workbookSession.subscribe(() => {
    publication = workbookSession.view();
    reportBusy();
  });
  const unsubscribe = preferences?.subscribe(() => {
    preferenceView = preferences.view(); title = preferenceView.draft.title; ready = preferenceView.initialized; preview = null; workbookReview = null;
    onBusyChange(busy || preferenceView.busy || preferenceView.blocked || preferenceView.authorityUnknown || !!preferenceView.draftError || workbookBarrier);
  });

  function cancel() {
    generation++; reads.cancelAll(); preferences?.cancelLoad(); busy = false; reportBusy();
  }
  async function options() {
    if (workbookBarrier) return;
    cancel(); const request = generation; ready = false; error = ''; preview = null; workbookReview = null;
    busy = true; reportBusy();
    try {
      const value = await reads.track(ContextService.GetLongEnvironmentOptions(contextId));
      if (request !== generation) return;
      if (value.contextId !== contextId || typeof value.title !== 'string' || reportTitleError(value.title)) {
        throw new Error('Configured report title is incomplete or belongs to another context.');
      }
      if (!preferencesEnabled) title = value.title;
      ready = true;
    } catch (cause) {
      if (request === generation) error = `Report options unavailable: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; reportBusy(); }
    }
  }
  async function show() {
    if (busy || preferenceBarrier || workbookBarrier || !ready || titleError || su === 'None') return;
    cancel(); const request = generation; const requestedTitle = title; error = ''; preview = null; workbookReview = null;
    busy = true; reportBusy();
    try {
      const value = await reads.track(ContextService.PreviewLongEnvironment(contextId, { title: requestedTitle }));
      if (request !== generation) return;
      preview = validateLongEnvironmentPreview(value, contextId, project, projectPath, su, suPath, requestedTitle);
    } catch (cause) {
      if (request === generation) error = `Report unavailable; this report preparation made no data or configuration writes: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; reportBusy(); }
    }
  }
  async function loadSavedTitle() {
    if (!preferences || preferenceView?.busy || preferenceView?.blocked || workbookBarrier) return;
    cancel(); const request = generation; const token = preferences.beginLoad(); error = ''; busy = true; reportBusy();
    try {
      const value = await reads.track(LongEnvironmentPreferencesService.GetLongEnvironmentPreferences(contextId));
      if (request !== generation) return;
      preferences.applyLoaded(value, token);
    } catch (cause) {
      if (request === generation) error = `Saved title unavailable; no defaults applied: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; reportBusy(); }
    }
  }
  async function saveTitle() {
    if (!preferences || busy || preferenceView?.busy || preferenceView?.blocked || workbookBarrier) return;
    cancel(); error = '';
    try {
      await preferences.save(request => LongEnvironmentPreferencesService.SaveLongEnvironmentPreferences(contextId, JSON.stringify(request)));
    } catch (cause) { error = String(cause); }
  }
  function editTitle(value: string) {
    if (preferences) { cancel(); preferences.edit(value); }
    else title = value;
    preview = null; workbookReview = null; if (!reportTitleError(title)) error = '';
  }
  async function reviewWorkbook() {
    if (!workbookEnabled || busy || preferenceBarrier || workbookBarrier || !ready || titleError || su === 'None' || su === 'USysSuTableDynamic') return;
    cancel(); const request = generation, requestedTitle = title;
    error = ''; preview = null; workbookReview = null; busy = true; reportBusy();
    try {
      const value = await reads.track(EnvironmentWorkbookService.GetReview(contextId, JSON.stringify({ title: requestedTitle })));
      if (request !== generation) return;
      workbookReview = validateEnvironmentWorkbookReview(value, workbookOwner, requestedTitle);
      preview = workbookReview.preview;
    } catch (cause) {
      if (request === generation) error = `Workbook review unavailable; no file published: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; reportBusy(); }
    }
  }
  async function publishWorkbook() {
    if (!workbookEnabled || busy || preferenceBarrier || workbookBarrier || !workbookReview) return;
    cancel(); error = '';
    try {
      await workbookSession.publish(workbookReview, destination, request =>
        EnvironmentWorkbookService.ExportReviewed(contextId, JSON.stringify(request)));
    } catch (cause) { error = String(cause); }
  }
  function acknowledgeWorkbook() {
    workbookSession.acknowledge();
    workbookReview = null;
  }
  onMount(() => { if (!preferencesEnabled) void options(); });
  onDestroy(() => { unsubscribe?.(); unsubscribeWorkbook(); cancel(); });
</script>

<section class="report" data-long-environment-report aria-label="Long Environment report">
  <h1>Long Environment</h1>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  {#if titleError}<p class="error" role="alert">{titleError}</p>{/if}
  {#if preferenceView?.draftError}<p class="error" role="alert">{preferenceView.draftError}</p>{/if}
  {#if preferenceView?.error}<p class="error" role="alert">{preferenceView.error}</p>{/if}
  {#if publication.error}<p class="error" role="alert">{publication.error}</p>{/if}
  {#if publication.busy}<p role="status">Publishing the reviewed workbook; publication cannot be cancelled.</p>{/if}
  {#if publication.outcome}
    <p data-environment-workbook-receipt role="status">Workbook receipt: {publication.outcome.status}.
      Requested {publication.requestedDestination}; observed {publication.outcome.path || '(no committed file)'}.
      SHA256 {publication.outcome.sha256 || '(none)'}.</p>
  {:else if publication.blocked && !publication.busy}
    <p data-environment-workbook-unknown role="status">Workbook outcome unknown. Retain and inspect {publication.requestedDestination}; never replay blindly.</p>
  {/if}
  {#if publication.blocked && !publication.busy}
    <button onclick={acknowledgeWorkbook}>Acknowledge workbook outcome</button>
  {/if}
  <p>Project {project}; selected SU {su}. Whole selected-SU scope; profile navigation does not filter this report.</p>
  <label for="long-environment-title">Title</label>
  {#if preferencesEnabled}
    <textarea id="long-environment-title" data-source-control="rptSvTitle" rows="2" value={title}
      disabled={!!preferenceView?.busy || !!preferenceView?.blocked || workbookBarrier}
      oninput={(event) => { const value = event.currentTarget.value; editTitle(value); }}></textarea>
  {:else}
    <input id="long-environment-title" data-source-control="rptSvTitle" value={title} disabled={busy || !ready || workbookBarrier}
      oninput={(event) => { const value = event.currentTarget.value; editTitle(value); }} />
  {/if}
  <div class="actions">
    <button data-source-control="btnViewReport" disabled={busy || preferenceBarrier || workbookBarrier || !ready || !!titleError || su === 'None'} onclick={show}>View Report</button>
    <button disabled={!workbookEnabled || busy || preferenceBarrier || workbookBarrier || !ready || !!titleError || su === 'None' || su === 'USysSuTableDynamic'} onclick={reviewWorkbook}>Review XLSX workbook (no file yet)</button>
    {#if preferencesEnabled}
      <button data-long-environment-preferences-load disabled={!!preferenceView?.busy || !!preferenceView?.blocked || workbookBarrier} onclick={loadSavedTitle}>Load saved title</button>
      <button data-long-environment-preferences-save disabled={busy || !!preferenceView?.busy || !!preferenceView?.blocked || !!preferenceView?.draftError || !preferenceView?.saved || workbookBarrier}
        onclick={saveTitle}>Save reviewed title</button>
      <button data-long-environment-preferences-undo disabled={!!preferenceView?.busy || !!preferenceView?.blocked || !preferenceView?.saved || workbookBarrier} onclick={() => { cancel(); preferences?.undo(); }}>Undo title</button>
      {#if preferenceView?.blocked}
        <button data-long-environment-preferences-acknowledge disabled={preferenceView.busy} onclick={() => preferences?.acknowledge()}>Acknowledge preference outcome</button>
      {/if}
    {:else}
      <button disabled={busy || workbookBarrier} onclick={options}>Reload report options</button>
    {/if}
    {#if busy}<button onclick={cancel}>Cancel report read</button><p role="status">Reading owned report snapshots…</p>{/if}
  </div>
  {#if workbookReview}
    <p>Reviewed workbook: {workbookReview.sheets.length} source unit sheets; {workbookReview.bytes} bytes;
      SHA256 {workbookReview.workbookSHA256}. Hidden lossless source metadata preserves typed NULL/empty values and diagnostics.</p>
    <ul>{#each workbookReview.sheets as sheet}<li>Unit “{sheet.unit}” → worksheet “{sheet.name}”</li>{/each}</ul>
    <label for="environment-workbook-destination">New XLSX output file (existing files are never replaced)</label>
    <textarea id="environment-workbook-destination" rows="2" disabled={busy || preferenceBarrier || workbookBarrier} bind:value={destination}></textarea>
    <button disabled={busy || preferenceBarrier || workbookBarrier || !destination || !/\.xlsx$/i.test(destination)} onclick={publishWorkbook}>Publish reviewed XLSX workbook</button>
  {/if}
  {#if preferenceView?.busy}<p role="status">Saving reviewed title; this preference write cannot be cancelled.</p>{/if}
  {#each preferenceView?.receipts ?? [] as receipt}
    {#if receipt.kind === 'known'}
      <p data-long-environment-preferences-receipt role="status">Preference receipt: changed {String(receipt.outcome.changed)};
        committed {String(receipt.outcome.committed)}. {receipt.outcome.errorMessage}</p>
    {:else}
      <p data-long-environment-preferences-receipt data-long-environment-preferences-unknown-receipt role="status">
        Preference receipt: outcome unknown; changed unknown; committed unknown. {receipt.errorMessage}</p>
    {/if}
  {/each}
  {#if preferencesEnabled}<p class="guidance">Load saved title explicitly, review the literal draft, then Save or Undo.
    Save changes only ReportOptions.LEReportTitle in YAML. Input and report preparation never save preferences.</p>{/if}
  {#if su === 'None'}<p>Select an SU in Projects &amp; context before viewing the report.</p>{/if}
  {#if preview}
    <h2>{preview.report.title}</h2>
    <p role="status">{preview.report.units?.length} units; 72 ordered rows. This report preparation made no data/configuration writes.</p>
    {#each preview.report.diagnostics ?? [] as diagnostic}
      <p class="diagnostic">{diagnostic.code}: unit {diagnostic.unit === null ? 'not applicable' : JSON.stringify(diagnostic.unit)};
        plot {diagnostic.plotNumber === null ? 'not applicable' : JSON.stringify(diagnostic.plotNumber)}; count {diagnostic.count}.</p>
    {/each}
    {#each preview.report.units ?? [] as unit (unit.code)}
      <section class="unit">
        <h3>Site Unit — {unit.code === '' ? '"" (empty code)' : unit.code}</h3>
        <p>Long name: {unit.longName === null ? unit.nameStatus : unit.longName === '' ? '"" (empty name)' : unit.longName}</p>
        <details><summary>Reference name candidates ({unit.nameCandidates?.length})</summary>
          {#each unit.nameCandidates ?? [] as candidate}
            <p>Reference row {candidate.rowId}: {reportCellText(candidate.value)}</p>
          {/each}
        </details>
        <div class="table-scroll" role="region" aria-label={`Environmental table for unit ${unit.code}`}>
          <table>
            <caption>Environment Table — Site Unit {JSON.stringify(unit.code)}</caption>
            <thead><tr><th scope="col">Field</th>{#each unit.plots ?? [] as plot}<th scope="col">{JSON.stringify(plot.plotNumber)}<br />{plot.status}</th>{/each}</tr></thead>
            <tbody>{#each preview.report.fields ?? [] as field, i}
              <tr class:heading={field.heading}><th scope="row">{field.label}</th>
                {#each unit.plots ?? [] as plot}<td>{field.heading ? '' : reportCellText(plot.values![i])}</td>{/each}
              </tr>
            {/each}</tbody>
          </table>
        </div>
      </section>
    {/each}
  {/if}
  <p class="guidance">Read-only experimental preview.
    {#if !preferencesEnabled}Title starts from ReportOptions.LEReportTitle in YAML; edits affect only this preview, not saved preferences.{/if}
    NULL, empty text and historical storage remain distinct. Missing rows and conflicting names are shown, not repaired.
    No quality filtering, summaries or Excel automation is implemented.
    Workbook review and explicit no-replace XLSX publication require their separate gate; report preparation never publishes a file.</p>
</section>

<style>
  .report { display: grid; grid-template-columns: minmax(0, 1fr); min-width: 0; overflow-wrap: anywhere; gap: .75rem; }
  h1 { font-size: 1.5rem; } h2 { font-size: 1.25rem; } h3 { font-weight: 600; }
  input, textarea { box-sizing: border-box; min-width: 0; border: 1px solid #cbd5e1; border-radius: .4rem; padding: .6rem; width: 100%; }
  .actions { display: flex; flex-wrap: wrap; gap: .5rem; align-items: center; }
  button { border: 1px solid #cbd5e1; border-radius: .4rem; padding: .5rem .75rem; background: white; }
  button:disabled { opacity: .5; }
  .unit { min-width: 0; display: grid; gap: .5rem; }
  .table-scroll { overflow-x: auto; max-width: 100%; border: 1px solid #cbd5e1; }
  table { border-collapse: collapse; width: 100%; }
  th, td { border: 1px solid #e2e8f0; padding: .5rem; text-align: left; min-width: 9rem; white-space: pre-wrap; overflow-wrap: anywhere; }
  th[scope="row"] { min-width: 16rem; } .heading { background: #e7f1ec; }
  .error { color: #b91c1c; } .diagnostic { color: #92400e; } .guidance { color: #57534e; font-size: .875rem; }
</style>
