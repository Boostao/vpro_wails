<script lang="ts">
  import { onMount, onDestroy, untrack } from 'svelte';
  import { ExternalSUEnvironmentService } from '../bindings/github.com/boostao/vpro-wails';
  import type { ExternalSUEnvironmentReview, ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
  import { externalSUEnvironmentSession, validateExternalSUEnvironmentReview } from './externalSUEnvironment';
  import { ReadRequests } from './readRequests';
  import type { EditorCloseState } from './closeLifecycle';

  let { contextId, project, projectPath, su, suPath, onclosed }: {
    contextId: string; project: string; projectPath: string; su: string; suPath: string; onclosed: () => void;
  } = $props();
  const scope = untrack(() => ({ contextId, project, projectPath, su, suPath }));
  const session = externalSUEnvironmentSession(scope);
  let activity = $state.raw(session.view());
  let review = $state.raw<ExternalSUEnvironmentReview | null>(null);
  let reading = $state(false);
  let error = $state('');
  let generation = 0;
  const reads = new ReadRequests();
  onMount(() => {
    const unsubscribe = session.subscribe(() => { activity = session.view(); });
    if (!session.view().request) void reload();
    return unsubscribe;
  });
  onDestroy(() => { generation++; reads.cancelAll(); });
  export function getCloseState(): EditorCloseState {
    const retained = session.closeState();
    if (retained.blocked || retained.busy) return retained;
    return { unsaved: reading || !!error || Boolean(review?.review.changes?.length), busy: reading, canSave: false,
      error: error || null, saveReason: 'Finish or dismiss the explicit transfer review before ordinary Save, Lock or close.' };
  }
  export function undo() {
    if (session.view().blocked) { error = 'Recover and acknowledge the retained transfer; Undo cannot reverse or erase its outcome.'; return; }
    cancelReview(); review = null; error = '';
  }
  function cancelReview() {
    generation++; reads.cancelAll(); reading = false;
  }
  function cancel() {
    if (reading) { cancelReview(); error = 'Review cancelled; no transfer request was sent.'; }
    else session.cancel();
  }
  async function reload() {
    if (reading || session.view().blocked || session.view().busy) return;
    const request = ++generation;
    reading = true; error = ''; review = null;
    try {
      const value = await reads.track(ExternalSUEnvironmentService.GetReview(scope.contextId));
      if (request !== generation) return;
      const checked = validateExternalSUEnvironmentReview(value, scope);
      if (request !== generation) return;
      review = checked;
    } catch (cause) {
      if (request === generation) error = `External SU Into Env review unavailable; no transfer applied: ${String(cause)}`;
    } finally { if (request === generation) reading = false; }
  }
  async function transfer() {
    if (!review || reading || session.view().busy || session.view().blocked) {
      error = 'Review available changes and resolve the retained transfer before explicit confirmation.';
      return;
    }
    const approved = review;
    try {
      await session.transfer(approved, request => ExternalSUEnvironmentService.TransferReviewed(scope.contextId, JSON.stringify(request)));
      review = null;
    } catch (cause) { error = `Transfer was not started: ${String(cause)}`; }
  }
  async function recover() {
    try { await session.recover(request => ExternalSUEnvironmentService.RecoverOutcome(scope.contextId, JSON.stringify(request))); }
    catch (cause) { error = `Readonly recovery unavailable; retain the exact request: ${String(cause)}`; }
  }
  function acknowledge() {
    try { session.acknowledge(); review = null; error = ''; }
    catch (cause) { error = String(cause); }
  }
  function close() {
    if (getCloseState().unsaved || getCloseState().busy) { error = 'Finish or dismiss the review and resolve any retained transfer before returning.'; return; }
    onclosed();
  }
  function cell(value: ProjectMetadataCell) { return value.storage === 'null' ? 'NULL' : `Text ${JSON.stringify(value.text)}`; }
</script>

<section class="external-transfer" aria-label="SU Into Env: separate selected file" data-external-su-environment>
  <h3>SU Into Env: separate selected file</h3>
  {#if error || activity.error}<p role="alert">{error || activity.error}</p>{/if}
  <dl class="scope"><dt>Target project</dt><dd>{project} / {projectPath}</dd><dt>Read-only selected SU</dt><dd>{su} / {suPath}</dd></dl>
  {#if reading}<p role="status">Reading the complete owned transfer sources...</p>{/if}
  {#if review}
    <p>{review.review.changes?.length} reviewed changed Admin cells across matching selected-SU plots, not only this plot.</p>
    <dl class="scope"><dt>Source digest</dt><dd>{review.sourceHash}</dd><dt>Approval</dt><dd>{review.approvalHash}</dd></dl>
    <div class="table-scroll"><table><thead><tr><th>Plot / field</th><th>Current value</th><th>Reviewed value / source</th></tr></thead>
      <tbody>{#each review.review.changes ?? [] as change (`${change.adminRowId}/${change.field}`)}
        <tr><th scope="row">{change.plotNumber} / {change.field}</th><td>{cell(change.before)}</td><td>{cell(change.after)} / {change.origin}</td></tr>
      {/each}</tbody></table></div>
  {/if}
  {#if activity.request}
    <dl class="scope"><dt>Retained request</dt><dd>{activity.request.requestId}</dd></dl>
    {#if activity.outcome}
      <p role="status">Exact transfer outcome: {activity.outcome.status}.</p>
      {#if activity.outcome.receipt}<p data-external-su-receipt>{activity.outcome.receipt.changedCells} cells across {activity.outcome.receipt.changedRows} plots; atomic history {activity.outcome.receipt.historyId}.</p>{/if}
      {#if activity.outcome.errorMessage}<p>{activity.outcome.errorMessage}</p>{/if}
    {:else}<p>Outcome is not yet certified. Retain this request and recover readonly; never repeat the transfer.</p>{/if}
  {/if}
  <div class="toolbar">
    <button type="button" disabled={reading || activity.busy || activity.blocked || !review?.review.changes?.length} onclick={transfer}>Confirm selected SU into environment</button>
    <button type="button" disabled={!reading && !activity.busy} onclick={cancel}>Cancel request wait</button>
    <button type="button" disabled={activity.busy || !activity.blocked} onclick={recover}>Recover transfer outcome readonly</button>
    <button type="button" disabled={activity.busy || !activity.blocked || !activity.outcome} onclick={acknowledge}>Acknowledge retained transfer outcome</button>
    <button type="button" disabled={reading || activity.busy || activity.blocked} onclick={undo}>Dismiss transfer review</button>
    <button type="button" disabled={reading || activity.busy || activity.blocked} onclick={reload}>Reload transfer review</button>
    <button type="button" disabled={reading || activity.busy || activity.blocked || Boolean(error) || Boolean(review?.review.changes?.length)} onclick={close}>Return to plot</button>
  </div>
  <p>Copies only reviewed selected-SU values and uniquely resolved names into the project Admin table. Personal definitions override master names; missing definitions preserve existing names. NULL and empty text remain distinct. Locked plots and ambiguous links/definitions refuse. Source SU, definitions, descriptions and original audits are not written; no forward external transfer is enabled.</p>
</section>

<style>
  .external-transfer { min-width: 0; padding: 1rem; border: 1px solid #d6d3d1; border-radius: .5rem; }
  h3 { font-weight: 600; } p, dl { margin: .75rem 0; } [role="alert"] { color: #991b1b; }
  .scope { display: grid; grid-template-columns: minmax(0, 1fr); gap: .25rem; } dt { font-weight: 600; }
  dd { min-width: 0; overflow-wrap: anywhere; white-space: pre-wrap; }
  .toolbar { display: flex; flex-wrap: wrap; gap: .5rem; }
  button { max-width: 100%; padding: .5rem .75rem; border: 1px solid #d6d3d1; border-radius: .375rem; }
  button:disabled { opacity: .5; } .table-scroll { overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; } th, td { padding: .5rem; border-bottom: 1px solid #e7e5e4; text-align: left; overflow-wrap: anywhere; }
</style>
