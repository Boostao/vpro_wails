<script lang="ts">
  import { onDestroy } from 'svelte';
  import { LifeformSummaryService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { validateLifeformSummary, lifeformCellText, lifeformRatioText, type LifeformSummaryPreview } from './lifeformSummary';

  let { contextId, project, projectPath, su, suPath, onBusyChange }: {
    contextId: string; project: string; projectPath: string; su: string; suPath: string;
    onBusyChange: (busy: boolean) => void;
  } = $props();
  const enabled = import.meta.env.VITE_LIFEFORM_SUMMARY === 'true';
  const reads = new ReadRequests();
  let generation = 0;
  let busy = $state(false);
  let error = $state('');
  let preview = $state<LifeformSummaryPreview | null>(null);
  const ownerKey = $derived(JSON.stringify([contextId, project, projectPath, su, suPath]));
  let previewKey = $state('');
  const currentPreview = $derived(previewKey === ownerKey ? preview : null);
  function cancel() { generation++; reads.cancelAll(); busy = false; onBusyChange(false); }
  async function show() {
    if (!enabled || busy || su === 'None' || su === 'USysSuTableDynamic') return;
    cancel(); const request = generation, key = ownerKey;
    const owner = { contextId, project, projectPath, su, suPath };
    busy = true; error = ''; preview = null; onBusyChange(true);
    try {
      const value = await reads.track(LifeformSummaryService.Preview(contextId));
      if (request !== generation || key !== ownerKey) return;
      preview = validateLifeformSummary(value, owner); previewKey = key;
    } catch (cause) {
      if (request === generation && key === ownerKey) error = `Lifeform Summary unavailable; no data or configuration writes: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  onDestroy(cancel);
</script>

<section class="lifeform-summary" data-lifeform-summary aria-label="Lifeform Summary">
  <h1>Lifeform Summary</h1>
  {#if error}<p role="alert" class="error">{error}</p>{/if}
  <p>Project {project}; selected normal SU {su}. Profile navigation does not filter this report.</p>
  {#if !enabled}<p>This preview is disabled in this build.</p>{/if}
  {#if su === 'None' || su === 'USysSuTableDynamic'}<p>A selected normal SU is required. Hierarchy and dynamic breaks are unavailable.</p>{/if}
  <div class="actions">
    <button onclick={show} disabled={!enabled || busy || su === 'None' || su === 'USysSuTableDynamic'}>Preview Lifeform Summary</button>
    {#if busy}<button onclick={cancel}>Cancel preview</button><p role="status">Reading owned project and SU…</p>{/if}
  </div>
  {#if currentPreview}
    {#each currentPreview.report.units as unit}
      <article>
        <h2>Unit: {lifeformCellText(unit.code)}</h2>
        <dl>
          <div><dt>nPlots (physical non-NULL PlotNumber count)</dt><dd>{unit.nPlots}</dd></div>
          <div><dt>Number of unique species</dt><dd>{unit.uniqueSpecies}</dd></div>
          <div><dt>Total species occurrences</dt><dd>{unit.occurrences}</dd></div>
        </dl>
        <div class="table-scroll">
          <table>
            <caption>Lifeform presence and mean cover for {lifeformCellText(unit.code)}</caption>
            <thead><tr><th scope="col">Lifeform</th><th scope="col">Catalogue label</th><th scope="col">Definition</th><th scope="col">Short name</th><th scope="col">Presence</th><th scope="col">Mean Cover</th></tr></thead>
            <tbody>
              {#each unit.rows as row, i}
                <tr><th scope="row">{row.lifeform}</th><td>{lifeformCellText(currentPreview.report.catalogue[i].label)}</td>
                  <td>{lifeformCellText(currentPreview.report.catalogue[i].definition)}</td><td>{lifeformCellText(currentPreview.report.catalogue[i].shortName)}</td>
                  <td>{lifeformRatioText(row.presence)}</td><td>{lifeformRatioText(row.meanCover)}</td></tr>
              {/each}
            </tbody>
          </table>
        </div>
      </article>
    {/each}
    {#if currentPreview.report.units.length === 0}<p>No physical SU memberships.</p>{/if}
  {/if}
  <p class="guidance">Read-only preview: no database, configuration or Excel writes. Global inserted rows rejoin SU by PlotNumber, retaining duplicate and cross-unit weights. NULL means stay NULL. Catalogue rows use deterministic physical row order, not a claim about Access collation. Zero denominators are shown as NULL instead of evaluating division by zero. Attribute summaries, hierarchy breaks and export remain unavailable.</p>
</section>

<style>
  .lifeform-summary { display: grid; gap: 1rem; min-width: 0; }
  h1, h2, p { margin: 0; }
  .actions { display: flex; gap: .75rem; flex-wrap: wrap; align-items: center; }
  article { display: grid; gap: .75rem; padding: 1rem; border: 1px solid #9ca3af; border-radius: .5rem; min-width: 0; }
  dl { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 15rem), 1fr)); gap: .75rem; margin: 0; }
  dt { font-weight: 600; } dd { margin: .25rem 0 0; }
  .table-scroll { overflow-x: auto; }
  table { border-collapse: collapse; width: 100%; }
  th, td { padding: .5rem; border-bottom: 1px solid #9ca3af; text-align: left; }
  caption { text-align: left; padding-bottom: .5rem; }
  .error { color: #b91c1c; } .guidance { font-size: .9rem; }
</style>
