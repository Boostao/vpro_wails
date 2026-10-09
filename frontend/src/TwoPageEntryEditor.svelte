<script lang="ts">
  import { onDestroy, untrack, type Snippet } from 'svelte';
  import { AuditRestoreAction, TwoPageParentEntryService, type SIVIParentSharedReference } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import type { EditorCloseState } from './closeLifecycle';
  import type { TwoPageParentForm } from './twoPageParentSession';
  import { twoPageParentCommonFields } from './twoPageParentCommonSession';
  import type { TwoPageEntryInput } from './twoPageEntryDraft';
  import { TwoPageEntrySession, twoPageEntryCell, type TwoPageEntrySessionCache } from './twoPageEntrySession';
  import TwoPageEntryLayout from './TwoPageEntryLayout.svelte';
  import { twoPageEntryProjection, twoPageEntryValues, type TwoPageEntryChild, type TwoPageEntryField } from './twoPageEntryProjection';
  import { metadataCellText } from './projectMetadataEditor';
  import { controlLabel, wideControl } from './formPresentation';
  import type { PaperControl } from './paperLayout';

  let { contextId, project, plot, cache, revision, onchange, onbusy, onclosed, oncommitted, embedded: childContent }: {
    contextId: string; project: string; plot: string; cache: TwoPageEntrySessionCache; revision: number;
    onchange: () => void; onbusy: (busy: boolean) => void; onclosed: () => void; oncommitted: () => Promise<void>;
    embedded: Snippet<[TwoPageEntryChild, PaperControl]>;
  } = $props();
  const owner = untrack(() => ({ contextId, project, plot }));
  const reviewEnabled = import.meta.env.VITE_TWO_PAGE_PARENT_REVIEW === 'true'
    && import.meta.env.VITE_TWO_PAGE_PARENT_ENTRY_REVIEW === 'true';
  const editingEnabled = reviewEnabled && import.meta.env.VITE_TWO_PAGE_PARENT_ENTRY_EDITING === 'true';
  const sessions = untrack(() => {
    const key = JSON.stringify(owner);
    const existing = cache.get(key) ?? new Map<TwoPageParentForm, TwoPageEntrySession>();
    cache.set(key, existing); return existing;
  });
  let form = $state<TwoPageParentForm>('FS882-8x6XL');
  let session = $state(makeSession('FS882-8x6XL'));
  let error = $state<string | null>(null);
  const view = $derived.by(() => { revision; return session.view(); });
  const close = $derived.by(() => { revision; return session.closeState(); });
  const projection = $derived(twoPageEntryProjection(form));

  function makeSession(sourceForm: TwoPageParentForm) {
    const existing = sessions.get(sourceForm);
    if (existing) return existing;
    const requests = new ReadRequests();
    const created = new TwoPageEntrySession(owner, sourceForm, {
      read: () => requests.track(TwoPageParentEntryService.GetOriginal(owner.contextId, owner.plot, sourceForm)),
      readReferences: filters => requests.track(TwoPageParentEntryService.GetReferences(owner.contextId, owner.plot, sourceForm, filters)),
      cancelRead: () => requests.cancelAll(),
      save: request => TwoPageParentEntryService.Save(owner.contextId, owner.plot, sourceForm, request),
      restore: (history, action) => TwoPageParentEntryService.Restore(owner.contextId, owner.plot, sourceForm, history, action),
      refreshParent: async () => {
        await oncommitted();
        for (const [variant, peer] of sessions) {
          if (variant === sourceForm || !peer.view().original) continue;
          if (peer.closeState().unsaved || peer.closeState().busy || !await peer.load()) {
            throw new Error(peer.view().error ?? 'Another complete-entry variant could not refresh; no write was replayed.');
          }
        }
      },
    }, onchange);
    sessions.set(sourceForm, created); return created;
  }
  $effect(() => { onbusy(view.busy || view.referencesBusy); });
  onDestroy(() => {
    const active = session.view();
    if (active.operation === 'read' || active.referencesBusy) session.cancel();
  });
  export function getCloseState(): EditorCloseState {
    return { ...close, unsaved: true, canSave: false, error: error ?? close.error,
      saveReason: 'Save or Undo complete-entry drafts explicitly, then close their review before saving or closing the plot.' };
  }
  export function undo() { void operation('undo'); }
  async function chooseForm(next: TwoPageParentForm) {
    if (close.unsaved || close.busy || close.blocked) {
      error = 'Save or Undo the owned complete-entry draft before changing source variant.'; return;
    }
    form = next; session = makeSession(next); onchange();
    await operation('load');
  }
  async function operation(action: 'load' | 'save' | 'undo' | 'retain' | 'prune' | 'references') {
    try {
      if (!reviewEnabled || !editingEnabled && action !== 'load' && action !== 'undo' && action !== 'references') {
        throw new Error('Complete-entry review/editing is independently disabled.');
      }
      error = null;
      const ok = action === 'load' ? await session.load() : action === 'save' ? await session.save()
        : action === 'undo' ? await session.undo() : action === 'references' ? await session.loadReferences()
        : await session.restore(action === 'retain' ? AuditRestoreAction.AuditRestoreRetain : AuditRestoreAction.AuditRestorePrune);
      if (!ok) error = session.view().error ?? session.view().referenceError;
    } catch (cause) { error = `Complete-entry operation refused; owned state retained: ${String(cause)}`; }
  }
  function stage(column: string, input: TwoPageEntryInput) {
    try {
      if (!editingEnabled) throw new Error('Complete-entry editing is independently disabled.');
      session.stage(column, input); error = session.view().error;
    } catch (cause) { error = `Complete-entry draft refused; owned state retained: ${String(cause)}`; }
  }
  function selectProject(rowId: string | null) {
    try {
      if (!editingEnabled) throw new Error('Complete-entry editing is independently disabled.');
      session.selectProject(rowId); error = session.view().error;
    } catch (cause) { error = `Physical ProjectID selection refused: ${String(cause)}`; }
  }
  function acknowledge(column: string) {
    try {
      if (!editingEnabled) throw new Error('Complete-entry editing is independently disabled.');
      session.acknowledge(column); error = session.view().error;
    } catch (cause) { error = `Literal acknowledgement refused: ${String(cause)}`; }
  }
  function cancel() {
    try { session.cancel(); error = session.view().error; }
    catch (cause) { error = `Complete-entry cancellation refused: ${String(cause)}`; }
  }
  function closeReview() {
    if (close.unsaved || close.busy || close.blocked) {
      error = close.saveReason || 'Save or Undo complete-entry drafts before closing their review.'; return;
    }
    onclosed();
  }
  function referenceChoices(reference: SIVIParentSharedReference) {
    if (reference.choices === null) throw new Error(`${reference.column} reference lost its complete choice receipt.`);
    return reference.choices;
  }
</script>

<section class="space-y-4 rounded border bg-white p-4" aria-label="Complete two-page entry review" data-two-page-entry-panel={form}>
  <h3 class="font-semibold">{form}: one owned complete-entry draft</h3>
  {#if error}<p role="alert" class="text-sm text-red-700">{error}</p>{/if}
  {#if view.error && view.error !== error}<p role="alert" class="text-sm text-red-700">{view.error}</p>{/if}
  {#if view.referenceError}<p role="alert" class="text-sm text-red-700">{view.referenceError}</p>{/if}
  {#if view.blocked}<p role="alert">Write outcome or refresh is unresolved. Undo / reload explicitly; never replay Save.</p>{/if}
  {#if view.busy || view.referencesBusy}<p role="status">Complete-entry {view.operation ?? 'reference read'} in progress...</p>{/if}
  <div class="flex flex-wrap gap-2">
    <button type="button" disabled={close.unsaved || close.busy || close.blocked} aria-pressed={form === 'FS882-8x6XL'}
      onclick={() => chooseForm('FS882-8x6XL')}>Normal two-page source</button>
    <button type="button" disabled={close.unsaved || close.busy || close.blocked} aria-pressed={form === 'FS882-8x6XL-CHARS'}
      onclick={() => chooseForm('FS882-8x6XL-CHARS')}>CHARS two-page source</button>
    <button type="button" disabled={!reviewEnabled || close.unsaved || close.busy || close.blocked}
      onclick={() => operation('load')}>Load complete entry</button>
    <button type="button" disabled={!editingEnabled || !close.canSave || close.busy || close.blocked}
      onclick={() => operation('save')}>Save complete entry</button>
    <button type="button" disabled={close.busy} onclick={() => operation('undo')}>Undo / reload</button>
    <button type="button" disabled={!reviewEnabled || close.busy || !view.original || view.blocked}
      onclick={() => operation('references')}>Refresh draft choices</button>
    <button type="button" disabled={close.unsaved || close.busy || close.blocked} onclick={closeReview}>Close complete-entry review</button>
    {#if view.busy && view.operation === 'read' || view.referencesBusy}
      <button type="button" onclick={cancel}>Cancel entry read</button>
    {/if}
    {#if view.historyId}
      <button type="button" disabled={!editingEnabled || close.unsaved || close.busy || close.blocked}
        onclick={() => operation('retain')}>Restore (retain audits)</button>
      <button type="button" disabled={!editingEnabled || close.unsaved || close.busy || close.blocked}
        onclick={() => operation('prune')}>Restore (prune audits)</button>
    {/if}
  </div>
  {#if view.original && view.snapshot}
    <TwoPageEntryLayout {form} values={twoPageEntryValues(view.original, form)}
      editors={(['xl', 'common', 'extra'] as const).map(scope => ({
        scope, columns: projection.fields.filter(field => field.scope === scope && field.page !== null).map(field => field.column),
        input: fieldInput, labelled: true,
      }))} identity={identityField} embedded={childContent} />
  {/if}
  <p class="text-xs text-stone-600">One atomic parent Save covers all three accepted field scopes.
    Identity changes, coordinate-conversion callbacks, metadata creation, pictures and unverified source actions remain unavailable.
    Linked child rows are read-only in this review; their existing independent editors remain outside this parent transaction.</p>
</section>

{#snippet identityField(field: TwoPageEntryField)}
  <label class="block text-sm font-medium" for="complete-entry-identity">Plot number</label>
  <input id="complete-entry-identity" class="w-full rounded border p-2" disabled
    data-column={field.column} value={owner.plot} />
{/snippet}

{#snippet fieldInput(control: PaperControl, _position: string)}
  {#if view.original && view.snapshot && control.column}
    {@const column = control.column}
    {@const policy = view.snapshot.Policies.find(field => field.column === column)}
    {@const draft = view.drafts[column]}
    {@const value = draft?.value ?? twoPageEntryCell(view.original, column)}
    {@const raw = draft?.input.kind === 'text' ? draft.input.raw : value.storage === 'null' ? '' : metadataCellText(value)}
    {@const reference = view.snapshot.Fields.find(field => field.column === column)}
    {@const id = `complete-entry-${column}`}
    {@const unavailable = !editingEnabled || close.busy || view.blocked || column === 'BECSiteUnit' && !view.snapshot.MasterEditingAvailable}
    <div class="min-w-0 space-y-1" data-two-page-entry-field={column} data-source-control={control.controlId}>
      <label class="block text-sm font-medium" for={id}>{controlLabel(control, true)}</label>
      {#if column === 'ProjectID'}
        <select {id} class="w-full rounded border p-2" value={view.projectRowId ?? ''}
          disabled={unavailable || !view.snapshot.ProjectAssignmentAvailable}
          onchange={event => selectProject(event.currentTarget.value || null)}>
          <option value="">Keep original ({value.storage === 'null' ? 'NULL' : raw})</option>
          {#each view.snapshot.ProjectChoices.Choices.rows as row (row.rowId)}
            {@const code = row.cells[0]}
            {@const title = row.cells[1]}
            <option value={row.rowId} disabled={code.storage !== 'text' || !code.text || code.text.length > 30}>
              {metadataCellText(code)} — {title.storage === 'null' ? '(NULL title)' : title.text === '' ? '(empty title)' : metadataCellText(title)} [row {row.rowId}]
            </option>
          {/each}
        </select>
        {#if !view.snapshot.ProjectAssignmentAvailable}<p role="status" class="text-xs">{view.snapshot.ProjectAssignmentDiagnostic}</p>{/if}
      {:else if policy?.kind === 'boolean'}
        {@const selected = value.storage === 'null' ? '' : value.integer === '-1' ? 'true' : value.integer === '0' ? 'false' : 'historical'}
        <select {id} class="w-full rounded border p-2" value={selected} disabled={unavailable}
          onchange={event => stage(column, event.currentTarget.value === '' ? { kind: 'clear' }
            : { kind: 'boolean', value: event.currentTarget.value === 'true' })}>
          <option value="">NULL</option><option value="true">Yes</option><option value="false">No</option>
          {#if selected === 'historical'}<option value="historical" disabled>Historical {raw}</option>{/if}
        </select>
      {:else if policy?.kind === 'option'}
        {@const selected = value.storage === 'null' ? '' : value.text === '1' || value.text === '2' ? value.text : 'historical'}
        {@const options = twoPageParentCommonFields(form).find(field => field.column === column)?.options ?? []}
        <select {id} class="w-full rounded border p-2" value={selected} disabled={unavailable}
          onchange={event => stage(column, { kind: 'option', option: event.currentTarget.value === '' ? null : Number(event.currentTarget.value) })}>
          <option value="">NULL</option>
          {#each options as option}<option value={String(option.value)}>{option.label}</option>{/each}
          {#if selected === 'historical'}<option value="historical" disabled>Historical {raw}</option>{/if}
        </select>
      {:else if wideControl(control)}
        <textarea {id} class="w-full rounded border p-2" rows="3" value={raw} disabled={unavailable}
          aria-invalid={Boolean(draft?.error)} aria-describedby={draft?.error ? `${id}-error` : undefined}
          oninput={event => stage(column, { kind: 'text', raw: event.currentTarget.value })}></textarea>
      {:else}
        <input {id} class="w-full rounded border p-2" type="text" value={raw} disabled={unavailable}
          list={reference?.available ? `${id}-choices` : undefined}
          aria-invalid={Boolean(draft?.error)} aria-describedby={draft?.error ? `${id}-error` : undefined}
          oninput={event => stage(column, { kind: 'text', raw: event.currentTarget.value })} />
        {#if reference?.available}
          <datalist id={`${id}-choices`}>
            {#each referenceChoices(reference).filter(choice => choice.selectable && choice.code !== null) as choice (JSON.stringify([choice.rowId, choice.code, choice.description]))}
              <option value={choice.code ?? ''}>{choice.description === null ? '(NULL description)' : choice.description}</option>
            {/each}
          </datalist>
        {/if}
      {/if}
      {#if column !== 'ProjectID'}
        <div class="flex flex-wrap gap-2 text-xs">
          <button type="button" disabled={unavailable} onclick={() => stage(column, { kind: 'clear' })}>Clear to NULL</button>
          <button type="button" disabled={unavailable} onclick={() => stage(column, { kind: 'original' })}>Keep original</button>
        </div>
      {/if}
      {#if draft?.error}<p id={`${id}-error`} role="alert" class="text-xs text-red-700">{draft.error}</p>{/if}
      {#if reference && !reference.available}<p role="status" class="text-xs">{reference.diagnostic}</p>{/if}
      {#if column === 'BECSiteUnit' && !view.snapshot.MasterEditingAvailable}
        <p role="status" class="text-xs">The owned current user is not authorized to change Master site units.</p>
      {/if}
      {#if view.advisories[column]}
        <p class="text-xs">{view.advisories[column]}</p>
        <button type="button" disabled={unavailable} onclick={() => acknowledge(column)}>Acknowledge literal {column}</button>
      {/if}
    </div>
  {/if}
{/snippet}
