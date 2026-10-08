<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import { AuditRestoreAction, type ProjectMetadataRestoreHistory, type ProjectMetadataRestoreReview } from '../bindings/github.com/boostao/vpro-wails';
  import type { bindContextPlots } from './contextPlots';
  import type { EditorCloseState } from './closeLifecycle';
  import { ReadRequests } from './readRequests';
  import { metadataCellText } from './projectMetadataEditor';
  import { metadataLabel } from './projectMetadataPresentation';
  import { validateMetadataRestorationHistory, validateMetadataRestorationReview, metadataRestorationRequest, validateMetadataRestorationResult } from './projectMetadataRestore';

  let { client, contextId, plot, onclosed, onbusy, oncommitted }: {
    client: ReturnType<typeof bindContextPlots>; contextId: string; plot: string;
    onclosed: () => void; onbusy: (busy: boolean) => void; oncommitted: (rowId: string) => Promise<void>;
  } = $props();
  let history = $state<ProjectMetadataRestoreHistory | null>(null);
  let proposal = $state<ProjectMetadataRestoreReview | null>(null);
  let reading = $state(false);
  let saving = $state(false);
  let committedRow = $state<string | null>(null);
  let committedFailure = $state(false);
  let error = $state<string | null>(null);
  let success = $state<string | null>(null);
  let generation = 0;
  const reads = new ReadRequests();
  const busy = $derived(reading || saving);
  $effect(() => onbusy(busy));
  onMount(() => { void reload(); });
  onDestroy(() => { generation++; reads.cancelAll(); onbusy(false); });

  export function getCloseState(): EditorCloseState {
    return { unsaved: true, busy, canSave: false, error, blocked: committedFailure,
      saveReason: 'Restore or explicitly dismiss metadata restoration before closing; ordinary Save never applies this proposal.' };
  }
  export function undo() {
    if (busy || committedFailure) { error = 'Wait or reload a committed restoration; completed writes must not be replayed.'; return; }
    proposal = null; error = null; success = 'Restoration proposal dismissed; no data or history changed.';
  }
  async function reload() {
    if (busy || proposal) { error = 'Dismiss the existing restoration proposal before reloading history.'; return; }
    const request = ++generation;
    reads.cancelAll(); reading = true; history = null; error = null;
    try {
      if (committedFailure && committedRow !== null) await oncommitted(committedRow);
      const result = await reads.track(client.ListProjectMetadataRestoreHistory(plot));
      if (request !== generation) return;
      history = validateMetadataRestorationHistory(result); committedFailure = false;
    } catch (cause) {
      if (request === generation) error = `Metadata history unavailable; no restoration was applied: ${String(cause)}`;
    } finally { if (request === generation) reading = false; }
  }
  async function review(historyId: string) {
    if (busy || proposal || committedFailure || !history?.events?.some(event => event.historyId === historyId && !event.restored)) {
      error = 'Select one available unconsumed typed metadata edit after dismissing any previous proposal.'; return;
    }
    const request = ++generation;
    reading = true; error = null; success = null;
    try {
      const result = await reads.track(client.ReviewProjectMetadataRestoration(plot, historyId));
      if (request !== generation) return;
      proposal = validateMetadataRestorationReview(result, contextId, plot);
      if (proposal.historyId !== historyId) throw new Error('Restoration returned another technical edit identity.');
    } catch (cause) {
      if (request === generation) { proposal = null; error = `Restoration review rejected; existing data/history retained: ${String(cause)}`; }
    } finally { if (request === generation) reading = false; }
  }
  async function restore(action: AuditRestoreAction) {
    if (busy || !proposal || committedFailure) { error = 'Review one available typed metadata edit before explicit restoration.'; return; }
    let committed = false;
    saving = true; error = null; success = null;
    try {
      const request = metadataRestorationRequest($state.snapshot(proposal), action, contextId, plot);
      const result = await client.RestoreProjectMetadata(request);
      committed = true; committedRow = request.review.current.rowId; proposal = null;
      validateMetadataRestorationResult(result, request);
      await oncommitted(committedRow);
      history = validateMetadataRestorationHistory(await client.ListProjectMetadataRestoreHistory(plot));
      success = `Restored ${result.restoredRows} audited metadata fields; ${result.prunedAuditRows} proven audit rows pruned. Original typed provenance and no-replay receipt retained. No parent, other metadata record or support database changed.`;
    } catch (cause) {
      if (committed || String(cause).includes('metadata edit committed, but')) {
        committedRow ??= proposal?.current.rowId ?? null;
        committedFailure = true; proposal = null;
        error = `Metadata restoration committed, but response/refresh/cleanup failed. Reload without replaying the completed write: ${String(cause)}`;
      } else error = `Restoration failed; reviewed proposal retained for retry or explicit dismissal: ${String(cause)}`;
    } finally { saving = false; }
  }
  function close() {
    if (busy || proposal || committedFailure || error) { error = 'Wait, reload a committed result, or explicitly dismiss the failed/proposed restoration before closing.'; return; }
    onclosed();
  }
</script>

<section aria-label="Typed metadata restoration" data-metadata-restoration>
  <h3>Restore explicitly audited metadata fields</h3>
  {#if error}<p class="failure" role="alert">{error}</p>{/if}
  {#if success}<p role="status">{success}</p>{/if}
  {#if busy}<p role="status">{saving ? 'Restoring metadata atomically…' : 'Reading typed metadata history…'}</p>{/if}
  <div class="toolbar">
    <button type="button" disabled={busy || proposal !== null} onclick={reload}>Reload typed metadata history</button>
    <button type="button" disabled={busy || committedFailure} onclick={undo}>Dismiss restoration proposal</button>
    <button type="button" disabled={busy || proposal !== null || committedFailure || error !== null} onclick={close}>Return to metadata editor</button>
  </div>
  {#if proposal}
    <p>Edit {proposal.historyId}; metadata physical row {proposal.current.rowId}, ID {proposal.id}.
      Existing Project ID {JSON.stringify(proposal.projectId)} and parent {proposal.plotNumber} remain unchanged.</p>
    <div class="table-scroll"><table>
      <thead><tr><th>Audited field</th><th>Current typed value</th><th>Reviewed restoration</th></tr></thead>
      <tbody>{#each proposal.audits ?? [] as audit (audit.rowId)}
        {@const index = proposal.columns?.findIndex(column => column.name === audit.editField) ?? -1}
        <tr><th scope="row">{metadataLabel(audit.editField)}</th>
          <td>{proposal.current.cells?.[index]?.storage}: {proposal.current.cells?.[index] ? metadataCellText(proposal.current.cells[index]) || 'Empty/NULL' : 'Unavailable'}</td>
          <td>{proposal.restored.cells?.[index]?.storage}: {proposal.restored.cells?.[index] ? metadataCellText(proposal.restored.cells[index]) || 'Empty/NULL' : 'Unavailable'}</td></tr>
      {/each}</tbody>
    </table></div>
    <button type="button" disabled={busy || committedFailure} onclick={() => restore(AuditRestoreAction.AuditRestoreRetain)}>Restore metadata and retain proven audits</button>
    <button type="button" disabled={busy || committedFailure} onclick={() => restore(AuditRestoreAction.AuditRestorePrune)}>Restore metadata and prune proven audits</button>
  {:else}
    <fieldset disabled={busy || committedFailure || history === null}><legend>Select one typed edit explicitly</legend>
      {#each history?.events ?? [] as event (event.historyId)}
        <button type="button" data-metadata-history={event.historyId} disabled={event.restored}
          onclick={() => review(event.historyId)}>Edit {event.historyId}, physical row {event.rowId}: {event.fields?.join(', ')}{event.restored ? ' (already restored)' : ''}</button>
      {:else}<p>No typed edits for this parent. Older plaintext audit rows remain visible in Audit, but cannot safely prove historical storage classes.</p>{/each}
    </fieldset>
  {/if}
  <p>Only proven audited fields are restored. Unaudited changes, NULL versus empty text, historical unchanged values, other records and original table descriptions remain intact.
    Source metadata does not call AuditTrail; this is a typed-storage desktop adaptation, not native Access restoration parity.</p>
</section>

<style>
  h3 { font-weight: 600; }
  .failure { color: #b91c1c; }
  .toolbar { display: flex; flex-wrap: wrap; gap: .5rem; margin: 1rem 0; }
  button { border: 1px solid #94a3b8; border-radius: .35rem; padding: .5rem .75rem; margin: .25rem; overflow-wrap: anywhere; }
  button:disabled { opacity: .5; }
  fieldset { border: 1px solid #cbd5e1; padding: .75rem; min-width: 0; }
  .table-scroll { overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; margin: 1rem 0; }
  th, td { border: 1px solid #cbd5e1; padding: .5rem; text-align: left; overflow-wrap: anywhere; }
  td { min-width: 8rem; }
</style>
