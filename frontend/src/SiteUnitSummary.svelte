<script lang="ts">
  import { onMount, onDestroy, untrack } from 'svelte';
  import { ContextService, SiteUnitSummaryPreferencesService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { validateSiteUnitSummary, validateSiteUnitSummaryOptions, type ValidatedSiteUnitSummary, type SiteUnitSummaryMode } from './siteUnitSummary';
  import { reportCellText } from './longEnvironmentReport';
  import { siteUnitSummaryPreferencesSession } from './siteUnitSummaryPreferences';
  import SiteUnitSummaryWorkbookPanel from './SiteUnitSummaryWorkbookPanel.svelte';
  import { summarySpeciesCriteria, validateSummarySpecies, type ValidatedSummarySpecies } from './siteUnitSummarySpecies';
  import { extendedSummaryWorkbookPublicationSession, type ExtendedSummaryWorkbookOptions } from './siteUnitSummaryExtendedWorkbook';

  let { contextId, project, projectPath, su, suPath, onBusyChange }: {
    contextId: string; project: string; projectPath: string; su: string; suPath: string;
    onBusyChange: (busy: boolean) => void;
  } = $props();
  let method = $state<number | null>(null);
  let siteUnitType = $state<number | null>(null);
  let ready = $state(false);
  let busy = $state(false);
  let workbookBusy = $state(false);
  let error = $state('');
  let preview = $state<ValidatedSiteUnitSummary | null>(null);
  const extendedWorkbookEnabled = import.meta.env.VITE_SITE_UNIT_SUMMARY_EXTENDED_WORKBOOK === 'true';
  const extendedAuthority = untrack(() => extendedWorkbookEnabled
    ? extendedSummaryWorkbookPublicationSession({ contextId, project, projectPath, su, suPath }) : null);
  const retainedOptions = extendedAuthority?.view().blocked ? extendedAuthority.requestedOptions() : null;
  let mode = $state<SiteUnitSummaryMode>(retainedOptions?.orderBy === 2 ? 'lifeform' : 'layer');
  const lifeformsEnabled = import.meta.env.VITE_SITE_UNIT_SUMMARY_LIFEFORMS === 'true';
  const speciesEnabled = import.meta.env.VITE_SITE_UNIT_SUMMARY_SPECIES === 'true';
  const ownedCriteria = () => summarySpeciesCriteria({ contextId, project, projectPath, su, suPath });
  const criteria = speciesEnabled ? ownedCriteria() : null;
  let speciesView = $state(criteria?.view() ?? null);
  let speciesPreview = $state<ValidatedSummarySpecies | null>(null);
  const speciesBarrier = $derived(!!speciesView?.error);
  const extendedOptions = $derived.by((): ExtendedSummaryWorkbookOptions | null => {
    if ((method !== 1 && method !== 2) || speciesBarrier) return null;
    if (speciesView?.enabled && criteria) return { ...criteria.request(method, mode), includeSpecies: 1 };
    return mode === 'lifeform' ? { method, orderBy: 2, includeSpecies: 0, coverCalculation: 1,
      andOr: 1, presenceGreaterThan: 0, coverGreaterThan: 0 } : null;
  });
  const reads = new ReadRequests();
  let generation = 0;
  const preferencesEnabled = import.meta.env.VITE_SITE_UNIT_SUMMARY_PREFERENCES === 'true';
  const ownedPreferences = () => siteUnitSummaryPreferencesSession({ contextId, project, projectPath, su, suPath });
  const preferences = preferencesEnabled ? ownedPreferences() : null;
  let preferenceView = $state(preferences?.view() ?? null);
  const preferenceBarrier = $derived(!!preferenceView && (preferenceView.busy || preferenceView.blocked ||
    preferenceView.authorityUnknown || !!preferenceView.draftError));
  function reportBusy() { onBusyChange(busy || preferenceBarrier || workbookBusy || speciesBarrier); }
  const unsubscribeSpecies = criteria?.subscribe(() => {
    speciesView = criteria.view(); preview = null; speciesPreview = null; reportBusy();
  });
  const unsubscribe = preferences?.subscribe(() => {
    preferenceView = preferences.view(); method = preferenceView.draft?.method ?? null;
    siteUnitType = preferenceView.draft?.siteUnitType ?? null;
    ready = (method === 1 || method === 2) && siteUnitType === 1 && !preferenceView.authorityUnknown;
    preview = null; speciesPreview = null;
    reportBusy();
  });
  function cancel() { generation++; reads.cancelAll(); preferences?.cancelLoad(); busy = false; reportBusy(); }
  async function options() {
    if (workbookBusy) return;
    cancel(); const request = generation;
    ready = false; error = ''; preview = null; speciesPreview = null; busy = true; reportBusy();
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
      if (request === generation) { busy = false; reportBusy(); }
    }
  }
  async function show() {
    if (busy || workbookBusy || preferenceBarrier || speciesBarrier || !ready || su === 'None' || (method !== 1 && method !== 2)) return;
    cancel(); const request = generation; const requestedMethod = method; const requestedMode = mode;
    const speciesRequest = speciesView?.enabled ? criteria?.request(requestedMethod, requestedMode) : null;
    error = ''; preview = null; speciesPreview = null; busy = true; reportBusy();
    try {
      if (speciesRequest) {
        const value = await reads.track(ContextService.PreviewSiteUnitSummarySpecies(contextId, speciesRequest));
        if (request !== generation) return;
        speciesPreview = validateSummarySpecies(value, { contextId, project, projectPath, su, suPath }, speciesRequest);
        preview = speciesPreview.environment;
      } else {
        const value = await reads.track(requestedMode === 'lifeform'
          ? ContextService.PreviewSiteUnitSummaryLifeforms(contextId, { method: requestedMethod })
          : ContextService.PreviewSiteUnitSummary(contextId, { method: requestedMethod }));
        if (request !== generation) return;
        preview = validateSiteUnitSummary(value, contextId, project, projectPath, su, suPath, requestedMethod, requestedMode);
      }
    } catch (cause) {
      if (request === generation) error = `Summary unavailable; no data or configuration writes: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; reportBusy(); }
    }
  }
  async function loadPreferences() {
    if (workbookBusy || !preferences || preferenceView?.busy || preferenceView?.blocked) return;
    cancel(); const request = generation; const token = preferences.beginLoad();
    error = ''; busy = true; reportBusy();
    try {
      const value = await reads.track(SiteUnitSummaryPreferencesService.GetSiteUnitSummaryPreferences(contextId));
      if (request !== generation) return;
      preferences.applyLoaded(value, token);
    } catch (cause) {
      if (request === generation) error = `Saved summary options unavailable; no defaults applied: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; reportBusy(); }
    }
  }
  async function savePreferences() {
    if (workbookBusy) return;
    if (!preferences || busy || speciesBarrier || preferenceView?.busy || preferenceView?.blocked) return;
    cancel(); error = '';
    try {
      await preferences.save(request => SiteUnitSummaryPreferencesService.SaveSiteUnitSummaryPreferences(contextId, JSON.stringify(request)));
    } catch (cause) { error = String(cause); }
  }
  function editMethod(value: number) {
    if (workbookBusy || speciesBarrier) return;
    cancel(); error = ''; preview = null; speciesPreview = null;
    if (preferences) preferences.editMethod(value);
    else method = value;
  }
  function editMode(value: SiteUnitSummaryMode) {
    if (busy || workbookBusy || preferenceBarrier || speciesBarrier || (value === 'lifeform' && !lifeformsEnabled)) return;
    cancel(); error = ''; preview = null; speciesPreview = null; mode = value;
  }
  function editSpeciesEnabled(enabled: boolean) {
    if (busy || workbookBusy || preferenceBarrier || speciesBarrier || !ready || !criteria) return;
    cancel(); error = ''; criteria.setEnabled(enabled);
  }
  function editSpeciesText(field: 'presenceGreaterThan' | 'coverGreaterThan', value: string) {
    if (busy || workbookBusy || preferenceBarrier || !criteria) return;
    cancel(); error = ''; criteria.editText(field, value);
  }
  function editSpeciesChoice(field: 'coverCalculation' | 'andOr', value: number) {
    if (busy || workbookBusy || preferenceBarrier || !criteria) return;
    cancel(); error = ''; criteria.editChoice(field, value);
  }
  function resetSpeciesCriteria() {
    if (busy || workbookBusy || preferenceBarrier || !criteria) return;
    cancel(); error = ''; criteria.reset();
  }
  onMount(() => {
    if (!preferencesEnabled) void options();
    else if (!preferenceView?.draft && !preferenceBarrier) void loadPreferences();
  });
  onDestroy(() => { unsubscribe?.(); unsubscribeSpecies?.(); cancel(); });
</script>

<section class="summary" data-site-unit-summary aria-label="Summary Environment">
  <h1>Summary Environment</h1>
  {#if error}<p role="alert" class="error">{error}</p>{/if}
  {#if preferenceView?.error}<p role="alert" class="error">{preferenceView.error}</p>{/if}
  {#if preferenceView?.draftError}<p role="alert" class="error">{preferenceView.draftError}</p>{/if}
  {#if speciesView?.error}<p role="alert" class="error" data-summary-species-error>{speciesView.error}</p>{/if}
  <p>Project {project}; selected normal SU {su}. Profile navigation does not filter this report.</p>
  {#if lifeformsEnabled}
    <fieldset disabled={busy || workbookBusy || preferenceBarrier || speciesBarrier || !ready}>
      <legend>Vegetation summary preview</legend>
      <label><input type="radio" name="summary-vegetation" checked={mode === 'layer'} onchange={() => editMode('layer')} />Layer covers</label>
      <label><input type="radio" name="summary-vegetation" checked={mode === 'lifeform'} onchange={() => editMode('lifeform')} />Lifeform cover values</label>
    </fieldset>
    {#if mode === 'lifeform'}
      <p>Explicit read-only lifeform preview; saved report grouping and species-list options are unchanged.
        Lifeform values use source minimum/mean/maximum even when other fields use Interquartile.
        {#if !speciesEnabled}Species-threshold output is unavailable.{/if}
        {#if !extendedWorkbookEnabled}Lifeform workbook publication remains unavailable.{/if}</p>
    {/if}
  {/if}
  <fieldset disabled={busy || workbookBusy || speciesBarrier || preferenceView?.busy || preferenceView?.blocked || !ready} data-source-control="optValueMethod">
    <legend>Quantitative Values</legend>
    <label><input type="radio" name="summary-method" value={1} checked={method === 1} onchange={() => editMethod(1)} />Mean</label>
    <label><input type="radio" name="summary-method" value={2} checked={method === 2} onchange={() => editMethod(2)} />Interquartile (25% - 50% - 75%)</label>
  </fieldset>
  {#if speciesEnabled && speciesView}
    <fieldset disabled={busy || workbookBusy || preferenceBarrier}>
      <legend>Species preview criteria</legend>
      <label><input type="checkbox" data-summary-species-enabled checked={speciesView.enabled} disabled={speciesBarrier || !ready}
        onchange={(event) => editSpeciesEnabled(event.currentTarget.checked)} />Include species list in preview</label>
      {#if speciesView.enabled}
        <label>Cover calculation<select data-summary-species-average value={speciesView.coverCalculation}
          onchange={(event) => editSpeciesChoice('coverCalculation', Number(event.currentTarget.value))}>
          <option value={1}>All plots (physical report count)</option><option value={2}>Present values</option>
        </select></label>
        <label>Criteria combination<select data-summary-species-combination value={speciesView.andOr}
          onchange={(event) => editSpeciesChoice('andOr', Number(event.currentTarget.value))}>
          <option value={1}>AND</option><option value={2}>OR</option>
        </select></label>
        <label>Presence greater than (%)<input data-summary-species-presence inputmode="numeric" value={speciesView.presenceGreaterThan}
          aria-invalid={!!speciesView.error} oninput={(event) => editSpeciesText('presenceGreaterThan', event.currentTarget.value)} /></label>
        <label>Cover greater than (%)<input data-summary-species-cover inputmode="numeric" value={speciesView.coverGreaterThan}
          aria-invalid={!!speciesView.error} oninput={(event) => editSpeciesText('coverGreaterThan', event.currentTarget.value)} /></label>
        <button data-summary-species-reset onclick={resetSpeciesCriteria}>Reset preview criteria</button>
      {/if}
    </fieldset>
    <p class="guidance">Explicit read-only criteria start at all plots, AND, and zero thresholds; they do not load or save source species preferences.
      Draft criteria/errors survive remounts. Strict thresholds compare source-formatted values; physical presence can exceed100%.
      Layer species use raw covers in layers1-7; lifeform species use capped plot/species covers.
      {#if !extendedWorkbookEnabled}Species workbook publication is unavailable.{/if}</p>
  {/if}
  <p data-source-control="optSiteUnitType">Site-unit type:
    {siteUnitType === 1 ? 'Normal SU' : siteUnitType === 2 ? 'Hierarchy (unavailable)' :
      siteUnitType === 3 ? 'Field-derived units (unavailable)' : 'not loaded'}.</p>
  <div class="actions">
    <button data-source-control="btnCreateReport" disabled={busy || workbookBusy || preferenceBarrier || speciesBarrier || !ready || su === 'None'} onclick={show}>Create Report</button>
    {#if preferencesEnabled}
      <button data-summary-preferences-load disabled={workbookBusy || !!preferenceView?.busy || !!preferenceView?.blocked} onclick={loadPreferences}>Load saved summary options</button>
      <button data-summary-preferences-save disabled={busy || workbookBusy || speciesBarrier || !!preferenceView?.busy || !!preferenceView?.blocked || !!preferenceView?.draftError ||
        !preferenceView?.saved || !ready} onclick={savePreferences}>Save reviewed summary options</button>
      <button data-summary-preferences-undo disabled={workbookBusy || !!preferenceView?.busy || !!preferenceView?.blocked || !preferenceView?.saved}
        onclick={() => { if (workbookBusy) return; cancel(); preferences?.undo(); }}>Undo summary options</button>
      {#if siteUnitType !== 1}
        <button data-summary-preferences-normal disabled={workbookBusy || !!preferenceView?.busy || !!preferenceView?.blocked || !preferenceView?.draft}
          onclick={() => { if (workbookBusy) return; cancel(); preferences?.selectNormalSU(); }}>Select normal SU</button>
      {/if}
      {#if preferenceView?.blocked}
        <button data-summary-preferences-acknowledge disabled={workbookBusy || preferenceView.busy}
          onclick={() => { if (!workbookBusy) preferences?.acknowledge(); }}>Acknowledge summary preference outcome</button>
      {/if}
    {:else}
      <button data-summary-options-reload disabled={busy || workbookBusy} onclick={options}>Reload summary options</button>
    {/if}
    {#if busy}<button data-summary-cancel onclick={cancel}>Cancel report read</button><p role="status">Reading owned summary options or snapshots...</p>{/if}
  </div>
  {#if import.meta.env.VITE_SITE_UNIT_SUMMARY_WORKBOOK === 'true' && mode === 'layer' && !speciesView?.enabled}
    <SiteUnitSummaryWorkbookPanel owner={{ contextId, project, projectPath, su, suPath }} {method}
      disabled={busy || preferenceBarrier || speciesBarrier || !ready || su === 'None'}
      onBusyChange={(held) => { workbookBusy = held; reportBusy(); }} />
  {:else if extendedWorkbookEnabled && (mode === 'lifeform' || speciesView?.enabled)}
    <SiteUnitSummaryWorkbookPanel owner={{ contextId, project, projectPath, su, suPath }} {method} {extendedOptions}
      disabled={busy || preferenceBarrier || speciesBarrier || !ready || su === 'None'}
      onBusyChange={(held) => { workbookBusy = held; reportBusy(); }} />
  {/if}
  {#if preferenceView?.busy}<p role="status">Saving reviewed summary options; this preference write cannot be cancelled.</p>{/if}
  {#each preferenceView?.receipts ?? [] as receipt}
    {#if receipt.kind === 'known'}
      <p data-summary-preferences-receipt role="status">Preference receipt: changed {String(receipt.outcome.changed)};
        committed {String(receipt.outcome.committed)}. {receipt.outcome.errorMessage}</p>
    {:else}
      <p data-summary-preferences-receipt data-summary-preferences-unknown-receipt role="status">
        Preference receipt: outcome unknown; changed unknown; committed unknown. {receipt.errorMessage}</p>
    {/if}
  {/each}
  <p class="guidance">Read-only normal-SU Environment fields. Saved calculation method and site-unit type load from configuration.
    {#if preferencesEnabled}Explicit reviewed Save replaces the source's immediate AfterUpdate preferences. It changes only the two saved summary keys, never plot data or registry.
      Drafts and receipts survive tab remounts. Unknown outcomes require acknowledgement and explicit Load, never replay.
    {:else}Calculation changes are preview-only; Reload restores saved options. No registry or saved-preference changes.{/if}
    Hierarchy/field-derived units {#if !speciesEnabled}and species-list output {/if}remain unavailable.
    Lifeform cover values require their separate preview gate.
    {#if extendedWorkbookEnabled}Extended workbooks additionally require their independent backend gate and the corresponding preview gates.
    {:else}Only layer-cover workbooks without a species list are supported.{/if}</p>
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
        {#if speciesPreview}
          {@const speciesUnit = speciesPreview.units[unitIndex]}
          <section data-summary-species-unit aria-label={`Vegetation species for ${unit.code}`}>
            <h3>Vegetation species</h3>
            {#each speciesUnit.groups as group}
              <h4>{group.caption}</h4>
              {#if group.rows.some(row => row.included)}
                <div class="species-table"><table>
                  <thead><tr><th scope="col">Scientific Name</th><th scope="col">Common Name</th><th scope="col">%Cover</th><th scope="col">%Presence</th></tr></thead>
                  <tbody>{#each group.rows.filter(row => row.included) as row}
                    <tr><th scope="row" aria-label={`${reportCellText(row.scientificName)} (${row.species})`}>{reportCellText(row.scientificName)}</th>
                      <td>{reportCellText(row.englishName)}</td><td>{row.cover}</td><td>{row.presence}</td></tr>
                  {/each}</tbody>
                </table></div>
              {:else}<p>No species pass the strict formatted criteria.</p>{/if}
              <details><summary>Physical species values, definitions and excluded rows</summary>
                <ul>{#each group.rows as row}
                  <li>{row.species}: {row.cover} cover; {row.presence} presence; {row.physicalValues} physical values;
                    UNION reference rows {row.referenceRowIds.join(', ')}; {row.included ? 'included' : 'excluded'}.</li>
                {/each}</ul>
              </details>
            {:else}<p>No matched species definitions in the reportable groups.</p>{/each}
          </section>
        {/if}
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
  .species-table { overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; margin: .5rem 0; }
  th, td { padding: .5rem; border-bottom: 1px solid #cbd5e1; text-align: left; overflow-wrap: anywhere; }
  select, input:not([type="radio"]):not([type="checkbox"]) { max-width: 100%; border: 1px solid #94a3b8; border-radius: .25rem; padding: .4rem; }
  @media (max-width: 640px) { dl > div { grid-template-columns: minmax(0, 1fr); } }
</style>
