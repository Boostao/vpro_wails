<script lang="ts">
  import { tick, untrack } from 'svelte';
  import { TwoPageSoilService, type ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
  import SoilReferenceChoices from './SoilReferenceChoices.svelte';
  import { SoilReferenceLookup, type SoilReferenceView } from './twoPageSoilReferences';
  import { twoPageSoilSession, twoPageSoilWritingEnabled } from './twoPageSoilSessions';
  import { twoPageSoilKind, type TwoPageSoilForm } from './twoPageSoilEditor';
  import { equalCell, metadataCellText } from './projectMetadataEditor';
  import { soilFields } from './soilChildEditor';
  import { enterNavigationMode } from './enterNavigation';
  import { twoPageSoilGroups, twoPageSoilCell, twoPageSoilStoredText, twoPageSoilRowDiagnostic, twoPageSoilDepthOrder } from './twoPageSoilPresentation';

  let { contextId, project, plot, form, parentBlocked }: {
    contextId: string; project: string; plot: string; form: TwoPageSoilForm; parentBlocked: boolean;
  } = $props();
  const session = $derived(twoPageSoilSession({ contextId, project, plot, form }));
  let revision = $state(0);
  let selected = $state<string | null>(untrack(() => {
    const original = session.view().original;
    return original ? twoPageSoilDepthOrder(original).rows[0]?.rowId ?? null : null;
  }));
  let error = $state<string | null>(null);
  let panel = $state<HTMLElement>();
  let referenceView = $state.raw<SoilReferenceView>({ busy: false, data: null, error: null });
  let referenceLookup = $state.raw<SoilReferenceLookup | null>(null);
  $effect(() => {
    const owner = { contextId, project, plot, form };
    const lookup = new SoilReferenceLookup(owner,
      owner => TwoPageSoilService.GetReferences(owner.contextId, owner.plot, owner.form),
      next => { referenceView = next; });
    untrack(() => { referenceLookup = lookup; referenceView = lookup.snapshot(); });
    return () => lookup.dispose();
  });
  $effect(() => session.subscribe(() => untrack(() => revision++)));
  const view = $derived.by(() => { revision; return session.view(); });
  const close = $derived.by(() => { revision; return session.closeState(); });
  const depthOrder = $derived(view.original ? twoPageSoilDepthOrder(view.original) : null);
  const row = $derived(depthOrder?.rows.find(row => row.rowId === selected) ?? depthOrder?.rows[0]);
  const diagnostic = $derived(view.original && row ? twoPageSoilRowDiagnostic(view.original, row.rowId) : null);
  const fieldsDisabled = $derived(parentBlocked || !twoPageSoilWritingEnabled || view.busy || referenceView.busy ||
    view.blocked || view.acknowledgementRequired || !!diagnostic);

  async function focusHorizon(plannedRow = row?.rowId, owned = session): Promise<boolean> {
    await tick();
    if (!panel || owned !== session || plannedRow !== row?.rowId) return false;
    if (panel.dataset.twoPageSoilPanel !== form || panel.dataset.soilCurrentRow !== plannedRow) {
      throw new Error('Observed soil panel/physical record changed before Horizon focus.');
    }
    const input = panel?.querySelector<HTMLInputElement>('[data-soil-column="Horizon"] input[type="text"]');
    if (!input || input.disabled || !input.checkVisibility()) return false;
    input.focus();
    if (document.activeElement !== input) throw new Error('The observed soil Horizon did not receive focus; no record was saved or created.');
    return true;
  }
  async function selectRow(rowId: string) {
    try {
      if (!depthOrder?.rows.some(row => row.rowId === rowId)) throw new Error('The selected physical soil record is unavailable.');
      selected = rowId;
      await focusHorizon(rowId);
    } catch (cause) { error = `Soil selection/focus refused without mutation: ${String(cause)}`; }
  }
  async function enterRecord(event: KeyboardEvent, column: string) {
    if (event.key !== 'Enter' || event.defaultPrevented || event.isComposing || event.repeat ||
        event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
    event.preventDefault();
    try {
      if (fieldsDisabled || close.blocked || !row || !view.original || !panel || enterNavigationMode(form) !== 'record') {
        throw new Error('Correct retained validation and resolve parent/soil operations before source record navigation.');
      }
      const control = event.currentTarget;
      if (!(control instanceof HTMLInputElement || control instanceof HTMLTextAreaElement) || control !== event.target ||
          document.activeElement !== control || !panel.contains(control) || !control.checkVisibility() || control.disabled ||
          control.closest<HTMLElement>('[data-soil-column]')?.dataset.soilColumn !== column ||
          panel.dataset.twoPageSoilPanel !== form || panel.dataset.soilCurrentRow !== row.rowId) {
        throw new Error('Observed Enter focus/control/physical owner differs from its planned soil field.');
      }
      const draft = view.drafts[JSON.stringify([row.rowId, column])];
      const source = twoPageSoilCell(view.original, row.rowId, column);
      const nullValue = draft?.nullValue ?? source.storage === 'null';
      const raw = nullValue ? '' : draft?.raw ?? metadataCellText(source);
      // Compare browser display only; retain original tagged bytes and unchanged assignments.
      const displayed = control instanceof HTMLTextAreaElement ? raw.replace(/\r\n|\r/g, '\n') : raw.replace(/[\r\n]/g, '');
      if (control.value !== displayed) throw new Error('Observed soil value differs from its retained raw/original display.');
      const current = depthOrder?.rows.findIndex(candidate => candidate.rowId === row.rowId) ?? -1;
      const next = current >= 0 ? depthOrder?.rows[current + 1] : undefined;
      if (!next) throw new Error('No next existing soil record. Creation and automatic Save are unavailable.');
      error = null;
      selected = next.rowId;
      if (!await focusHorizon(next.rowId)) throw new Error('Next physical soil record is selected, but its Horizon is read only or no longer visible; no Save was dispatched.');
    } catch (cause) { error = `Source soil Enter refused without mutation: ${String(cause)}`; }
  }
  async function loadReferences() {
    try {
      if (parentBlocked || view.busy || view.blocked || view.acknowledgementRequired || !referenceLookup) {
        throw new Error('Resolve parent/soil ownership before loading source choices.');
      }
      await referenceLookup.refresh();
    } catch (cause) { error = `Source soil choice read refused: ${String(cause)}`; }
  }
  function chooseReference(column: string, plannedRow: string, definitionId: string, value: ProjectMetadataCell) {
    try {
      if (!row || row.rowId !== plannedRow || panel?.dataset.soilCurrentRow !== plannedRow ||
          panel.dataset.twoPageSoilPanel !== form ||
          referenceView.data?.contextId !== contextId || referenceView.data.project !== project ||
          referenceView.data.plot !== plot || referenceView.data.form !== form) {
        throw new Error('Observed soil choice owner/physical row differs from its planned field.');
      }
      const observed = referenceView.data.fields.find(field => field.column === column)
        ?.definitions.rows.find(definition => definition.rowId === definitionId)?.cells[3];
      if (!observed || !equalCell(observed, value)) throw new Error('Observed source definition identity/value differs from the planned soil choice.');
      if (value.storage !== 'text' && value.storage !== 'null') throw new Error('Source choice Item must be explicit TEXT or NULL.');
      stage(column, value.text ?? '', value.storage === 'null');
    } catch (cause) { error = `Source soil choice refused without mutation: ${String(cause)}`; }
  }
  async function operation(action: 'load' | 'save' | 'recover' | 'undo' | 'acknowledge' | 'cancel') {
    try {
      if ((parentBlocked || referenceView.busy) && (action === 'load' || action === 'save')) {
        throw new Error('Finish actual parent drafts/operations and unlock the plot before loading or saving linked soil.');
      }
      error = null;
      const owned = session;
      if (action === 'cancel') owned.cancel();
      else if (action === 'undo') owned.undo();
      else if (action === 'acknowledge') owned.acknowledge();
      else {
        const ok = action === 'load' ? await owned.load()
          : action === 'save' ? await owned.save(crypto.randomUUID()) : await owned.resolve();
        if (!ok) error = owned.view().error;
        else if (action === 'load' && owned === session) {
          selected = row?.rowId ?? null;
          await focusHorizon(selected ?? undefined, owned);
        }
      }
    } catch (cause) { error = `Linked soil operation refused; owned state retained: ${String(cause)}`; }
  }
  function stage(column: string, raw: string, nullValue: boolean) {
    try {
      if (fieldsDisabled || !row) throw new Error('Linked soil staging is unavailable while parent ownership, Lock or the retained soil operation blocks it.');
      session.stage(row.rowId, column, raw, nullValue); error = null;
    } catch (cause) { error = `Linked soil draft refused; owned state retained: ${String(cause)}`; }
  }
</script>

<section bind:this={panel} class="min-w-0 space-y-3 rounded border bg-white p-3" aria-label={`${form} linked soil`} data-two-page-soil-panel={form} data-soil-current-row={row?.rowId}>
  <h4 class="font-semibold">{form === 'SoilHumusXL' ? 'Humus' : 'Mineral soil'}</h4>
  {#if parentBlocked}<p role="status">Finish actual parent drafts/operations and unlock the plot before new soil staging.</p>{/if}
  {#if error}<p role="alert" class="break-words text-sm text-red-700">{error}</p>{/if}
  {#if view.error && view.error !== error}<p role="alert" class="break-words text-sm text-red-700">{view.error}</p>{/if}
  {#if referenceView.error}<p role="alert" class="break-words text-sm text-red-700">{referenceView.error}</p>{/if}
  {#if referenceView.busy}<p role="status">Reading current source soil definitions...</p>{/if}
  {#if view.blocked}<p role="alert">Save outcome unresolved. Recover the exact retained request; do not repeat Save.</p>{/if}
  {#if view.noCommitCertificate}<p role="status">Server certified no commit for {view.noCommitCertificate.requestId}. Drafts remain for correction or Undo; another Save needs a new request.</p>{/if}
  {#if Object.keys(view.drafts).length}<p role="status">{Object.keys(view.drafts).length} retained field draft(s). Save applies all retained edits together, including other physical rows.</p>{/if}
  <div class="flex flex-wrap gap-2">
    <button type="button" disabled={parentBlocked || close.unsaved || view.busy || referenceView.busy || close.blocked} onclick={() => operation('load')}>Load {twoPageSoilKind(form)}</button>
    <button type="button" disabled={parentBlocked || referenceView.busy || !close.canSave} onclick={() => operation('save')}>Save {twoPageSoilKind(form)}</button>
    <button type="button" disabled={parentBlocked || view.busy || referenceView.busy || view.blocked || view.acknowledgementRequired}
      onclick={loadReferences}>{referenceView.error ? 'Retry' : 'Load'} source soil choices</button>
    {#if referenceView.busy}<button type="button" onclick={() => referenceLookup?.cancel()}>Cancel source soil choices</button>{/if}
    <button type="button" disabled={view.busy || view.blocked || view.acknowledgementRequired || !Object.keys(view.drafts).length} onclick={() => operation('undo')}>Undo soil drafts</button>
    {#if view.busy}<button type="button" onclick={() => operation('cancel')}>Cancel soil operation</button>{/if}
    {#if view.blocked}<button type="button" disabled={view.busy} onclick={() => operation('recover')}>Recover soil outcome (read only)</button>{/if}
    {#if view.acknowledgementRequired}<button type="button" disabled={view.busy} onclick={() => operation('acknowledge')}>Acknowledge committed soil receipt</button>{/if}
  </div>
  {#if view.original}
    {#if depthOrder?.diagnostic}<p role="alert">{depthOrder.diagnostic}</p>
    {:else}<p role="status">Depth Up {form === 'SoilHumusXL' ? 'descending (NULL last)' : 'ascending (NULL first)'}. Ties retain physical reader order.</p>{/if}
    <div class="flex flex-wrap gap-2" aria-label={`${form} physical rows`}>
      {#each depthOrder?.rows ?? [] as physical (physical.rowId)}
        <button type="button" class="min-w-0 break-words rounded border p-2 text-left" aria-pressed={row?.rowId === physical.rowId}
          onclick={() => selectRow(physical.rowId)}>
          Horizon {view.original.records.columns.some(column => column.name === 'Horizon') ? twoPageSoilStoredText(view.original, physical.rowId, 'Horizon') : '(source column unavailable)'};
          Depth Up {view.original.records.columns.some(column => column.name === 'UpperDepth') ? twoPageSoilStoredText(view.original, physical.rowId, 'UpperDepth') : '(source column unavailable)'};
          ID {twoPageSoilStoredText(view.original, physical.rowId, 'ID')}; physical row {physical.rowId}
        </button>
      {:else}<p role="status">No physical soil records for this exact parent. Creation is unavailable.</p>{/each}
    </div>
    {#if row}
      <p class="break-words text-sm">Application ID {twoPageSoilStoredText(view.original, row.rowId, 'ID')}; physical row {row.rowId}; parent {plot}</p>
      {#if diagnostic}<p role="alert">{diagnostic}</p>{/if}
      {#each twoPageSoilGroups[twoPageSoilKind(form)] as group}
        <fieldset class="min-w-0 rounded border p-3">
          <legend class="text-sm font-semibold">{group.label}</legend>
          <div class="grid min-w-0 grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
            {#each group.fields as [column, label]}
              {@const index = view.original.records.columns.findIndex(field => field.name === column)}
              {#if index >= 0}
                {@const source = twoPageSoilCell(view.original, row.rowId, column)}
                {@const draft = view.drafts[JSON.stringify([row.rowId, column])]}
                {@const raw = draft?.raw ?? (source.storage === 'null' ? '' : metadataCellText(source))}
                {@const nullValue = draft?.nullValue ?? source.storage === 'null'}
                {@const id = `${form}-${row.rowId}-${column}`}
                {@const memo = soilFields[twoPageSoilKind(form)].find(field => field.column === column)?.kind === 'memo'}
                {@const reference = referenceView.data?.fields.find(field => field.column === column)}
                {@const plannedRow = row.rowId}
                <div class={`min-w-0 space-y-1 ${memo ? 'sm:col-span-2' : ''}`} data-soil-column={column}>
                  <label class="block text-sm font-medium" for={id}>{label}</label>
                  {#if memo}
                    <textarea {id} class="w-full min-w-0 rounded border p-2" rows="3" value={nullValue ? '' : raw} disabled={fieldsDisabled} placeholder={nullValue ? 'NULL' : undefined}
                      aria-invalid={!!draft?.error} aria-describedby={draft?.error ? `${id}-error` : undefined}
                      onkeydown={event => enterRecord(event, column)}
                      oninput={event => stage(column, event.currentTarget.value, false)}></textarea>
                  {:else}
                    <input {id} class="w-full min-w-0 rounded border p-2" type="text" value={nullValue ? '' : raw} disabled={fieldsDisabled} placeholder={nullValue ? 'NULL' : undefined}
                      aria-invalid={!!draft?.error} aria-describedby={draft?.error ? `${id}-error` : undefined}
                      onkeydown={event => enterRecord(event, column)}
                      oninput={event => stage(column, event.currentTarget.value, false)} />
                  {/if}
                  <label class="flex items-center gap-2 text-sm">
                    <input type="checkbox" checked={nullValue} disabled={fieldsDisabled} onchange={event => stage(column, raw, event.currentTarget.checked)} />
                    {label}: NULL (not empty text)
                  </label>
                  {#if draft?.error}<p id={`${id}-error`} role="alert" class="text-sm text-red-700">{draft.error}</p>{/if}
                  {#if !draft}<p class="text-xs text-gray-600">Stored {source.storage}</p>{/if}
                  {#if reference}<SoilReferenceChoices field={reference} {label} disabled={fieldsDisabled}
                    onchoose={(definitionId, value) => chooseReference(column, plannedRow, definitionId, value)} />{/if}
                </div>
              {:else}<p role="alert">Source field {column} unavailable; row is read only.</p>{/if}
            {/each}
          </div>
        </fieldset>
      {/each}
    {/if}
  {/if}
  {#each Object.values(view.drafts).filter(draft => draft.error && draft.rowId !== row?.rowId) as draft}
    <p role="alert">Physical row {draft.rowId}, {draft.column}: {draft.error}. Select that row to correct it or Undo.</p>
  {/each}
  {#if view.receipts.length}<p role="status">{view.receipts.length} immutable committed soil receipt(s) retained across remounts.</p>{/if}
  <p class="text-xs text-gray-600">Existing-row experimental review only. Numeric/NULL depth direction follows the source form; stable ties and historical nonnumeric physical order are explicit desktop adaptations. Enter moves to the next existing record's Horizon without Save; modifiers keep normal field behavior. Source choices use current configured definitions and stage through ordinary soil validation; creation, deletion and restoration remain unavailable. NULL is explicit; typing replaces NULL without completing or trimming text. Save is explicit and never dispatched by changing rows or choosing a definition.</p>
</section>
