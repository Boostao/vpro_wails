<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import type { EnvironmentSiteUnitReview, SiteUnitEnvironmentReview, ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
  import type { bindContextPlots } from './contextPlots';
  import type { EditorCloseState } from './closeLifecycle';
  import { ReadRequests } from './readRequests';
  import { environmentSURequest, validateEnvironmentSUReview, validateEnvironmentSUResult } from './environmentSUTransfer';
  import { siteUnitEnvironmentRequest, validateSiteUnitEnvironmentReview, validateSiteUnitEnvironmentResult } from './siteUnitEnvironment';

  let { client, contextId, project, su, path, onclosed, direction = 'forward' }: {
    client: ReturnType<typeof bindContextPlots>; contextId: string; project: string; su: string; path: string; onclosed: () => void;
    direction?: 'forward' | 'reverse';
  } = $props();
  let review = $state<EnvironmentSiteUnitReview | SiteUnitEnvironmentReview | null>(null);
  const caption = $derived(direction === 'reverse' ? 'SU Into Env' : 'Env Into SU');
  let busy = $state(false);
  let error = $state<string | null>(null);
  let success = $state<string | null>(null);
  let committedFailure = $state(false);
  let generation = 0;
  const reads = new ReadRequests();
  onMount(() => { void reload(); });
  onDestroy(() => { generation++; reads.cancelAll(); });
  export function getCloseState(): EditorCloseState {
    return { unsaved: true, busy, canSave: false, error,
      saveReason: 'Confirm or dismiss the SU transfer and return to the plot explicitly; ordinary Save never applies it.' };
  }
  export function undo() {
    if (busy || committedFailure) { error = 'Wait or reload the committed transfer without replaying it.'; return; }
    review = null; error = null; success = 'SU transfer review dismissed; no stored values changed.';
  }
  async function reload() {
    if (busy) return;
    const request = ++generation;
    reads.cancelAll(); busy = true; error = null; review = null;
    try {
      if (direction === 'reverse') {
        const result = await reads.track(client.ReviewSiteUnitEnvironment());
        if (request !== generation) return;
        review = validateSiteUnitEnvironmentReview(result, contextId, project, su, path);
      } else {
        const result = await reads.track(client.ReviewEnvironmentSiteUnits());
        if (request !== generation) return;
        review = validateEnvironmentSUReview(result, contextId, project, su, path);
      }
      committedFailure = false;
    } catch (cause) {
      if (request === generation) error = `SU transfer review unavailable; no write applied: ${String(cause)}`;
    } finally { if (request === generation) busy = false; }
  }
  async function transfer() {
    if (busy || committedFailure || !review?.changes?.length) {
      error = 'Review a nonempty available SU transfer before explicit confirmation.'; return;
    }
    busy = true; error = null; success = null;
    let committed = false;
    try {
      if ('source' in review) {
        const request = siteUnitEnvironmentRequest($state.snapshot(review), contextId, project, su, path);
        const result = await client.TransferSiteUnitEnvironment(request);
        committed = true; review = null;
        validateSiteUnitEnvironmentResult(result, request);
        success = `Copied ${result.changedCells} reviewed Admin values across ${result.changedRows} plots; typed history ${result.historyId} retained atomically. SU, Env, original audits and support data unchanged. Return to plot reloads the stored header.`;
      } else {
        const request = environmentSURequest($state.snapshot(review), contextId, project, su, path);
        const result = await client.TransferEnvironmentSiteUnits(request);
        committed = true; review = null;
        validateEnvironmentSUResult(result, request);
        success = `Copied ${result.changedRows} environmental Working Unit values into ${su}_SU; typed transfer history ${result.historyId} retained atomically. No parent, other SU row, original audit or support database changed.`;
      }
    } catch (cause) {
      if (committed || String(cause).includes('environment/SU transfer committed, but') || String(cause).includes('SU/environment transfer committed, but')) {
        committedFailure = true; review = null;
        error = `Transfer committed; reload without replaying the completed write: ${String(cause)}`;
      } else error = `Transfer failed; original data retained and review kept for retry or explicit dismissal: ${String(cause)}`;
    } finally { busy = false; }
  }
  function close() {
    if (busy || error || committedFailure || review?.changes?.length) {
      error = 'Finish or dismiss the reviewed transfer; reload any committed failure before returning to the plot.'; return;
    }
    onclosed();
  }
  function cell(value: ProjectMetadataCell) {
    return value.storage === 'null' ? 'NULL' : `Text ${JSON.stringify(value.text)}`;
  }
</script>

<section class="transfer-panel" aria-label={caption} data-environment-su-transfer data-su-environment-transfer={direction === 'reverse' ? '' : undefined}>
  <h3>{caption}: review before copying</h3>
  {#if error}<p class="failure" role="alert">{error}</p>{/if}
  {#if success}<p role="status">{success}</p>{/if}
  {#if busy}<p role="status">Reading or applying the owned-context transfer...</p>{/if}
  <p>Project {project}; selected SU {su}; owned project file {path}.</p>
  {#if review}
    <p>{review.changes?.length} changed {direction === 'reverse' ? 'cells' : 'rows'}. This covers matching plots in the selected SU, not only the displayed plot or profile-navigation subset.</p>
    <div class="table-scroll"><table>
      <thead><tr><th>Plot / field</th><th>Current value</th><th>Reviewed value / source</th></tr></thead>
      <tbody>{#each review.changes ?? [] as change (`${change.adminRowId}/${change.suRowId}/${'field' in change ? change.field : ''}`)}
        <tr><th scope="row">{change.plotNumber}{#if 'field' in change} / {change.field}{/if}</th><td>{cell(change.before)}</td><td>{cell(change.after)}{#if 'origin' in change} / {change.origin}{/if}</td></tr>
      {/each}</tbody>
    </table></div>
  {/if}
  <div class="toolbar">
    <button type="button" disabled={busy || committedFailure || !review?.changes?.length} onclick={transfer}>{direction === 'reverse' ? 'Confirm SU into environment' : 'Confirm environmental units into SU'}</button>
    <button type="button" disabled={busy || committedFailure} onclick={undo}>Dismiss SU transfer review</button>
    <button type="button" disabled={busy || Boolean(review?.changes?.length)} onclick={reload}>Reload SU transfer review</button>
    <button type="button" disabled={busy || error !== null || committedFailure || Boolean(review?.changes?.length)} onclick={close}>Return to plot</button>
  </div>
  <p class="guidance">{#if direction === 'reverse'}Copies selected SU values to Admin.UserSiteUnit and uniquely resolved short/long names. Personal definitions override master names; missing definitions retain existing names. Changed locked plots and duplicate definitions are rejected. NULL and empty text remain distinct; no definition or support-file write occurs.{:else}Copies only stored Admin.UserSiteUnit through unique original Env/Admin/SU links. NULL and empty text remain distinct. No reverse copy, personal-definition creation, automatic selection or independent-file write occurs.{/if}</p>
</section>

<style>
  .transfer-panel { padding: 1rem; border: 1px solid #d6d3d1; border-radius: .5rem; }
  h3 { font-weight: 600; } p { margin: .5rem 0; } .failure { color: #991b1b; }
  .toolbar { display: flex; flex-wrap: wrap; gap: .5rem; margin: .75rem 0; }
  button { padding: .5rem .75rem; border: 1px solid #d6d3d1; border-radius: .375rem; }
  button:disabled { opacity: .5; } .table-scroll { overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; } th, td { padding: .5rem; border-bottom: 1px solid #e7e5e4; text-align: left; overflow-wrap: anywhere; }
  .guidance { color: #57534e; font-size: .875rem; }
</style>
