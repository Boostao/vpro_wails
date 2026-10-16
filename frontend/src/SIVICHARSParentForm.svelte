<script lang="ts">
  import { onMount, onDestroy, untrack, type Snippet } from 'svelte';
  import { SIVICHARSParentEntryService, AuditRestoreAction } from '../bindings/github.com/boostao/vpro-wails';
  import { ReadRequests } from './readRequests';
  import { SIVICHARSParentEntrySession } from './siviCHARSParentEntrySession';
  import { siviCHARSParentFields, siviCHARSParentSpeciesAction } from './siviCHARSParentEntry';
  import { siviReferencePolicies } from './referenceEditor';
  import { metadataCellText } from './projectMetadataEditor';
  import type { SourceParentEntryInput } from './sourceParentEntryDraft';
  import type { EditorCloseState } from './closeLifecycle';
  import source from '../../resources/fs1333-chars-sivi-layout.json';

  let { contextId, project, plotNumber, onSaved, onBusyChange, linkedVegetation, peerBlockedReason = '', collectedAvailable = false, speciesAvailable = false, identityAvailable = false, creationAvailable = false, deletionAvailable = false, restorationAvailable = false, creationUndoAvailable = false, peerUnlockAvailable = false, metadataAvailable = false, onMetadata }: {
    contextId: string; project: string; plotNumber: string;
    onSaved?: (plot: string) => void | Promise<void>; onBusyChange?: (busy: boolean) => void;
    linkedVegetation?: Snippet; peerBlockedReason?: string; collectedAvailable?: boolean; speciesAvailable?: boolean; identityAvailable?: boolean;
    creationAvailable?: boolean; deletionAvailable?: boolean; restorationAvailable?: boolean;
    creationUndoAvailable?: boolean; peerUnlockAvailable?: boolean;
    metadataAvailable?: boolean; onMetadata?: () => void;
  } = $props();
  const owner = untrack(() => ({ contextId, project, plot: plotNumber }));
  const enabled = import.meta.env.VITE_SIVI_CHARS_PARENT_ENTRY === 'true';
  const reads = new ReadRequests();
  let view = $state.raw<ReturnType<SIVICHARSParentEntrySession['view']> | null>(null);
  let locked = $state(true);
  let operationError = $state('');
  const session = new SIVICHARSParentEntrySession(owner, {
    read: () => reads.track(SIVICHARSParentEntryService.Review(owner.contextId, owner.plot)),
    cancelRead: () => reads.cancelAll(),
    save: request => SIVICHARSParentEntryService.Save(owner.contextId, owner.plot, request),
    restore: (history, action) => SIVICHARSParentEntryService.Restore(owner.contextId, owner.plot, history, action),
    refreshParent: async () => { await onSaved?.(owner.plot); },
  }, () => { view = session.view(); });
  view = session.view();
  const close = $derived.by(() => { view; return session.closeState(); });
  const disabled = $derived(!enabled || locked || !view?.original || Boolean(view?.busy || view?.blocked || peerBlockedReason));
  const peerParentReadAvailable = $derived(peerUnlockAvailable && !view?.original &&
    !close.unsaved && !close.busy && !close.blocked);
  const lockToggleDisabled = $derived(!enabled || close.busy || close.unsaved || close.blocked ||
    (!view?.original && !(locked && peerParentReadAvailable)) ||
    Boolean(peerBlockedReason && !(locked && peerUnlockAvailable)));

  function toggleParentLock(): boolean {
    if (lockToggleDisabled) {
      operationError = 'Keep the parent clean and idle; an unavailable original permits only owned peer recovery unlock.';
      return false;
    }
    locked = !locked;
    operationError = '';
    return true;
  }
  const unavailableChildren = $derived([speciesAvailable ? '' : 'Species', collectedAvailable ? '' : 'Collected',
    identityAvailable ? '' : 'identity', creationAvailable ? '' : 'creation', deletionAvailable ? '' : 'deletion',
    restorationAvailable ? '' : 'typed restoration', creationUndoAvailable ? '' : 'historical creation Undo'].filter(Boolean).join(', '));
  const site = siviCHARSParentFields.filter(field => 'pageId' in field && field.pageId === 'form:frmSIVIsite-CHARS/Site');
  const interval = (first: string, last: string) =>
    site.slice(site.findIndex(field => field.binding === first), site.findIndex(field => field.binding === last) + 1);
  const groups = [
    { name: 'Plot & survey', fields: siviCHARSParentFields.filter(field => !('pageId' in field) && field.binding !== 'siteNotes') },
    { name: 'Site description', fields: interval('Location', 'SnowCoverregime') },
    { name: 'Topography', fields: interval('SlopeGradient', 'SV_FloodPlain') },
    { name: 'STAND', fields: interval('StandAge', 'StructuralStage') },
    { name: 'TERRAIN', fields: interval('TerrainTextureSurf', 'BedrockGeology2') },
    { name: 'SOIL', fields: interval('HumusForm', 'RootRestrictingDepth') },
    { name: 'Vegetation', fields: siviCHARSParentFields.filter(field => 'pageId' in field && field.pageId === 'form:frmSIVIsite-CHARS/Veg') },
    { name: 'Notes', fields: siviCHARSParentFields.filter(field => field.binding === 'siteNotes') },
  ];
  if (groups.some(group => group.fields.length === 0) || groups.reduce((sum, group) => sum + group.fields.length, 0) !== 71 ||
      new Set(groups.flatMap(group => group.fields.map(field => field.controlId))).size !== 71) {
    throw new Error('CHARS presentation requires every original binding exactly once in its source group.');
  }
  function caption(field: typeof siviCHARSParentFields[number]) {
    if (field.binding === 'RootRestrictingDepth') {
      const label = source.forms[0].fields.find(row => row.controlName === 'Label601');
      if (!label?.caption) throw new Error('CHARS root-restricting depth lost its original separate label.');
      return label.caption;
    }
    if (field.binding === 'SV_StandAgeEstMeas') return 'Stand Age - Est./Meas.';
    if (field.binding === 'SV_StandHeightEstMeas') return 'Stand Ht - Est./Meas.';
    return siviReferencePolicies.find(row => row.column === field.binding)?.label || field.caption || field.controlName;
  }
  async function operation(kind: 'load' | 'save' | 'undo' | 'retain' | 'prune') {
    operationError = '';
    try {
      if (!enabled) throw new Error('CHARS parent entry is disabled in this build.');
      if (peerBlockedReason && !(kind === 'load' && peerParentReadAvailable)) throw new Error(peerBlockedReason);
      if (kind === 'load') return await session.load();
      if ((kind === 'save' || kind === 'retain' || kind === 'prune') && !view?.original) {
        throw new Error('Load the owned CHARS parent original before writing.');
      }
      if (kind === 'save') {
        if (locked) throw new Error('Unlock CHARS entry before saving.');
        return await session.save();
      }
      if (kind === 'undo') return await session.undo();
      if (locked) throw new Error('Unlock CHARS entry before restoring history.');
      return await session.restore(kind === 'retain' ? AuditRestoreAction.AuditRestoreRetain : AuditRestoreAction.AuditRestorePrune);
    } catch (cause) {
      operationError = String(cause);
      return false;
    }
  }
  function stage(column: string, input: SourceParentEntryInput) {
    operationError = '';
    try {
      if (disabled) throw new Error(peerBlockedReason || 'CHARS entry is locked, busy or unavailable.');
      session.stage(column, input);
    } catch (cause) { operationError = String(cause); }
  }
  export function getCloseState(): EditorCloseState {
    view;
    return { ...session.closeState(), error: operationError || session.closeState().error };
  }
  export async function saveForClose() { return operation('save'); }
  export function canReloadLinkedParent() {
    view;
    return enabled && !locked && !close.unsaved && !close.busy && !close.blocked;
  }
  export function canOpenLinkedVegetation() {
    return canReloadLinkedParent() && !!view?.original;
  }
  export async function refreshAfterLinkedSave() {
    if (!canReloadLinkedParent()) throw new Error('CHARS parent must remain clean, unlocked and idle during linked refresh.');
    if (!await session.load()) throw new Error(session.view().error || 'CHARS linked parent refresh failed.');
    operationError = '';
  }
  export function canOpenProjectMetadata() { return metadataAvailable && typeof onMetadata === 'function'; }
  export function openProjectMetadata() {
    if (!canOpenProjectMetadata()) {
      operationError = 'Project metadata requires the clean unlocked CHARS owner and its independent editing gate.';
      return;
    }
    operationError = '';
    onMetadata?.();
  }
  $effect(() => { onBusyChange?.(Boolean(view?.busy)); });
  onMount(() => { if (enabled) void operation('load'); });
  onDestroy(() => { session.dispose(); onBusyChange?.(false); });
</script>

<section class="min-w-0 space-y-4 rounded border border-stone-200 bg-white p-4" aria-label="CHARS SIVI parent entry" data-chars-parent-panel>
  <h2 class="text-lg font-semibold">CHARS SIVI / FS1333 — {owner.plot}</h2>
  {#if operationError || view?.error}<p role="alert" class="text-red-800">{operationError || view?.error}</p>{/if}
  {#if view?.blocked}<p role="alert">The mutation outcome or refresh is unresolved. Undo / reload to observe storage; never replay Save.</p>{/if}
  {#if view?.busy}<p role="status">CHARS parent entry: {view.operation}...</p>{/if}
  {#if !enabled}<p role="status">CHARS parent entry is disabled in this build.</p>{/if}
  <div class="flex flex-wrap gap-2">
    {#if onMetadata}
      <button type="button" class="rounded border px-3 py-2 disabled:opacity-50" data-chars-metadata-open
        data-source-control="form:frmSIVIsite-CHARS/btnEditMetadata"
        disabled={!canOpenProjectMetadata()} onclick={openProjectMetadata}>Edit Metadata</button>
    {/if}
    <button type="button" class="rounded border px-3 py-2 disabled:opacity-50" data-chars-lock
      disabled={lockToggleDisabled}
      onclick={() => { toggleParentLock(); }}>{locked ? 'Unlock CHARS entry' : 'Lock CHARS entry'}</button>
    <button type="button" class="rounded border px-3 py-2 disabled:opacity-50" data-chars-save
      disabled={disabled || !close.canSave} onclick={() => void operation('save')}>Save CHARS entry</button>
    <button type="button" class="rounded border px-3 py-2 disabled:opacity-50" data-chars-undo
      disabled={!enabled || close.busy || Boolean(peerBlockedReason)} onclick={() => void operation('undo')}>Undo / reload</button>
    {#if peerParentReadAvailable}
      <button type="button" class="rounded border px-3 py-2 disabled:opacity-50" data-chars-peer-parent-read
        disabled={!enabled} onclick={() => void operation('load')}>Read parent for retained peer recovery</button>
    {/if}
    {#if view?.historyId}
      <button type="button" class="rounded border px-3 py-2 disabled:opacity-50"
        disabled={disabled || close.unsaved || Boolean(close.blocked)}
        onclick={() => void operation('retain')}>Restore CHARS (retain audits)</button>
      <button type="button" class="rounded border px-3 py-2 disabled:opacity-50"
        disabled={disabled || close.unsaved || Boolean(close.blocked)}
        onclick={() => void operation('prune')}>Restore CHARS (prune audits)</button>
    {/if}
    {#if view?.operation === 'read'}
      <button type="button" class="rounded border px-3 py-2" onclick={() => session.cancel()}>Cancel CHARS read</button>
    {/if}
  </div>
  {#if enabled && view?.original && view.snapshot}
    {#each groups as group}
      <section class="min-w-0 space-y-2" aria-label={group.name}>
        <h3 class="font-semibold">{group.name}</h3>
        <div class="grid min-w-0 grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {#each group.fields as field (field.controlId)}
            {@const binding = view.original.Bindings.find(row => row.ControlID === field.controlId)!}
            {@const row = view.original.Rows[0][binding.Table === view.original.EnvTable ? 'Env' : 'Admin']}
            {@const original = row.cells[binding.Column]}
            {@const policy = view.snapshot.policies.find(row => row.controlId === field.controlId)}
            {@const draft = policy ? view.drafts[policy.column] : undefined}
            {@const value = draft?.value ?? original}
            {@const raw = draft?.input.kind === 'text' ? draft.input.raw : metadataCellText(value)}
            {@const id = `chars-${field.binding}`}
            {@const reference = view.snapshot.fields.find(row => row.column === policy?.column)}
            <div class="min-w-0 space-y-1" data-chars-field={policy?.column ?? field.binding} data-source-control={field.controlId}>
              <label class="block text-sm font-medium" for={id}>{caption(field)}</label>
              {#if !policy}
                <output {id} class="block break-words whitespace-pre-wrap rounded bg-stone-100 p-2">{metadataCellText(original) || (original.storage === 'null' ? 'NULL' : '(empty text)')}</output>
                <p class="text-xs text-stone-600">This source identity/action remains read-only.</p>
              {:else if policy.kind === 'boolean' || policy.kind === 'option'}
                <select {id} class="w-full min-w-0 rounded border p-2" disabled={disabled || !policy.writable}
                  aria-invalid={Boolean(draft?.error)} aria-describedby={draft?.error ? `${id}-error` : undefined}
                  value={value.storage === 'null' ? 'null' : policy.kind === 'boolean' && value.storage === 'integer' && ['-1', '0'].includes(value.integer ?? '') ? value.integer! : policy.kind === 'option' && value.storage === 'text' && ['1', '2'].includes(value.text ?? '') ? value.text! : 'original'}
                  onchange={event => {
                    const choice = event.currentTarget.value;
                    stage(policy.column, choice === 'original' ? { kind: 'original' } : choice === 'null' ? { kind: 'clear' }
                      : policy.kind === 'boolean' ? { kind: 'boolean', value: choice === '-1' } : { kind: 'option', option: Number(choice) });
                  }}>
                  <option value="original">Retain original storage</option>
                  <option value="null" disabled={policy.kind === 'boolean' && original.storage !== 'null'}>
                    {policy.kind === 'boolean' ? 'Retain historical NULL' : 'NULL'}</option>
                  {#if policy.kind === 'boolean'}<option value="-1">Yes (-1)</option><option value="0">No (0)</option>
                  {:else}<option value="1">Est.</option><option value="2">Meas.</option>{/if}
                </select>
              {:else if policy.kind === 'text' && policy.maximum === 0}
                <textarea {id} class="w-full min-w-0 rounded border p-2" value={raw} disabled={disabled || !policy.writable}
                  aria-invalid={Boolean(draft?.error)} aria-describedby={draft?.error ? `${id}-error` : undefined}
                  oninput={event => stage(policy.column, { kind: 'text', raw: event.currentTarget.value })}></textarea>
              {:else}
                <input {id} class="w-full min-w-0 rounded border p-2" type="text" value={raw}
                  disabled={disabled || !policy.writable} list={reference ? `${id}-choices` : undefined}
                  aria-invalid={Boolean(draft?.error)} aria-describedby={draft?.error ? `${id}-error` : undefined}
                  oninput={event => stage(policy.column, { kind: 'text', raw: event.currentTarget.value })} />
              {/if}
              {#if policy}
                {#if reference}
                  <datalist id={`${id}-choices`}>
                    {#each reference.available ? reference.choices.filter(choice => choice.selectable && choice.code !== null) : [] as choice}
                      <option value={choice.code ?? ''} data-reference-row={choice.rowId}>{choice.description ?? '(NULL description)'}</option>
                    {/each}
                  </datalist>
                {/if}
                <div class="flex flex-wrap gap-2 text-xs">
                  <button type="button" class="rounded border px-2 py-2"
                    disabled={disabled || !policy.writable || policy.kind === 'boolean' && original.storage !== 'null'}
                    aria-label={`Set NULL ${caption(field)}`} onclick={() => stage(policy.column, { kind: 'clear' })}>Set NULL</button>
                  <button type="button" class="rounded border px-2 py-2" disabled={disabled}
                    aria-label={`Retain original ${caption(field)}`} onclick={() => stage(policy.column, { kind: 'original' })}>Retain original</button>
                  <span class="self-center">{value.storage === 'text' && value.text === '' ? 'empty text' : value.storage}</span>
                </div>
                {#if policy.diagnostic}<p role="status" class="text-xs text-amber-800">{policy.diagnostic}</p>{/if}
                {#if reference && !reference.available}<p role="status" class="text-xs text-amber-800">{reference.diagnostic}</p>{/if}
                {#if view.advisories[policy.column]}<p role="status" class="text-xs text-amber-800">{view.advisories[policy.column]}</p>{/if}
                {#if draft?.error}<p id={`${id}-error`} role="alert" class="text-sm text-red-800">{draft.error}</p>{/if}
              {/if}
            </div>
          {/each}
          {#if group.name === 'Vegetation'}
            {@const binding = view.original.Bindings.find(row => row.Binding === 'SpeciesListComplete' && row.Implicit)!}
            {@const original = view.original.Rows[0].Env.cells[binding.Column]}
            {@const draft = view.drafts.SpeciesListComplete}
            {@const value = draft?.value ?? original}
            <div class="min-w-0 space-y-1" data-chars-action="SpeciesListComplete" data-source-control={siviCHARSParentSpeciesAction}>
              <label class="block text-sm font-medium" for="chars-SpeciesListComplete">Spp List</label>
              <select id="chars-SpeciesListComplete" class="w-full min-w-0 rounded border p-2" {disabled}
                aria-invalid={Boolean(draft?.error)} aria-describedby={draft?.error ? 'chars-SpeciesListComplete-error' : undefined}
                value={value.storage === 'integer' && value.integer === '-1' ? '1'
                  : value.storage === 'integer' && value.integer === '0' ? '2' : 'original'}
                onchange={event => {
                  const option = event.currentTarget.value;
                  stage('SpeciesListComplete', option === 'original' ? { kind: 'original' } : { kind: 'option', option: Number(option) });
                }}>
                <option value="original">Retain original storage ({metadataCellText(original) || (original.storage === 'null' ? 'NULL' : original.storage)})</option>
                <option value="1">Comp.</option>
                <option value="2">Part.</option>
              </select>
              <button type="button" class="rounded border px-2 py-2 text-xs" {disabled}
                onclick={() => stage('SpeciesListComplete', { kind: 'original' })}>Retain original Species list complete</button>
              {#if draft?.error}<p id="chars-SpeciesListComplete-error" role="alert" class="text-sm text-red-800">{draft.error}</p>{/if}
              <p class="text-xs text-stone-600">Species list complete: Comp. stores Yes (-1); Part. stores No (0). Historical NULL remains by retention, not a new NULL assignment.</p>
            </div>
          {/if}
        </div>
        {#if group.name === 'Vegetation'}{@render linkedVegetation?.()}{/if}
      </section>
    {/each}
  {/if}
  {#if !view?.original}{@render linkedVegetation?.()}{/if}
  <p class="text-sm text-stone-600">Explicit Save, Undo and history restoration are desktop safety adaptations, not original callback parity.
    Only Exposure 1/2, Soil Drainage and Successional Status require listed non-NULL codes. Other codes remain literal.
    Coordinates retain finite source DOUBLE values without new geographic bounds. Numeric blanks mean NULL; spaces are not silently trimmed.
    Dates use exact YYYY-MM-DD HH:MM:SS with an optional fractional second.</p>
  <p role="status" class="text-sm text-amber-800">CHARS ProjectID/PlotType and unqualified source actions,
    {linkedVegetation ? `${unavailableChildren ? `child ${unavailableChildren} and ` : ''}unqualified callbacks,`
      : 'child vegetation workflows,'}
    changed SubZone and blank A-horizon assignment remain unavailable in this {linkedVegetation ? 'CHARS editor' : 'parent-only editor'}.
    Use the separately accepted normal SIVI/FS882 workflows where applicable.</p>
</section>
