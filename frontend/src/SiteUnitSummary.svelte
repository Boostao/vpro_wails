<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { ContextService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { validateSiteUnitSummary, validateSiteUnitSummaryOptions, type ValidatedSiteUnitSummary } from './siteUnitSummary';
  import { reportCellText } from './longEnvironmentReport';

  let { contextId, project, projectPath, su, suPath, onBusyChange }: {
    contextId: string; project: string; projectPath: string; su: string; suPath: string;
    onBusyChange: (busy: boolean) => void;
  } = $props();
  let method = $state<number | null>(null);
  let siteUnitType = $state<number | null>(null);
  let ready = $state(false);
  let busy = $state(false);
  let error = $state('');
  let preview = $state<ValidatedSiteUnitSummary | null>(null);
  const reads = new ReadRequests();
  let generation = 0;
  function cancel() { generation++; reads.cancelAll(); busy = false; onBusyChange(false); }
  async function options() {
    cancel(); const request = generation;
    ready = false; error = ''; preview = null; busy = true; onBusyChange(true);
    try {
      const value = await reads.track(ContextService.GetSiteUnitSummaryOptions(contextId));
      if (request !== generation) return;
      const configured = validateSiteUnitSummaryOptions(value, contextId, project, projectPath, su, suPath);
      method = configured.method; siteUnitType = configured.siteUnitType;
      ready = siteUnitType === 1;
      if (!ready) error = `Saved site-unit type ${siteUnitType} is unavailable in this normal-SU preview; no saved options were reset.`;
    } catch (cause) {
      if (request === generation) error = `Summary options unavailable; no defaults or configuration writes: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  async function show() {
    if (busy || !ready || su === 'None' || (method !== 1 && method !== 2)) return;
    cancel(); const request = generation; const requestedMethod = method;
    error = ''; preview = null; busy = true; onBusyChange(true);
    try {
      const value = await reads.track(ContextService.PreviewSiteUnitSummary(contextId, { method: requestedMethod }));
      if (request !== generation) return;
      preview = validateSiteUnitSummary(value, contextId, project, projectPath, su, suPath, requestedMethod);
    } catch (cause) {
      if (request === generation) error = `Summary unavailable; no data or configuration writes: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  onMount(() => { void options(); });
  onDestroy(cancel);
</script>

<section class="summary" data-site-unit-summary aria-label="Summary Environment">
  <h1>Summary Environment</h1>
  {#if error}<p role="alert" class="error">{error}</p>{/if}
  <p>Project {project}; selected normal SU {su}. Profile navigation does not filter this report.</p>
  <fieldset disabled={busy || !ready} data-source-control="optValueMethod">
    <legend>Quantitative Values</legend>
    <label><input type="radio" name="summary-method" value={1} bind:group={method} onchange={() => { preview = null; error = ''; }} />Mean</label>
    <label><input type="radio" name="summary-method" value={2} bind:group={method} onchange={() => { preview = null; error = ''; }} />Interquartile (25% - 50% - 75%)</label>
  </fieldset>
  <p data-source-control="optSiteUnitType">Saved site-unit type:
    {siteUnitType === 1 ? 'Normal SU' : siteUnitType === 2 ? 'Hierarchy (unavailable)' :
      siteUnitType === 3 ? 'Field-derived units (unavailable)' : 'not loaded'}.</p>
  <div class="actions">
    <button data-source-control="btnCreateReport" disabled={busy || !ready || su === 'None'} onclick={show}>Create Report</button>
    <button data-summary-options-reload disabled={busy} onclick={options}>Reload summary options</button>
    {#if busy}<button data-summary-cancel onclick={cancel}>Cancel report read</button><p role="status">Reading owned summary options or snapshots...</p>{/if}
  </div>
  <p class="guidance">Read-only normal-SU report with layer covers, without a species summary. Saved calculation method and site-unit type load from configuration.
    Calculation changes are preview-only; Reload restores saved options. No registry or saved-preference changes.
    Hierarchy/field-derived units, lifeform covers, species summaries and Excel/file publication are unavailable.</p>
  <p class="guidance">Counts preserve every physical SU/Env/Admin join, including duplicate weights. Text equality/order is literal.
    Numeric formatting uses the measured English locale; unsupported historical text in numeric fields is refused, not repaired.
    Both disturbance labels retain the source's SiteDisturbance2 binding.</p>
  {#if preview}
    <p role="status" data-summary-count>{preview.report.units.length} units; {preview.report.memberships.length} physical memberships.
      {preview.report.method === 1 ? 'Min---Mean---Max' : 'Interquartile 25% - 50% - 75%'}</p>
    {#if preview.report.units.length === 0}<p>No joined plots in the selected units.</p>{/if}
    {#each preview.report.units as unit, unitIndex (unit.code)}
      <article data-summary-unit>
        <h2>{unit.code === '' ? '"" (empty unit)' : unit.code}</h2>
        <p>{unit.longName === '' ? '"" (empty name)' : unit.longName ?? `Name ${unit.nameStatus}`}</p>
        <p>Plots in unit: {unit.plots.length} (physical joined rows)</p>
        {#each ['SITE', 'VEGETATION', 'SOILS'] as section}
          <section aria-labelledby={`summary-${unitIndex}-${section}`}>
            <h3 id={`summary-${unitIndex}-${section}`}>{section}</h3>
            <dl>
              {#each preview.report.fields as field, index}
                {#if field.section === section}
                  <div><dt>{field.label}</dt><dd data-summary-value>{unit.values[index] === '' ? '"" (empty summary)' : unit.values[index]}</dd></div>
                {/if}
              {/each}
            </dl>
          </section>
        {/each}
        <details>
          <summary>Physical plot joins and reference name candidates</summary>
          <ul>{#each unit.plots as plot}<li>{plot.plotNumber}: SU row {plot.suRowId}, Env row {plot.envRowId}, Admin row {plot.adminRowId}</li>{/each}</ul>
          <ul>{#each unit.nameCandidates as candidate}<li>Reference row {candidate.rowId}: {reportCellText(candidate.value)}</li>{/each}</ul>
        </details>
      </article>
    {/each}
    <details>
      <summary>All physical memberships and exclusions</summary>
      <ul>{#each preview.report.memberships as membership}
        <li>SU row {membership.rowId}: plot {reportCellText(membership.plotNumber)}, unit {reportCellText(membership.siteUnit)};
          {membership.status}, {membership.joinedRows} joined rows</li>
      {/each}</ul>
    </details>
  {/if}
</section>

<style>
  .summary { padding: 1rem; max-width: 90rem; margin: auto; }
  h1, h2, h3 { font-weight: 600; margin: .8rem 0; }
  h1 { font-size: 1.5rem; } h2 { font-size: 1.2rem; }
  fieldset, .actions { display: flex; flex-wrap: wrap; gap: 1rem; margin: 1rem 0; }
  label { display: flex; align-items: center; gap: .5rem; }
  button { border: 1px solid #94a3b8; border-radius: .25rem; padding: .5rem .8rem; }
  button:disabled { opacity: .5; }
  article { border: 1px solid #cbd5e1; border-radius: .4rem; padding: 1rem; margin-top: 1rem; }
  dl { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 24rem), 1fr)); gap: .5rem 1rem; }
  dl > div { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: .5rem; padding: .4rem; background: #f8fafc; }
  dt { font-weight: 500; } dd, li, h2 { white-space: pre-wrap; overflow-wrap: anywhere; }
  details { margin-top: .8rem; } .guidance { font-size: .9rem; margin: .6rem 0; }
  .error { color: #991b1b; white-space: pre-wrap; }
  @media (max-width: 640px) { dl > div { grid-template-columns: minmax(0, 1fr); } }
</style>
