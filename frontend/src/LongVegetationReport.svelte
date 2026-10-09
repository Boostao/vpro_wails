<script lang="ts">
  import { onMount, onDestroy, untrack } from 'svelte';
  import { ContextService, VegetationWorkbookService, type LongVegetationSettings } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { reportCellText } from './longEnvironmentReport';
  import { requireLongVegetationGrouping, validateLongVegetationOptions, validateLongVegetationPreview, type ValidatedLongVegetationPreview } from './longVegetationReport';
  import { vegetationWorkbookPublicationSession, VegetationWorkbookPublicationSession, type ValidatedVegetationWorkbookReview } from './vegetationWorkbook';

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
  const workbookOwner = untrack(() => ({ contextId, project, projectPath, su, suPath }));
  const workbookSession = workbookEnabled ? vegetationWorkbookPublicationSession(workbookOwner) : new VegetationWorkbookPublicationSession(workbookOwner);
  let publication = $state(workbookSession.view());
  const workbookBarrier = $derived(publication.busy || publication.blocked);
  const groupLabel = $derived(settings?.grouping === 'lifeform' ? 'Lifeform' : settings?.grouping === 'strata' ? 'Strata' : 'Layer');
  function reportBusy() { onBusyChange(busy || workbookBarrier); }
  const unsubscribeWorkbook = workbookSession.subscribe(() => {
    publication = workbookSession.view();
    if (publication.blocked) workbookReview = null;
    reportBusy();
  });

  function cancel() {
    generation++; reads.cancelAll(); busy = false; reportBusy();
  }
  async function options() {
    if (busy || operationHeld || workbookBarrier) return;
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
    if (busy || operationHeld || workbookBarrier || !settings || su === 'None') return;
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
    if (!workbookEnabled || busy || operationHeld || workbookBarrier || !settings || su === 'None' || su === 'USysSuTableDynamic') return;
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
    if (!workbookEnabled || busy || operationHeld || workbookBarrier || !workbookReview) return;
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
  const numberText = (value: number | null) => value === null ? 'NULL' : String(value);
  const presenceText = (value: number | null) => value === null ? 'NULL' : `${value * 100}%`;
  onMount(() => { void options(); });
  onDestroy(() => { unsubscribeWorkbook(); cancel(); });
</script>

<section class="report" data-long-vegetation-report aria-label="Long Vegetation report">
  <h1>Long Vegetation</h1>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
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
    <button data-source-control="btnViewReport" disabled={busy || operationHeld || workbookBarrier || !settings || su === 'None'} onclick={show}>View Report</button>
    <button disabled={busy || operationHeld || workbookBarrier} onclick={options}>Reload report options</button>
    {#if workbookEnabled}
      <button disabled={busy || operationHeld || workbookBarrier || !settings || su === 'None' || su === 'USysSuTableDynamic'} onclick={reviewWorkbook}>Review unlumped XLSX workbook (no file yet)</button>
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
      <textarea id="vegetation-workbook-destination" rows="2" disabled={busy || operationHeld || workbookBarrier} bind:value={destination}></textarea>
      <button disabled={busy || operationHeld || workbookBarrier || !destination || !/\.xlsx$/i.test(destination)} onclick={publishWorkbook}>Publish reviewed unlumped XLSX workbook</button>
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
  <p class="guidance">Read-only experimental vegetation preview. Options are loaded from YAML and cannot be edited here.
    NULL, empty text, zero and absent statistics remain distinct. Source reference/membership multiplicities and English-name fanout are reported, not deduplicated.
    Unit-name candidates preserve original reference rows; conflicting or unsupported names are not chosen arbitrarily.
    Taxon lumping, combined variants, Source4 attributes, title persistence and Excel automation remain unavailable.
    Workbook review and explicit no-replace XLSX publication require their separate default-off gate.</p>
</section>

<style>
  .report { display: grid; grid-template-columns: minmax(0, 1fr); gap: .75rem; min-width: 0; overflow-wrap: anywhere; }
  .workbook { display: grid; grid-template-columns: minmax(0, 1fr); gap: .75rem; min-width: 0; }
  textarea { box-sizing: border-box; min-width: 0; border: 1px solid #cbd5e1; border-radius: .4rem; padding: .6rem; width: 100%; }
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
