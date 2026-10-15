<script lang="ts">
  import { onDestroy } from 'svelte';
  import { UserLogService } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { userLogFromWire, userLogFields, userLogCellLabel, type UserLogView } from './userLogReview';

  let { contextId, operationHeld = false, onBusyChange }: {
    contextId: string; operationHeld?: boolean; onBusyChange: (busy: boolean) => void;
  } = $props();
  let mode = $state(2);
  let busy = $state(false);
  let error = $state('');
  let review = $state.raw<UserLogView | null>(null);
  const limit = 50;
  const reads = new ReadRequests();
  let generation = 0;

  function cancel() {
    generation++; reads.cancelAll(); busy = false; onBusyChange(false);
  }
  function changeMode(value: string) {
    cancel(); review = null;
    mode = value === '1' ? 1 : value === '2' ? 2 : 0;
    error = mode === 0 ? 'Choose Show All or Show Current; no spelling repair is applied.' : '';
  }
  async function show(offset = 0) {
    if (busy || operationHeld) { error = 'Wait for or cancel the current operation before reading the saved user log.'; return; }
    cancel();
    const request = generation, owner = contextId, selectedMode = mode;
    review = null; error = ''; busy = true; onBusyChange(true);
    try {
      if (!owner || selectedMode !== 1 && selectedMode !== 2) throw new Error('Choose a current context and a source Show mode.');
      const value = await reads.track(UserLogService.GetReview(owner, selectedMode, offset, limit));
      if (request !== generation || owner !== contextId) return;
      const validated = userLogFromWire(value, owner, selectedMode, offset, limit);
      if (request !== generation || owner !== contextId) return;
      review = validated;
    } catch (cause) {
      if (request === generation) error = `Saved user log unavailable; no session, data or configuration writes invoked: ${String(cause)}`;
    } finally {
      if (request === generation) { busy = false; onBusyChange(false); }
    }
  }
  onDestroy(cancel);
</script>

<section class="user-log" data-user-log-review aria-label="Saved VPro user log">
  <h1>VPro User Log</h1>
  {#if error}<p class="error" role="alert">{error}</p>{/if}
  <p>Saved Access log from this context's VPro64 support database, not a list of live Wails users.
    Read-only: no login/logout recording, editing or deletion.</p>
  <div class="actions">
    <label for="user-log-mode">Show
      <select id="user-log-mode" value={String(mode)} disabled={operationHeld} onchange={(event) => changeMode(event.currentTarget.value)}>
        <option value="1" selected={mode === 1}>Show All</option>
        <option value="2" selected={mode === 2}>Show Current</option>
      </select>
    </label>
    <button type="button" data-user-log-read disabled={busy || operationHeld || !contextId || mode !== 1 && mode !== 2} onclick={() => show()}>Review saved user log</button>
    {#if busy}
      <button type="button" data-user-log-cancel onclick={cancel}>Cancel user log read</button>
      <p role="status">Reading the owned saved log...</p>
    {/if}
  </div>
  {#if review}
    <dl class="provenance"><div><dt>Owned VPro64 database</dt><dd>{review.sourcePath}</dd></div></dl>
    <p data-user-log-count>{review.total} saved rows; showing {review.table.rows.length ? review.offset + 1 : 0}
      through {review.table.rows.length ? review.offset + review.table.rows.length : 0}.</p>
    <div class="table-scroll">
      <table data-user-log-table>
        <caption>Saved sessions: {review.mode === 1 ? 'Show All' : 'Show Current'}</caption>
        <thead><tr>{#each userLogFields as field}<th scope="col">{field.caption}</th>{/each}</tr></thead>
        <tbody>
          {#each review.table.rows as row (row.rowId)}
            <tr aria-label={`Saved session row ${row.rowId}`}>
              {#each userLogFields as field}
                <td>{userLogCellLabel(row.cells[review.table.columns.findIndex(column => column.name === field.name)])}</td>
              {/each}
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    <div class="actions">
      <button type="button" data-user-log-previous disabled={busy || operationHeld || review.offset === 0} onclick={() => show(Math.max(0, review!.offset - limit))}>Previous saved rows</button>
      <button type="button" data-user-log-next disabled={busy || operationHeld || review.offset + limit >= review.total} onclick={() => show(review!.offset + limit)}>Next saved rows</button>
    </div>
  {/if}
  <p class="guidance">Show Current means a stored NULL OutTime, not a verified active session.
    NULL and empty text remain distinct. Temporal values are displayed as stored, without timezone conversion.
    Technical row order and paging are desktop adaptations; logs above 10,000 physical rows are explicitly unavailable.</p>
</section>

<style>
  .user-log { min-width: 0; }
  .actions { display: flex; flex-wrap: wrap; align-items: end; gap: .75rem; margin-block: 1rem; }
  label { display: grid; gap: .25rem; }
  .provenance { display: grid; grid-template-columns: minmax(0, 1fr); }
  dd { margin: 0; overflow-wrap: anywhere; }
  .table-scroll { max-width: 100%; overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; }
  th, td { padding: .5rem; text-align: left; border-bottom: 1px solid #d1d5db; white-space: pre-wrap; overflow-wrap: anywhere; }
  caption { text-align: left; font-weight: 600; }
  .guidance { margin-block-start: 1rem; }
</style>
