<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { ContextService, type LongVegetationSettings } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { reportCellText } from './longEnvironmentReport';
  import { validateLongVegetationOptions, validateLongVegetationPreview, type ValidatedLongVegetationPreview } from './longVegetationReport';

  let { contextId, project, projectPath, su, suPath, onBusyChange }: {
    contextId: string; project: string; projectPath: string; su: string; suPath: string;
    onBusyChange: (busy: boolean) => void;
  } = $props();
  let settings = $state<LongVegetationSettings | null>(null);
  let busy = $state(false);
  let error = $state('');
  let preview = $state<ValidatedLongVegetationPreview | null>(null);
  const reads = new ReadRequests();
  let generation = 0;

  function cancel() {
    generation++; reads.cancelAll(); busy = false; onBusyChange(false);
  }
  async function options() {
    cancel(); const request = generation; settings = null; error = ''; preview = null;
    busy = true; onBusyChange(true);
    try {
      const value = await reads.track(ContextService.GetLongVegetationOptions(contextId));
      if (request !== generation) return;
      settings = validateLongVegetationOptions(value, contextId, project, projectPath, su, suPath).settings;
    } catch (cause) {
      if (request === generation) error = `Report options unavailable: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  async function show() {
    if (busy || !settings || su === 'None') return;
    cancel(); const request = generation; error = ''; preview = null;
    busy = true; onBusyChange(true);
    try {
      const value = await reads.track(ContextService.PreviewLongVegetation(contextId));
      if (request !== generation) return;
      preview = validateLongVegetationPreview(value, contextId, project, projectPath, su, suPath);
      settings = preview.settings;
    } catch (cause) {
      if (request === generation) error = `Report unavailable; no data or configuration written: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  const numberText = (value: number | null) => value === null ? 'NULL' : String(value);
  const presenceText = (value: number | null) => value === null ? 'NULL' : `${value * 100}%`;
  onMount(() => { void options(); });
  onDestroy(cancel);
</script>

<section class="report" data-long-vegetation-report aria-label="Long Vegetation report">
  <h1>Long Vegetation</h1>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <p>Project {project}; selected SU {su}. Whole selected-SU scope; the current plot and profile navigation do not filter this report.</p>
  <dl class="scope">
    <div><dt>Project database</dt><dd>{projectPath}</dd></div>
    <div><dt>Selected SU database</dt><dd>{suPath || 'No selected SU'}</dd></div>
  </dl>
  {#if settings}
    <dl class="settings" aria-label="Read-only report options">
      <div><dt>Report Title</dt><dd data-source-control="rptLvTitle">{settings.title === '' ? '"" (empty title)' : settings.title}</dd></div>
      <div><dt>Group by</dt><dd>Layer</dd></div>
      <div><dt>Average</dt><dd>{settings.average === 'all-plots' ? (settings.quality ? 'By n Plots — joined sum / qualified SU denominator' : 'By n Plots — joined sum / physical SU denominator') : 'Characteristic — mean of joined non-NULL observations'}</dd></div>
      <div><dt>Constant species list</dt><dd>{settings.constantSpeciesList ? 'On — same selected-SU list; thresholds bypassed' : 'Off — strict presence and mean-cover thresholds apply'}</dd></div>
      <div><dt>Show species with presence &gt; than</dt><dd>{settings.presenceGreaterThan}%{settings.constantSpeciesList ? ' (not applied)' : ''}</dd></div>
      <div><dt>Show species with mean cover &gt; than</dt><dd>{settings.meanCoverGreaterThan}%{settings.constantSpeciesList ? ' (not applied)' : ''}</dd></div>
      <div><dt>Order by</dt><dd>{settings.constantSpeciesList ? 'Layer, then species (constant-list source order)' : settings.order === 'presence' ? 'Layer, then descending presence' : 'Layer, then species'}</dd></div>
      <div><dt>English names</dt><dd>{settings.showEnglishName ? 'List and matched report names shown separately' : 'Not shown'}</dd></div>
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
    <button data-source-control="btnViewReport" disabled={busy || !settings || su === 'None'} onclick={show}>View Report</button>
    <button disabled={busy} onclick={options}>Reload report options</button>
    {#if busy}<button onclick={cancel}>Cancel report read</button><p role="status">Reading owned report snapshots...</p>{/if}
  </div>
  {#if su === 'None'}<p>Select an SU in Projects &amp; context before viewing the report.</p>{/if}
  {#if settings?.constantSpeciesList}<p class="notice">Constant species list is enabled in retained YAML. Presence and mean-cover thresholds are bypassed, as in the source report; missing combinations have NULL statistics, not zero.</p>{/if}
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
          <p>No layer/species rows satisfy the current report settings.</p>
        {:else}
          <div class="table-scroll" role="region" aria-label={`Vegetation table for unit ${reportCellText(unit.code)}`}>
            <table>
              <caption>Vegetation Table — Site Unit {reportCellText(unit.code)}</caption>
              <thead><tr>
                <th scope="col">Layer</th><th scope="col">Species</th>
                {#if preview.settings.showEnglishName}<th scope="col">List English name</th><th scope="col">Matched report English name</th>{/if}
                <th scope="col">Presence (%)</th><th scope="col">Mean cover (%)</th>
                {#each unit.rows[0].plots as plot}<th scope="col">Plot {JSON.stringify(plot.plotNumber)}</th>{/each}
              </tr></thead>
              <tbody>{#each unit.rows as row}
                <tr>
                  <td>{reportCellText(row.layer)}</td><th scope="row">{reportCellText(row.species)}</th>
                  {#if preview.settings.showEnglishName}<td>{reportCellText(row.englishName)}</td><td>{reportCellText(row.matchedName)}</td>{/if}
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
  <p class="guidance">Read-only experimental layer preview. Options are loaded from YAML and cannot be edited here.
    NULL, empty text, zero and absent statistics remain distinct. Source reference/membership multiplicities and English-name fanout are reported, not deduplicated.
    Unit-name candidates preserve original reference rows; conflicting or unsupported names are not chosen arbitrarily.
    No strata/lifeform grouping, taxon lumping, summaries, title persistence, Excel automation or file export is implemented.</p>
</section>

<style>
  .report { display: grid; gap: .75rem; min-width: 0; }
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
