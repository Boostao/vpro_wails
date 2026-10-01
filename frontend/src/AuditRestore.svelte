<script lang="ts" module>
  import type { AuditEntry } from '../bindings/github.com/boostao/vpro-wails';

  const adminColumns = new Set(['officenotes', 'startdate', 'becsiteunit', 'enteredby', 'siteplotquality', 'soilplotquality', 'usersiteunit', 'vegplotquality', 'humusthickness', 'updatedfromcards']);
  const headerAliases: Record<string, string> = {
    sitesurveyor: 'surveyor', fieldnumber: 'fieldNo', location: 'generalLocation', ntsmapsheet: 'mapSheet',
    utmeasting: 'easting', utmnorthing: 'northing', locationaccuracy: 'accuracy', transdistrib: 'transition',
    successionalstatus: 'successional', slopegradient: 'slope', mesoslopeposition: 'mesoSlopePos',
    surfacetopographytype: 'microtopType', surfacetopographysize: 'microtopSize', sitenotes: 'fieldNotes'
  };
  export type AuditCapabilities = {
    header: Record<string, boolean | undefined>;
    children: Record<string, Record<string, boolean | undefined>>;
  };

  export function exactAuditRowId(value: string): boolean {
    if (!/^-?(0|[1-9]\d*)$/.test(value)) return false;
    const id = BigInt(value);
    return id.toString() === value && id >= -(1n << 63n) && id < (1n << 63n);
  }

  export function auditRestoreReason(entry: AuditEntry, project: string, plot: string, capabilities: AuditCapabilities): string | null {
    if (!exactAuditRowId(entry.rowId)) return 'Invalid exact audit row identity.';
    if (entry.project !== project || entry.plotNumber !== plot) return 'Audit row belongs to another project or plot.';
    const table = ['Env', 'Admin', 'Veg', 'Humus', 'Mineral', 'Other'].find(name =>
      entry.table.toLowerCase() === `_${name}`.toLowerCase() ||
      entry.table.toLowerCase() === `${project}_${name}`.toLowerCase());
    if (!table) return 'Unsupported audit table.';
    const column = entry.editField.toLowerCase();
    if (['id', 'plotnumber', 'plot', 'locked'].includes(column)) return 'Identity and runtime fields cannot be restored.';
    if (table === 'Veg' && column.startsWith('cover')) return 'Cover* restoration is unsupported; its audit history will not be pruned.';
    let fields: Record<string, boolean | undefined>;
    let property = column;
    if (table === 'Env' || table === 'Admin') {
      if (entry.id !== null) return 'Parent audit rows must not have a child ID.';
      if (!capabilities.header.plotNumber) return 'Verified parent identity columns are unavailable.';
      if (adminColumns.has(column) !== (table === 'Admin')) return 'Field is not mapped to this parent table.';
      fields = capabilities.header;
      property = headerAliases[column] ?? column;
    } else {
      if (entry.id === null || !Number.isInteger(entry.id) || entry.id < -2147483648 || entry.id > 2147483647) return 'A supported signed32 child ID is required.';
      fields = capabilities.children[table] ?? {};
      if (!fields.id || !fields.plotNumber) return 'Verified child identity columns are unavailable.';
      if (table === 'Humus' && column === 'humusformph') property = 'ph';
    }
    return Object.entries(fields).some(([key, supported]) => supported === true && key.toLowerCase() === property.toLowerCase())
      ? null : 'Field is unsupported by the active project schema.';
  }
</script>

<script lang="ts">
  import { AuditRestoreAction } from '../bindings/github.com/boostao/vpro-wails';
  let { entries, project, plot, capabilities, enabled, disabled, gateReason, revision, onrestore }: {
    entries: AuditEntry[]; project: string; plot: string; capabilities: AuditCapabilities;
    enabled: boolean; disabled: boolean; gateReason: string; revision: number;
    onrestore: (rowIds: string[], action: AuditRestoreAction) => Promise<void>;
  } = $props();
  let selected = $state<string[]>([]);
  let confirming = $state(false);
  let submitting = $state(false);
  let dialog: HTMLDialogElement;
  const reasons = $derived(new Map(entries.map(entry => [entry.rowId, auditRestoreReason(entry, project, plot, capabilities)])));
  const selectionInvalid = $derived(selected.some(id => !reasons.has(id) || reasons.get(id) !== null));
  const blocked = $derived(!enabled || disabled || submitting);

  $effect(() => {
    void revision; void project; void plot;
    selected = [];
    confirming = false;
  });
  $effect(() => {
    if (blocked && !submitting) confirming = false;
  });
  $effect(() => {
    if (confirming && !dialog.open) dialog.showModal();
    else if (!confirming && dialog.open) dialog.close();
  });

  function toggle(id: string, checked: boolean) {
    if (blocked || reasons.get(id) !== null) return;
    selected = checked ? [...selected, id] : selected.filter(value => value !== id);
  }
  async function respond(action: AuditRestoreAction) {
    if (submitting || (action !== AuditRestoreAction.AuditRestoreCancel && (blocked || selectionInvalid || selected.length === 0))) return;
    const ids = [...selected];
    submitting = true;
    try {
      await onrestore(ids, action);
    } finally {
      submitting = false;
      selected = [];
      confirming = false;
    }
  }
</script>

<section class="audit-restore" aria-label="Selective audit restoration">
  <h3>Audit History ({entries.length})</h3>
  <p>Restore selected stored fields, not whole records. NULL clears a value. Vegetation records are never automatically deleted.</p>
  {#if !enabled}<p role="status">Restoration is disabled by application configuration.</p>
  {:else if disabled}<p role="status">{gateReason}</p>{/if}
  <div class="actions">
    <span aria-live="polite">{selected.length} selected</span>
    <button type="button" disabled={blocked || selected.length === 0 || selectionInvalid} onclick={() => confirming = true}>Restore selected fields…</button>
    <button type="button" disabled={blocked || selected.length === 0} onclick={() => selected = []}>Clear selection</button>
  </div>
  <div class="table-scroll">
    <table>
      <caption>Audit rows for {project} / {plot || 'unsaved plot'}. Selection uses exact audit row IDs.</caption>
      <thead><tr><th>Select</th><th>Audit row ID</th><th>Timestamp / User</th><th>Table / Child ID</th><th>Field</th><th>Before</th><th>After</th><th>Availability</th></tr></thead>
      <tbody>
        {#each entries as entry (entry.rowId)}
          {@const reason = reasons.get(entry.rowId)}
          <tr data-audit-row-id={entry.rowId}>
            <td><input type="checkbox" aria-label={`Select audit row ${entry.rowId} for ${entry.editField}`}
              checked={selected.includes(entry.rowId)} disabled={blocked || reason !== null}
              title={reason ?? 'Select this field restoration'} onchange={(event) => toggle(entry.rowId, event.currentTarget.checked)} /></td>
            <td><code>{entry.rowId}</code></td>
            <td>{entry.editWhen}<br />{entry.user}</td>
            <td>{entry.table}<br />{entry.id === null ? 'Parent' : `Child ID ${entry.id}`}</td>
            <td>{entry.editField}</td>
            <td>{entry.beforeEdit === null ? 'NULL' : entry.beforeEdit === '' ? '(empty string)' : entry.beforeEdit}</td>
            <td>{entry.afterEdit === null ? 'NULL' : entry.afterEdit === '' ? '(empty string)' : entry.afterEdit}</td>
            <td>{reason ?? 'Available; backend validates current values and ownership before restoring.'}</td>
          </tr>
        {:else}<tr><td colspan="8">No audit log records for this plot.</td></tr>{/each}
      </tbody>
    </table>
  </div>
</section>
<dialog bind:this={dialog} aria-labelledby="audit-confirm-title" aria-describedby="audit-confirm-description"
  oncancel={(event) => { event.preventDefault(); void respond(AuditRestoreAction.AuditRestoreCancel); }}>
  <h2 id="audit-confirm-title">Restore {selected.length} selected audit {selected.length === 1 ? 'field' : 'fields'}?</h2>
  <p id="audit-confirm-description">Only the selected fields change. No vegetation records are deleted. Retain keeps the selected history; prune permanently removes it after successful restoration.</p>
  <div class="actions">
    <button type="button" disabled={submitting} onclick={() => respond(AuditRestoreAction.AuditRestoreCancel)}>Cancel</button>
    <button type="button" disabled={blocked || selectionInvalid} onclick={() => respond(AuditRestoreAction.AuditRestoreRetain)}>Restore and retain history</button>
    <button type="button" disabled={blocked || selectionInvalid} onclick={() => respond(AuditRestoreAction.AuditRestorePrune)}>Restore and prune selected history</button>
  </div>
</dialog>

<style>
  .audit-restore { font-size: 0.8rem; }
  h3, h2 { font-weight: 700; margin-bottom: 0.5rem; }
  p { margin: 0.4rem 0; }
  .actions { display: flex; flex-wrap: wrap; gap: 0.7rem; align-items: center; margin: 0.75rem 0; }
  button { padding: 0.45rem 0.7rem; border: 1px solid #a8a29e; border-radius: 0.3rem; background: #fff; }
  button:disabled { opacity: 0.5; }
  button:focus-visible, input:focus-visible { outline: 3px solid #047857; outline-offset: 2px; }
  .table-scroll { overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; }
  caption { text-align: left; margin: 0.5rem 0; }
  th, td { text-align: left; padding: 0.5rem; border-bottom: 1px solid #e7e5e4; vertical-align: top; }
  th { background: #f5f5f4; }
  dialog { max-width: 38rem; padding: 1.5rem; border: 1px solid #a8a29e; border-radius: 0.5rem; }
  dialog::backdrop { background: #0007; }
</style>
