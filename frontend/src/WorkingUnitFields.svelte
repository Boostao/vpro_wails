<script lang="ts">
  import { onDestroy, untrack, type Snippet } from 'svelte';
  import { WorkingUnitService, type FS882Header } from '../bindings/github.com/boostao/vpro-wails';
  import type { PaperControl } from './paperLayout';
  import { ReadRequests } from './readRequests';
  import FieldGuidance from './FieldGuidance.svelte';
  import { masterBECAcknowledged, rememberMasterBECAcknowledgement, masterBECValidation, masterBECWarnings } from './masterBECEditor';
  import { WorkingUnitLookup, isWorkingUnitControl, workingUnitAcknowledged, rememberWorkingUnitAcknowledgement,
    workingUnitChanged, workingUnitCopy, workingUnitDefinitions, workingUnitGroups, workingUnitValidation,
    workingUnitWarnings, type WorkingUnitCodes, type WorkingUnitMode, type WorkingUnitCopy, type WorkingUnitSession } from './workingUnitEditor';

  let { draft = $bindable(), original, controls, position, capabilities, disabled, onchange, onerror,
    onvalidation, onbusy, session, masterAllowed = false, children }: {
    draft: FS882Header; original: WorkingUnitCodes | null; controls: PaperControl[];
    position: (control: PaperControl) => string; capabilities: Record<string, boolean | undefined>;
    disabled: boolean; onchange: () => void; onerror: (message: string) => void;
    onvalidation: (key: 'workingUnit' | 'masterBEC', message: string | null) => void;
    masterAllowed?: boolean;
    onbusy?: (busy: boolean) => void; session: WorkingUnitSession; children: Snippet<[Snippet<[PaperControl?]>]>;
  } = $props();
  const editingEnabled = import.meta.env.VITE_WORKING_UNIT_EDITING !== 'false';
  const masterEditingEnabled = import.meta.env.VITE_MASTER_BEC_EDITING !== 'false';
  const componentId = $props.id();
  const listId = `${componentId}-working-unit`;
  function rows<T>(value: T[] | null): T[] {
    if (value === null) throw new Error('The Working Unit service did not return choice rows.');
    return value;
  }
  const reads = new ReadRequests();
  const lookup = new WorkingUnitLookup({
    getMode: () => WorkingUnitService.GetWorkingUnitMode(),
    setMode: mode => WorkingUnitService.SetWorkingUnitMode(mode),
    choices: async mode => rows(await reads.track(WorkingUnitService.GetWorkingUnitChoices(mode))),
    master: async () => rows(await reads.track(WorkingUnitService.GetMasterWorkingUnitChoices()))
  }, next => { view = next; onbusy?.(next.busy); }, untrack(() => session), () => reads.cancelAll());
  let view = $state(lookup.snapshot());
  let acknowledged = $state(false);
  let masterAcknowledged = $state(false);
  let notice = $state<string | null>(null);
  let proposal = $state<{ identity: FS882Header; previous: string | null; copy: WorkingUnitCopy } | null>(null);
  const changed = $derived(workingUnitChanged(draft, original));
  const warnings = $derived(workingUnitWarnings(draft.userSiteUnit, view));
  const groups = $derived(workingUnitGroups(view.choices));
  const definitions = $derived(workingUnitDefinitions(view.choices, draft.userSiteUnit));
  const masterDefinitions = $derived(workingUnitDefinitions(view.master, draft.becSiteUnit));
  const masterWarnings = $derived(masterBECWarnings(draft.becSiteUnit, view));
  const masterValidation = $derived(masterBECValidation(draft.becSiteUnit, original?.becSiteUnit ?? null, masterAllowed, view, masterAcknowledged));
  const masterGroups = $derived(workingUnitGroups(view.master));
  const modeControls: Record<string, WorkingUnitMode> = { Option455: 'env', Option457: 'master', Option459: 'su' };

  $effect(() => {
    const identity = draft;
    if ((editingEnabled || masterEditingEnabled) && identity) untrack(() => { void lookup.refresh(); });
  });
  $effect(() => {
    acknowledged = workingUnitAcknowledged(draft, draft.userSiteUnit);
    masterAcknowledged = masterBECAcknowledged(draft, draft);
    if (proposal && (proposal.identity !== draft || proposal.previous !== draft.userSiteUnit || proposal.copy.value !== draft.becSiteUnit)) {
      proposal = null;
    }
  });
  $effect(() => {
    const message = editingEnabled ? workingUnitValidation(draft, original, view, acknowledged) : null;
    untrack(() => onvalidation('workingUnit', message));
  });
  $effect(() => {
    const message = masterEditingEnabled ? masterValidation : null;
    untrack(() => onvalidation('masterBEC', message));
  });
  onDestroy(() => {
    lookup.dispose(); onbusy?.(false);
    if (masterEditingEnabled) onvalidation('masterBEC', masterBECValidation(draft.becSiteUnit,
      original?.becSiteUnit ?? null, masterAllowed, { ...view, busy: false }, masterAcknowledged));
  });

  function masterUnavailable(control: PaperControl): boolean {
    return !masterEditingEnabled || !masterAllowed || disabled || view.busy || capabilities.becSiteUnit !== true || !control.enabled;
  }
  function setMasterCode(raw: string, control: PaperControl): void {
    if (masterUnavailable(control)) {
      onerror('BEC Master editing is unavailable while unauthorized, locked, loading or unsupported.');
      return;
    }
    const value = raw === '' ? null : raw;
    if (draft.becSiteUnit === value) return;
    draft.becSiteUnit = value;
    proposal = null;
    onchange();
    onvalidation('masterBEC', masterBECValidation(value, original?.becSiteUnit ?? null, masterAllowed, view, masterBECAcknowledged(draft, draft)));
  }

  function unavailable(): boolean {
    return !editingEnabled || disabled || view.busy || capabilities.userSiteUnit !== true;
  }
  function setCode(raw: string): void {
    const value = raw === '' ? null : raw;
    if (draft.userSiteUnit === value) return;
    draft.userSiteUnit = value;
    notice = null;
    proposal = null;
    onchange();
  }
  function acknowledge(accepted: boolean): void {
    rememberWorkingUnitAcknowledgement(draft, draft.userSiteUnit, accepted);
    acknowledged = accepted;
  }
  async function setMode(mode: WorkingUnitMode): Promise<void> {
    try { await lookup.refresh(mode); }
    catch (cause) { onerror(`Working Unit mode could not be changed: ${String(cause)}`); }
  }
  function stageCopy(copy: WorkingUnitCopy): void {
    draft.userSiteUnit = copy.value;
    proposal = null;
    notice = 'BEC Master copied into the Working Unit draft. Save or Undo explicitly.';
    onchange();
  }
  function copy(): void {
    if (unavailable() || capabilities.becSiteUnit !== true) {
      onerror('Working Unit copy is unavailable while locked, loading or unsupported.');
      return;
    }
    const next = workingUnitCopy(draft);
    if (next.kind === 'invalid') { onerror(next.message); return; }
    if (next.kind === 'unchanged') { notice = next.message; return; }
    if (next.kind === 'confirm') proposal = { identity: draft, previous: draft.userSiteUnit, copy: next };
    else stageCopy(next);
  }
  function confirmCopy(): void {
    if (!proposal || unavailable() || proposal.identity !== draft ||
      proposal.previous !== draft.userSiteUnit || proposal.copy.value !== draft.becSiteUnit) {
      onerror('The copy proposal changed or became unavailable; review the current values again.');
      proposal = null;
      return;
    }
    stageCopy(proposal.copy);
  }
</script>

{#if masterEditingEnabled && masterAllowed}
  {#if masterValidation}<p role="alert">{masterValidation}</p>{/if}
  {#if draft.becSiteUnit !== (original?.becSiteUnit ?? null) && masterWarnings.length && !view.busy}
    <ul aria-label="BEC Master classification warnings">{#each masterWarnings as warning}<li>{warning}</li>{/each}</ul>
    <label><input id="master-bec-acknowledgement" type="checkbox" checked={masterAcknowledged} disabled={disabled}
      onchange={event => { rememberMasterBECAcknowledgement(draft, draft, event.currentTarget.checked); masterAcknowledged = event.currentTarget.checked; }} />
      Keep the unmatched BEC Master code exactly as entered; I have reviewed these warnings.
    </label>
  {/if}
  <datalist id={`${componentId}-master-bec`}>
    {#each masterGroups as group (group.code)}
      <option value={group.code}>{group.records.length === 1 ? group.records[0].description ?? group.code : `${group.records.length} Master definitions`}</option>
    {/each}
  </datalist>
{/if}

{#if editingEnabled}
  <div class="working-unit-toolbar" aria-label="Working Unit safety controls">
    {#if view.busy}<span role="status">Refreshing Working Unit choices and preference...</span>{/if}
    {#if view.warning}<span role="status">{view.warning}</span>{/if}
    {#if view.error}<span role="alert">{view.error}</span>{/if}
    {#if view.masterError}<span role="alert">{view.masterError}</span>{/if}
    {#if view.error || view.masterError}
      <button type="button" disabled={disabled || view.busy} onclick={() => void lookup.refresh()}>Retry Working Unit choices</button>
    {/if}
    {#if notice}<span role="status">{notice}</span>{/if}
    {#if proposal}
      <div class="copy-confirmation" role="group" aria-label="Confirm Working Unit replacement">
        <span>{proposal.copy.message}</span>
        <button id="working-unit-confirm-copy" type="button" disabled={unavailable()} onclick={confirmCopy}>Confirm draft replacement</button>
        <button type="button" disabled={disabled} onclick={() => proposal = null}>Cancel copy</button>
      </div>
    {/if}
    {#if warnings.length}
      <ul aria-label="Working Unit warnings">{#each warnings as warning}<li>{warning}</li>{/each}</ul>
      {#if changed && !view.busy}
        <label><input id="working-unit-acknowledgement" type="checkbox" checked={acknowledged} disabled={disabled}
          onchange={event => acknowledge(event.currentTarget.checked)} />
          Keep the Working Unit code exactly as entered; I have reviewed these warnings.
        </label>
      {/if}
    {/if}
  </div>
{/if}
{@render children(workingUnitInputs)}
{#if editingEnabled}
    <FieldGuidance title="Working Unit guidance and definitions">
    <p>Choice modes change only your preference. Copy stages this plot's BEC Master code; it never copies a plot, SU table or project.</p>
    {#if view.choices.some(row => !row.selectable || (row.code !== null && row.code.length > 100))}
      <span>Definitions without a valid selectable code are retained as reference metadata, not offered as blank or overlong values.</span>
    {/if}
    {#each [{ name: 'Working Unit', records: definitions }, { name: 'BEC Master (read-only)', records: masterDefinitions }] as group (group.name)}
      {#if group.records.length}
        <details>
          <summary>{group.name}: {group.records.length} definition{group.records.length === 1 ? '' : 's'}{group.records.length > 1 ? ' - code does not select one description' : ''}</summary>
          <ul>
            {#each group.records as definition (definition.rowId)}
              <li>{definition.description ?? '(description unavailable)'} | Scientific: {definition.scientificName ?? 'NULL'}
                | Source ID: {definition.sourceId ?? 'NULL'} | Origin: {definition.origin}
                <details><summary>All source metadata</summary><dl>
                  {#each Object.entries(definition) as [key, value]}
                    <dt>{key}</dt><dd>{value === null ? 'NULL' : value === '' ? '""' : String(value)}</dd>
                  {/each}
                </dl></details>
              </li>
            {/each}
          </ul>
        </details>
      {/if}
    {/each}
    </FieldGuidance>
  <datalist id={listId}>
    {#each groups as group (group.code)}
      <option value={group.code}>{group.records.length === 1 ? group.records[0].description ?? group.code : `${group.records.length} definitions; see details after choosing this code`}</option>
    {/each}
  </datalist>
{/if}

{#snippet workingUnitInputs(only?: PaperControl)}
  {#each (only ? [only] : controls).filter(control => isWorkingUnitControl(control.controlName)) as control (control.controlId)}
    {#if control.controlName === 'UserSiteUnit'}
      <input class="working-unit-control" id="header-userSiteUnit" data-source-control={control.controlName}
        data-column="UserSiteUnit" style={position(control)} aria-label="Working Unit" maxlength="100"
        value={draft.userSiteUnit ?? ''} list={editingEnabled ? listId : undefined}
        disabled={unavailable() || !control.enabled || control.locked}
        placeholder={capabilities.userSiteUnit === true ? '' : 'Pending'}
        oninput={event => setCode(event.currentTarget.value)} />
    {:else if control.controlName === 'BECSiteUnit'}
      <input class="working-unit-control" id="header-becSiteUnit" data-source-control={control.controlName}
        data-column="BECSiteUnit" style={position(control)} aria-label="BEC Master" readonly={!masterEditingEnabled || !masterAllowed}
        value={draft.becSiteUnit ?? ''} disabled={disabled || capabilities.becSiteUnit !== true || !control.enabled || (masterEditingEnabled && masterAllowed && view.busy)}
        list={masterEditingEnabled && masterAllowed ? `${componentId}-master-bec` : undefined}
        aria-invalid={masterEditingEnabled && masterValidation !== null}
        placeholder={capabilities.becSiteUnit === true ? '' : 'Pending'}
        title={masterEditingEnabled && masterAllowed ? 'Source-authorized nullable Master draft; Save or Undo explicitly.'
          : 'BEC Master remains read-only; ordinary copying does not grant editing or reverse-copy permission.'}
        oninput={event => setMasterCode(event.currentTarget.value, control)} />
    {:else if control.controlName === 'btnCoptToWorkingUnit'}
      <button class="working-unit-control" type="button" data-source-control={control.controlName} style={position(control)}
        disabled={unavailable() || capabilities.becSiteUnit !== true || !control.enabled} onclick={copy}>{control.caption.replaceAll('&', '')}</button>
    {:else if control.controlName && modeControls[control.controlName]}
      <input class="working-unit-control" id={`header-working-unit-${control.controlName}`} type="radio" name={listId} data-source-control={control.controlName}
        style={position(control)} aria-label={`Working Unit ${modeControls[control.controlName]} choices`}
        checked={view.mode === modeControls[control.controlName]} disabled={!editingEnabled || disabled || view.busy || !control.enabled}
        onchange={() => { if (control.controlName) void setMode(modeControls[control.controlName]); }} />
    {/if}
  {/each}
{/snippet}
<style>
  .working-unit-toolbar { display: grid; gap: .3rem; padding: .5rem; margin-bottom: .5rem; border: 1px solid #d6d3d1; font-size: .75rem; }
  .working-unit-toolbar ul { margin: 0; padding-left: 1.2rem; }
  .working-unit-toolbar button { padding: .2rem .4rem; border: 1px solid #a8a29e; background: white; }
  .working-unit-toolbar label { display: flex; align-items: center; gap: .35rem; }
  .copy-confirmation { display: flex; flex-wrap: wrap; align-items: center; gap: .4rem; }
  .working-unit-control { box-sizing: border-box; width: 100%; min-width: 0; min-height: 40px; border: 1px solid #b7cfc0; border-radius: 4px; padding: .5rem .65rem; background: white; font: inherit; line-height: 1.4; }
  .working-unit-control:disabled, .working-unit-control[readonly] { color: #78716c; background: #f5f5f4; }
  .working-unit-control:focus { outline: 2px solid #047857; outline-offset: 1px; }
</style>
