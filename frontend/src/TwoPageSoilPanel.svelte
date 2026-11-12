<script lang="ts">
  import { tick, untrack } from 'svelte';
  import { PlotEntryService, type ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
  import SoilReferenceChoices from './SoilReferenceChoices.svelte';
  import { SoilReferenceLookup, type SoilReferenceView } from './twoPageSoilReferences';
  import { twoPageSoilSession, twoPageSoilWritingEnabled, twoPageSoilCreationEnabled } from './twoPageSoilSessions';
  import { twoPageSoilKind, type TwoPageSoilForm } from './twoPageSoilEditor';
  import { equalCell, metadataCellText } from './projectMetadataEditor';
  import { soilFields } from './soilChildEditor';
  import { enterNavigationMode } from './enterNavigation';
  import { twoPageSoilGroups, twoPageSoilCell, twoPageSoilStoredText, twoPageSoilRowDiagnostic, twoPageSoilDepthOrder } from './twoPageSoilPresentation';

  interface SoilDeleteRestorePort {
    deleteHorizon(
      contextId: string,
      req: { requestId: string; plotNumber: string; form: string; id: number; rowId?: string }
    ): Promise<{ requestId: string; deletedId: number; horizon: string }>;
    restoreHorizon(
      contextId: string,
      req: { requestId: string; plotNumber: string; form: string; deleteRequestId: string }
    ): Promise<{ requestId: string; restoredId: number; restoredRowId: string; horizon: string }>;
    listDeletedHorizons(
      contextId: string,
      plotNumber: string,
      form: string
    ): Promise<
      Array<{
        deleteRequestId: string;
        plotNumber: string;
        form: string;
        kind: string;
        deletedId: number;
        horizon: string;
        upperDepth?: number;
        lowerDepth?: number;
        deletedBy: string;
        deletedAt: string;
        restored: boolean;
      }>
    >;
  }

  let { soilDeleteRestorePort, contextId, project, plot, form, parentBlocked }: {
    soilDeleteRestorePort?: SoilDeleteRestorePort;
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
  let deleteConfirmRowId = $state<string | null>(null);
  let deleteRestoreBusy = $state(false);
  let deletedHistoryOpen = $state(false);
  let deletedHorizons = $state<
    Array<{
      deleteRequestId: string;
      plotNumber: string;
      form: string;
      kind: string;
      deletedId: number;
      horizon: string;
      upperDepth?: number;
      lowerDepth?: number;
      deletedBy: string;
      deletedAt: string;
      restored: boolean;
    }>
  >([]);
  let deleteRestoreGeneration = 0;

  async function resolveSoilDeleteRestorePort(): Promise<SoilDeleteRestorePort> {
    if (soilDeleteRestorePort) return soilDeleteRestorePort;
    const runtime = await import('@wailsio/runtime');
    return {
      deleteHorizon: (cid, req) => runtime.Call.ByName('main.ContextService.DeleteTwoPageSoilHorizon', cid, req),
      restoreHorizon: (cid, req) => runtime.Call.ByName('main.ContextService.RestoreTwoPageSoilHorizon', cid, req),
      listDeletedHorizons: (cid, p, f) => runtime.Call.ByName('main.ContextService.ListTwoPageSoilDeletedHorizons', cid, p, f),
    };
  }

  async function refreshDeletedHorizons() {
    if (parentBlocked || view.busy || deleteRestoreBusy) return;
    const gen = ++deleteRestoreGeneration;
    deleteRestoreBusy = true;
    error = null;
    try {
      const port = await resolveSoilDeleteRestorePort();
      const list = await port.listDeletedHorizons(contextId, plot, form);
      if (gen !== deleteRestoreGeneration) return;
      deletedHorizons = list ?? [];
      deletedHistoryOpen = true;
    } catch (cause) {
      if (gen === deleteRestoreGeneration) {
        error = `Deleted soil horizon history unavailable: ${String(cause)}`;
      }
    } finally {
      if (gen === deleteRestoreGeneration) deleteRestoreBusy = false;
    }
  }

  async function confirmDeleteHorizon() {
    if (parentBlocked || referenceView.busy || view.busy || close.unsaved || close.blocked || deleteRestoreBusy || !view.original || !row || deleteConfirmRowId !== row.rowId) {
      error = 'Resolve retained drafts and confirm the selected physical row before deleting a horizon.';
      return;
    }
    const idText = twoPageSoilStoredText(view.original, row.rowId, 'ID');
    const parsedId = Number(idText);
    if (!Number.isInteger(parsedId) || parsedId === 0) {
      error = 'Soil horizon deletion requires a non-zero integer application ID.';
      return;
    }
    const req = $state.snapshot({
      requestId: crypto.randomUUID(),
      plotNumber: plot,
      form,
      id: parsedId,
      rowId: row.rowId,
    });
    const owned = session;
    const gen = ++deleteRestoreGeneration;
    deleteRestoreBusy = true;
    error = null;
    try {
      const port = await resolveSoilDeleteRestorePort();
      await port.deleteHorizon(contextId, req);
      if (gen !== deleteRestoreGeneration || owned !== session) return;
      deleteConfirmRowId = null;
      await owned.load();
      if (owned === session) {
        selected = depthOrder?.rows[0]?.rowId ?? null;
      }
      if (deletedHistoryOpen) {
        const list = await port.listDeletedHorizons(contextId, plot, form);
        if (gen === deleteRestoreGeneration) deletedHorizons = list ?? [];
      }
    } catch (cause) {
      if (gen === deleteRestoreGeneration) error = `Delete soil horizon refused: ${String(cause)}`;
    } finally {
      if (gen === deleteRestoreGeneration) deleteRestoreBusy = false;
    }
  }

  async function restoreDeletedHorizon(deleteRequestId: string) {
    if (parentBlocked || referenceView.busy || view.busy || close.unsaved || close.blocked || deleteRestoreBusy) {
      error = 'Save or Undo retained soil drafts before restoring a deleted horizon.';
      return;
    }
    const req = $state.snapshot({
      requestId: crypto.randomUUID(),
      plotNumber: plot,
      form,
      deleteRequestId,
    });
    const owned = session;
    const gen = ++deleteRestoreGeneration;
    deleteRestoreBusy = true;
    error = null;
    try {
      const port = await resolveSoilDeleteRestorePort();
      const restored = await port.restoreHorizon(contextId, req);
      if (gen !== deleteRestoreGeneration || owned !== session) return;
      await owned.load();
      if (owned === session && restored.restoredRowId) {
        selected = restored.restoredRowId;
      }
      const list = await port.listDeletedHorizons(contextId, plot, form);
      if (gen === deleteRestoreGeneration) deletedHorizons = list ?? [];
    } catch (cause) {
      if (gen === deleteRestoreGeneration) error = `Restore soil horizon refused: ${String(cause)}`;
    } finally {
      if (gen === deleteRestoreGeneration) deleteRestoreBusy = false;
    }
  }
  $effect(() => {
    const owner = { contextId, project, plot, form };
    const lookup = new SoilReferenceLookup(owner,
      owner => PlotEntryService.GetTwoPageSoilReferences(owner.contextId, owner.plot, owner.form),
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
    view.blocked || view.acknowledgementRequired || !!diagnostic || !!view.creation);

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
  function beginCreation() {
    try {
      if (parentBlocked || referenceView.busy) throw new Error('Resolve parent/source-choice operations before Add Horizon.');
      session.beginCreation(); error = null;
    } catch (cause) { error = `Add Horizon refused without mutation: ${String(cause)}`; }
  }
  function stageCreation(raw: string) {
    try {
      if (parentBlocked || referenceView.busy) throw new Error('Resolve parent/source-choice operations before staging a new Horizon.');
      session.stageCreation(raw); error = null;
    } catch (cause) { error = `New Horizon draft refused: ${String(cause)}`; }
  }
  async function createHorizon() {
    const owned = session;
    try {
      if (parentBlocked || referenceView.busy || !panel || panel.dataset.twoPageSoilPanel !== form) {
        throw new Error('Observed creation owner is unavailable.');
      }
      const input = panel.querySelector<HTMLInputElement>('[data-soil-new-horizon]');
      if (!input || input.disabled || !input.checkVisibility() || input.value !== view.creation?.raw) {
        throw new Error('Observed new Horizon differs from its retained literal draft.');
      }
      error = null;
      const ok = await owned.create(crypto.randomUUID());
      if (owned !== session) return;
      if (!ok) error = owned.view().error;
      else {
        const receipt = owned.view().receipts.at(-1);
        if (receipt?.history.CreatedRowID) selected = receipt.history.CreatedRowID;
      }
    } catch (cause) {
      if (owned === session) error = `Explicit Horizon creation refused; retained state preserved: ${String(cause)}`;
    }
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
  function soilHelpId(soilForm: TwoPageSoilForm, column: string): string {
    const lower = column.toLowerCase();
    if (soilForm === 'SoilHumusXL') {
      if (lower === 'horizon') return 'humus.horizon';
      if (lower === 'upperdepth' || lower === 'lowerdepth') return 'humus.upper_lower_depth';
      if (lower === 'vonpost') return 'humus.von_post';
      return 'humus.humus_form';
    }
    if (lower === 'horizon' || lower === 'upperdepth' || lower === 'lowerdepth') return 'mineral.horizon';
    return 'mineral.texture_cf';
  }
</script>

<section bind:this={panel} class="min-w-0 space-y-3 rounded border bg-white p-3" aria-label={`${form} linked soil`} data-two-page-soil-panel={form} data-soil-current-row={row?.rowId} data-help-id={form === 'SoilHumusXL' ? 'humus.upper_lower_depth' : 'mineral.horizon'}>
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
    {#if twoPageSoilCreationEnabled}
      <button type="button" disabled={parentBlocked || referenceView.busy || view.busy || close.unsaved || close.blocked || !view.original}
        onclick={beginCreation}>New {twoPageSoilKind(form)} horizon</button>
    {/if}
    {#if twoPageSoilCreationEnabled || soilDeleteRestorePort}
      <button type="button" data-soil-delete-trigger disabled={parentBlocked || referenceView.busy || view.busy || deleteRestoreBusy || close.unsaved || close.blocked || !view.original || !row || !!diagnostic}
        onclick={() => { deleteConfirmRowId = row?.rowId ?? null; error = null; }}>Delete selected {twoPageSoilKind(form)} horizon</button>
      <button type="button" data-soil-deleted-history-trigger disabled={parentBlocked || view.busy || deleteRestoreBusy || close.unsaved || close.blocked}
        onclick={refreshDeletedHorizons}>Deleted {twoPageSoilKind(form)} horizons</button>
    {/if}
    <button type="button" disabled={view.busy || view.blocked || view.acknowledgementRequired || !Object.keys(view.drafts).length && !view.creation} onclick={() => operation('undo')}>Undo soil drafts</button>
    {#if view.busy}<button type="button" onclick={() => operation('cancel')}>Cancel soil operation</button>{/if}
    {#if view.blocked}<button type="button" disabled={view.busy} onclick={() => operation('recover')}>Recover soil outcome (read only)</button>{/if}
    {#if view.acknowledgementRequired}<button type="button" disabled={view.busy} onclick={() => operation('acknowledge')}>Acknowledge committed soil receipt</button>{/if}
  </div>
  {#if deleteConfirmRowId && row && deleteConfirmRowId === row.rowId && view.original}
    <fieldset class="min-w-0 space-y-2 rounded border border-red-300 border-t-[3px] border-t-[#FCBA19] bg-red-50/40 p-3" data-soil-delete-confirm>
      <legend class="text-sm font-semibold text-[#003366]">Confirm guarded {twoPageSoilKind(form)} horizon deletion</legend>
      <p class="text-xs text-slate-800">
        Delete Horizon <span class="font-mono font-semibold">{twoPageSoilStoredText(view.original, row.rowId, 'Horizon')}</span>
        (ID <span class="font-mono font-semibold">{twoPageSoilStoredText(view.original, row.rowId, 'ID')}</span>) from plot <span class="font-mono">{plot}</span>?
        Its child ID remains permanently reserved in <span class="font-mono">__VPRO_ChildIdentity</span> and full provenance is recorded in <span class="font-mono">{project}_Audit</span> and <span class="font-mono">__VPRO_TwoPageSoilHistory</span>.
      </p>
      <div class="flex flex-wrap gap-2">
        <button type="button" data-soil-confirm-delete-btn disabled={deleteRestoreBusy || view.busy}
          class="rounded bg-[#003366] px-3 py-1.5 text-xs font-semibold text-white hover:bg-[#002244]"
          onclick={confirmDeleteHorizon}>Confirm delete horizon</button>
        <button type="button" disabled={deleteRestoreBusy}
          class="rounded border border-slate-300 bg-white px-3 py-1.5 text-xs"
          onclick={() => { deleteConfirmRowId = null; }}>Cancel</button>
      </div>
    </fieldset>
  {/if}
  {#if deletedHistoryOpen && (twoPageSoilCreationEnabled || soilDeleteRestorePort)}
    <fieldset class="min-w-0 space-y-2 rounded border border-slate-300 border-t-[3px] border-t-[#FCBA19] p-3" data-soil-deleted-history>
      <legend class="text-sm font-semibold text-[#003366]">Deleted {twoPageSoilKind(form)} horizons ({plot})</legend>
      {#if deletedHorizons.length === 0}
        <p class="text-xs text-slate-600">No deleted {twoPageSoilKind(form)} horizons recorded for {plot}.</p>
      {:else}
        <div class="space-y-1">
          {#each deletedHorizons as entry (entry.deleteRequestId)}
            <div class="flex flex-wrap items-center justify-between gap-2 rounded border border-slate-200 p-2 text-xs">
              <span>
                Horizon <span class="font-mono font-semibold">{entry.horizon || '(NULL)'}</span> | ID <span class="font-mono">{entry.deletedId}</span> | {entry.deletedAt} by {entry.deletedBy}
              </span>
              {#if entry.restored}
                <span class="text-slate-500">Restored</span>
              {:else}
                <button type="button" data-soil-restore-btn={entry.deleteRequestId}
                  disabled={parentBlocked || view.busy || deleteRestoreBusy || close.unsaved || close.blocked}
                  class="rounded border border-[#003366] bg-white px-2.5 py-1 font-semibold text-[#003366] hover:bg-slate-50"
                  onclick={() => restoreDeletedHorizon(entry.deleteRequestId)}>Restore horizon</button>
              {/if}
            </div>
          {/each}
        </div>
      {/if}
    </fieldset>
  {/if}
  {#if view.creation}
    <fieldset class="min-w-0 space-y-2 rounded border p-3">
      <legend class="font-semibold">New {twoPageSoilKind(form)} horizon for {plot}</legend>
      <label class="block text-sm font-medium" for={`${form}-new-horizon`}>New Horizon</label>
      <input id={`${form}-new-horizon`} data-soil-new-horizon data-help-id={form === 'SoilHumusXL' ? 'humus.horizon' : 'mineral.horizon'} type="text" class="w-full min-w-0 rounded border p-2"
        value={view.creation.raw} aria-invalid={!!view.creation.error}
        disabled={parentBlocked || referenceView.busy || view.busy || view.blocked || view.acknowledgementRequired}
        oninput={event => stageCreation(event.currentTarget.value)} />
      {#if view.creation.error}<p role="alert" class="break-words text-red-700">{view.creation.error}</p>{/if}
      <button type="button" disabled={parentBlocked || referenceView.busy || view.busy || view.blocked || view.acknowledgementRequired || !!view.creation.error}
        onclick={createHorizon}>Create {twoPageSoilKind(form)} horizon</button>
      <p class="break-words text-sm">Explicit creation only: the server allocates a reserved ID, Flag starts False, and other fields start NULL.
        Ordinary Save and Enter do not create a record. After creation, acknowledge its retained receipt before editing the new row.</p>
    </fieldset>
  {/if}
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
      {:else}<p role="status">No physical soil records for this exact parent.{twoPageSoilCreationEnabled ? ' Use the explicit New horizon action.' : ' Creation is unavailable.'}</p>{/each}
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
                <div class={`min-w-0 space-y-1 ${memo ? 'sm:col-span-2' : ''}`} data-soil-column={column} data-help-id={soilHelpId(form, column)}>
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
  <p class="text-xs text-gray-600">Experimental linked-soil review. Numeric/NULL depth direction follows the source form; stable ties and historical nonnumeric physical order are explicit desktop adaptations. Enter moves to the next existing record's Horizon without Save; modifiers keep normal field behavior. Source choices use current configured definitions and stage through ordinary soil validation; {twoPageSoilCreationEnabled || soilDeleteRestorePort ? 'creation requires the separate explicit New horizon action; guarded horizon deletion and restoration preserve permanent child IDs.' : 'creation, deletion and restoration remain unavailable.'} NULL is explicit; typing replaces NULL without completing or trimming text. Save is explicit and never dispatched by changing rows or choosing a definition.</p>
</section>
