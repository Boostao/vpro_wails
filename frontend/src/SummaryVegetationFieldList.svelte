<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import { fieldListOwner, type FieldListCell, type FieldListRow, type FieldListView,
    type SummaryVegetationFieldListSession, type FieldListOwner } from './summaryVegetationFieldList';

  let { scope, session, enabledFromPresentationDraft, peerDisabled = false, peerValidation = '',
    onStateChange }: {
    scope: FieldListOwner;
    session: SummaryVegetationFieldListSession;
    enabledFromPresentationDraft: boolean;
    peerDisabled?: boolean;
    peerValidation?: string;
    onStateChange: (view: FieldListView, close: ReturnType<SummaryVegetationFieldListSession['closeState']>) => void;
  } = $props();
  const ownedScope = untrack(() => fieldListOwner(scope));
  const retained = untrack(() => session);
  let view = $state.raw(untrack(() => retained.view()));
  let error = $state('');
  const unsubscribe = untrack(() => retained.subscribe(() => {
    view = retained.view();
    onStateChange(view, retained.closeState());
  }));
  function held() {
    return peerDisabled || !!peerValidation || view.busy || !view.dispatchSettled;
  }
  function outstanding() { return view.receipts.some(receipt => !receipt.acknowledged); }
  function canStage() {
    return !held() && enabledFromPresentationDraft && !!view.review && !view.readError && !outstanding();
  }
  function canLoad() { return !held() && (!view.actions.length || outstanding()); }
  function canAcknowledge() {
    return !held() && outstanding() && !view.readError && !view.draftError && !view.reconciliationReview &&
      view.receipts.every(receipt => receipt.acknowledged || receipt.reconciliation ||
        receipt.kind === 'known' && receipt.outcome?.verified && !receipt.outcome.errorMessage);
  }
  async function act(operation: () => unknown) {
    error = '';
    try { await operation(); } catch (cause) { error = String(cause); }
  }
  function load() { if (canLoad()) return act(() => retained.load()); }
  function save() {
    if (!held() && enabledFromPresentationDraft && retained.closeState().canSave)
      return act(() => retained.save());
  }
  function undo() { if (!held()) return act(() => retained.undo()); }
  function bulk(selected: boolean) {
    if (canStage()) return act(() => selected ? retained.stageAll() : retained.stageNone());
  }
  function editable(rowId: string) {
    const cell = view.review?.physical.rows.find(row => row.rowId === rowId)?.cells[4];
    return cell?.storage === 'integer' && (cell.integer === '0' || cell.integer === '-1');
  }
  function edit(rowId: string, raw: boolean) {
    if (canStage() && editable(rowId)) return act(() => retained.stageRow(rowId, raw));
  }
  function correct() {
    if (!held() && outstanding() && view.reconciliationReview && !view.readError)
      return act(() => retained.correctReceipt());
  }
  function acknowledge() { if (canAcknowledge()) return act(() => retained.acknowledge()); }
  function cancel() { return act(() => retained.cancel()); }
  function label(cell: FieldListCell) {
    if (cell.storage === 'null') return '[NULL Label]';
    if (cell.storage === 'text') return cell.text === '' ? '[empty Label]' : cell.text!;
    return `[${cell.storage} Label: ${cell.integer ?? cell.real ?? cell.blobHex}]`;
  }
  function rowName(row: FieldListRow) { return `${label(row.cells[2])} — physical row ${row.rowId}`; }
  function visibleRows() {
    // A Report diagnostic invalidates the combined source index, not physical Order visibility.
    return (view.proposed?.rows ?? []).filter(row => {
      const order = row.cells[0];
      return order.storage === 'integer' && order.integer !== null &&
        BigInt(order.integer) > 0n && BigInt(order.integer) <= 32767n;
    }).sort((a, b) => Number(a.cells[0].integer) - Number(b.cells[0].integer));
  }
  onDestroy(() => { unsubscribe(); retained.detach(); });
</script>

<section class="field-list" data-summary-field-list aria-label="Summary environmental field list">
  <h3>Environmental fields</h3>
  <div class="safety">
    {#if error}<p role="alert">{error}</p>{/if}
    {#if view.readError}<p role="alert">{view.readError}</p>{/if}
    {#if view.draftError}<p role="alert">{view.draftError}</p>{/if}
    {#if view.operationError}<p role="alert">{view.operationError}</p>{/if}
    {#if peerValidation}<p role="alert">Peer validation hold: {peerValidation}</p>{/if}
    {#if peerDisabled}<p role="status">Peer operation holds field-list actions.</p>{/if}
    {#if view.busy || !view.dispatchSettled}<p role="status">Field-list operation/delivery pending; wait for settlement before reconciliation.</p>{/if}
    {#each view.receipts as receipt (receipt.request.requestId)}
      <div data-field-list-receipt role={receipt.kind === 'unknown' || !receipt.outcome?.verified || receipt.error ? 'alert' : 'status'}>
        <p>Save attempt {receipt.request.requestId}: {receipt.kind};
          changed {receipt.outcome ? String(receipt.outcome.changed) : 'unknown'};
          committed {receipt.outcome ? String(receipt.outcome.committed) : 'unknown'};
          verified {receipt.outcome ? String(receipt.outcome.verified) : 'unknown'};
          replayed {receipt.outcome ? String(receipt.outcome.replayed) : 'unknown'}.
          {receipt.error}
          Acknowledged: {String(receipt.acknowledged)}; reconciliation recorded: {String(!!receipt.reconciliation)}.</p>
      </div>
    {/each}
    {#if outstanding()}<p role="status">Do not replay Save. Unknown, unverified or error outcomes require explicit Load, correction to loaded authority, then acknowledgement. Acknowledgement retains evidence.</p>{/if}
    {#if view.reconciliationReview}<p role="status">Loaded reconciliation: {view.reconciliationReview.physical.rows.length} physical rows. Historical draft remains shown until explicit correction.</p>{/if}
  </div>
  <p data-field-list-enablement>Include Environmental Fields? presentation draft: {enabledFromPresentationDraft ? 'enabled' : 'disabled'}.
    This draft flag does not change any field rows or prove report output.</p>
  {#if view.sourceIndex.error}<p role="alert">Source selection diagnostic: {view.sourceIndex.error}</p>{/if}
  {#if view.proposedIndex.error}<p role="alert">Draft selection diagnostic: {view.proposedIndex.error}</p>{/if}
  {#if view.review && view.proposed}
    <p data-field-list-counts>{view.proposed.rows.length} physical rows;
      {visibleRows().length} visible positive-order rows;
      {view.proposed.rows.length - visibleRows().length} hidden rows.
      Source selected {view.sourceIndex.error ? 'unavailable (see diagnostic)' : view.sourceIndex.selected.length};
      draft selected {view.proposedIndex.error ? 'unavailable (see diagnostic)' : view.proposedIndex.selected.length}
      (source SQL selection includes hidden rows).</p>
    <p data-field-list-bulk-warning>Select All / Select None changes Report for every physical row, including hidden rows and NULL/incompatible historical Report values.</p>
    <table>
      <caption>Environmental field selection — Label is locked, no additions</caption>
      <colgroup><col class="report-column" /><col /></colgroup>
      <thead><tr><th scope="col">Report</th><th scope="col">Field</th></tr></thead>
      <tbody>
        {#each visibleRows() as row (row.rowId)}
          <tr aria-label={rowName(row)} data-physical-row={row.rowId}>
            <td><input type="checkbox" data-source-binding="Report"
              aria-label={'Report — ' + rowName(row)}
              checked={row.cells[4].storage === 'integer' && row.cells[4].integer === '-1'}
              disabled={!canStage() || !editable(row.rowId)}
              onchange={event => { const raw = event.currentTarget.checked; edit(row.rowId, raw); }} />
              {#if !editable(row.rowId)}<small>Individual Report unavailable: original {view.review!.physical.rows.find(r => r.rowId === row.rowId)!.cells[4].storage} value.</small>{/if}
            </td>
            <th scope="row" data-source-binding="Label"><span class="literal">{label(row.cells[2])}</span>
              <small>Physical row {row.rowId}</small></th>
          </tr>
        {/each}
      </tbody>
    </table>
    <details data-field-list-physical><summary>Complete physical diagnostics ({view.proposed.rows.length} rows)</summary>
      <pre>{JSON.stringify(view.proposed, null, 2)}</pre>
    </details>
  {:else}<p role="status">Explicitly Load the owned complete field list before staging. Unloaded controls are not saved values.</p>{/if}
  <div class="actions">
    <button data-field-list-all disabled={!canStage()} onclick={() => bulk(true)}>Select All</button>
    <button data-field-list-none disabled={!canStage()} onclick={() => bulk(false)}>Select None</button>
    <button data-field-list-load disabled={!canLoad()} onclick={load}>Load field list</button>
    <button data-field-list-save disabled={held() || !enabledFromPresentationDraft || !retained.closeState().canSave}
      onclick={save}>Save field-list draft</button>
    <button data-field-list-undo disabled={held() || !view.review} onclick={undo}>Undo field-list draft</button>
    <button data-field-list-correct disabled={held() || !outstanding() || !view.reconciliationReview || !!view.readError}
      onclick={correct}>Correct to loaded authority</button>
    <button data-field-list-acknowledge disabled={!canAcknowledge()} onclick={acknowledge}>Acknowledge outcome</button>
    {#if view.busy}<button data-field-list-cancel onclick={cancel}>Cancel operation</button>{/if}
  </div>
  <p class="guidance">Owned context {ownedScope.contextId}; project {ownedScope.project}; unit {ownedScope.su}.
    Configuration-only physical field-list draft, not formatted-output parity.
    Labels remain literal read-only historical values; hidden rows remain in complete diagnostics.
    Load is observation, not proof of an unknown commit. Undo clears local actions/errors, never Save receipts.</p>
</section>

<style>
  .field-list { min-width: 0; max-width: 100%; margin: 0.75rem 0; }
  h3 { font-weight: 600; }
  .field-list p, small, summary { overflow-wrap: anywhere; }
  .safety { min-width: 0; }
  .safety [role="alert"] { color: #991b1b; margin-bottom: 0.5rem; }
  table { width: 100%; table-layout: fixed; border-collapse: collapse; }
  .report-column { width: 7rem; }
  th, td { padding: 0.5rem; text-align: left; vertical-align: top; border-bottom: 1px solid #d1d5db; overflow-wrap: anywhere; }
  input { min-width: 1.25rem; min-height: 1.25rem; }
  .literal { white-space: pre-wrap; font-weight: normal; }
  small { display: block; font-weight: normal; }
  details { min-width: 0; margin-top: 0.5rem; }
  pre { white-space: pre-wrap; overflow-wrap: anywhere; max-width: 100%; }
  .actions { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 0.5rem; margin-top: 0.5rem; }
  button { min-width: 0; min-height: 2.75rem; max-width: 100%; padding: 0.5rem 0.75rem; border: 1px solid #6b7280; border-radius: 0.25rem; overflow-wrap: anywhere; white-space: normal; }
  button:disabled { opacity: 0.55; }
  .guidance { font-size: 0.875rem; margin-top: 0.75rem; }
  @media (max-width: 800px) { .actions { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
  @media (max-width: 480px) { .actions { grid-template-columns: minmax(0, 1fr); } .report-column { width: 6rem; } }
</style>
