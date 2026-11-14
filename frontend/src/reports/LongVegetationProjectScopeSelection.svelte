<script lang="ts">
  import { onDestroy, untrack } from 'svelte';
  import type { ProjectScopeSelectionSession } from './longVegetationProjectScopeSession';

  let { selection, allPlotsAvailable, cancelReads, onChange, operationHeld = false, mutationHeld = false }: {
    selection: ProjectScopeSelectionSession;
    allPlotsAvailable: boolean;
    cancelReads: () => void;
    onChange: () => void;
    operationHeld?: boolean;
    mutationHeld?: boolean;
  } = $props();
  let view = $state.raw(untrack(() => selection.view()));
  let error = $state('');
  const unsubscribe = untrack(() => selection.subscribe(() => { view = selection.view(); onChange(); }));
  let generation = 0;

  function cancel() {
    generation++;
    cancelReads();
    selection.cancel();
  }
  async function run(operation: () => Promise<void>) {
    const request = ++generation;
    error = '';
    try { await operation(); }
    catch (cause) { if (request === generation) error = String(cause); }
  }
  async function load() {
    if (operationHeld || view.busy) return;
    await run(() => selection.load());
  }
  async function switchScope(raw: string) {
    if (operationHeld || mutationHeld || view.busy || view.authorityUnknown || !view.scope) return;
    if (raw !== 'selected-list' && raw !== 'project-selected-fields' && raw !== 'project-all-plots') {
      error = 'Choose an explicit report source scope.';
      return;
    }
    await run(() => selection.switchTo(raw));
  }
  async function build() {
    if (operationHeld || mutationHeld || view.busy || view.authorityUnknown || !view.scope ||
        view.scope.sourceKind === 'selected-list' ||
        view.scope.sourceKind === 'project-selected-fields' && !!view.draftError) return;
    await run(() => selection.build());
  }
  function edit(fields: string[]) {
    if (mutationHeld || view.busy || !view.scope) return;
    error = '';
    try { selection.edit(fields); }
    catch (cause) { error = String(cause); }
  }
  function move(index: number, offset: number) {
    const fields = [...view.fields], other = index + offset;
    if (other < 0 || other >= fields.length) return;
    [fields[index], fields[other]] = [fields[other], fields[index]];
    edit(fields);
  }
  function undo() {
    if (mutationHeld || view.busy || !view.scope) return;
    error = '';
    try { selection.undo(); }
    catch (cause) { error = String(cause); }
  }
  onDestroy(() => { unsubscribe(); cancel(); });
</script>

<section class="selection" aria-label="Project report source and unit fields" data-project-report-scope>
  {#if error}<p role="alert" class="error">{error}</p>{/if}
  {#if view.error}<p role="alert" class="error">{view.error}</p>{/if}
  {#if view.draftError}<p role="alert" class="error">{view.draftError}</p>{/if}
  {#if view.authorityUnknown}
    <p role="alert">Report scope authority is unknown. Wait for the pending mutation to settle, then Load owned scope.
      No field Save, Build or scope switch is replayed.</p>
  {/if}
  <label for="project-report-source">
    <span>Report source scope</span>
    <select id="project-report-source" data-project-report-source value={view.scope?.sourceKind ?? ''}
      disabled={operationHeld || mutationHeld || view.busy || view.authorityUnknown || !view.scope}
      onchange={event => switchScope(event.currentTarget.value)}>
      <option value="" disabled>Load owned scope</option>
      <option value="selected-list">Current selected site-unit list</option>
      <option value="project-selected-fields">Whole project: Selected fields</option>
      <option value="project-all-plots" disabled={!allPlotsAvailable}>Whole project: All plots (None SU)</option>
    </select>
  </label>
  {#if view.scope?.sourceKind === 'project-selected-fields'}
    <div class="lists" data-project-report-field-lists>
      <div>
        <h4>Field List</h4>
        <ul>
          {#each view.definitions as field, index}
            <li>
              <span>{field.table}.{field.name} ({field.declaredType || 'untyped'})</span>
              <button data-project-field-add={index} aria-label={`Add ${field.table}.${field.name}`}
                disabled={mutationHeld || view.busy || !field.available || view.fields.some(name => name.toLowerCase() === field.name.toLowerCase())}
                onclick={() => edit([...view.fields, field.name])}>Add</button>
              {#if !field.available}<small>{field.reason}</small>{/if}
            </li>
          {/each}
        </ul>
      </div>
      <div>
        <h4>Selected List</h4>
        <ol>
          {#each view.fields as name, index}
            <li>
              <span>{name}</span>
              <div class="actions">
                <button data-project-field-up={index} aria-label={`Move ${name} up`}
                  disabled={mutationHeld || view.busy || index === 0} onclick={() => move(index, -1)}>Up</button>
                <button data-project-field-down={index} aria-label={`Move ${name} down`}
                  disabled={mutationHeld || view.busy || index === view.fields.length - 1} onclick={() => move(index, 1)}>Down</button>
                <button data-project-field-remove={index} aria-label={`Remove ${name}`} disabled={mutationHeld || view.busy}
                  onclick={() => edit(view.fields.filter((_, i) => i !== index))}>Remove</button>
              </div>
            </li>
          {/each}
        </ol>
      </div>
    </div>
  {:else}
    <details data-project-inactive-fields>
      <summary>Retained inactive Selected List draft{view.dirty ? ' (unsaved)' : ''}</summary>
      <ol>{#each view.fields as name}<li>{name}</li>{/each}</ol>
      <p>This draft is retained, but is not used by the current source scope. Switch explicitly to Selected fields to correct it,
        or Undo the field draft.</p>
    </details>
  {/if}
  <div class="actions">
    <button data-project-scope-load disabled={operationHeld || view.busy} onclick={load}>Load owned scope and Field List</button>
    <button data-project-scope-build disabled={operationHeld || mutationHeld || view.busy || view.authorityUnknown || !view.scope ||
      view.scope.sourceKind === 'selected-list' || view.scope.sourceKind === 'project-selected-fields' && !!view.draftError}
      onclick={build}>Build project unit list</button>
    <button data-project-field-undo disabled={mutationHeld || view.busy || !view.scope} onclick={undo}>Undo field draft</button>
    {#if view.busy}
      <button data-project-scope-cancel onclick={cancel}>Cancel scope observation</button>
      <p role="status">Reading or changing owned report scope. Cancel does not cancel an issued mutation.</p>
    {/if}
  </div>
  {#if view.scope?.build}
    <div class="receipt" data-project-build-receipt>
      <p role="status">Read-only Build receipt: {view.scope.build.projectTuples.length} original project tuples.
        {view.outputReady ? 'Source approval is current.' : 'Load or Build explicitly before output.'}</p>
      <p>Approval: <code>{view.scope.build.approvalHash}</code></p>
    </div>
  {/if}
  <p class="guidance">Fields concatenate in Selected List order without a separator. Whole-project Selected fields uses original Env/Admin
    rows, not the current selected site-unit list. All plots uses original Env and requires the actual None SU context.
    Build is read-only: it does not save Unit Groups or change project data, audits or source scratch tables.
    After a current Build, Load actual saved Unit Groups and Save the matching choice explicitly.</p>
</section>

<style>
  .selection { display: grid; grid-template-columns: minmax(0, 1fr); gap: .75rem; min-width: 0; }
  .lists { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1rem; }
  .lists > div, .receipt, details { min-width: 0; }
  label { display: grid; gap: .35rem; min-width: 0; }
  select { width: 100%; box-sizing: border-box; min-width: 0; border: 1px solid #cbd5e1; border-radius: .4rem; padding: .5rem; }
  ul, ol { padding-left: 1.25rem; margin: .5rem 0; }
  li { margin-bottom: .65rem; overflow-wrap: anywhere; }
  li > span, small { display: block; }
  .actions { display: flex; flex-wrap: wrap; gap: .5rem; align-items: center; }
  button { border: 1px solid #cbd5e1; border-radius: .4rem; padding: .5rem .75rem; background: white; }
  button:disabled { opacity: .5; }
  summary { cursor: pointer; padding: .5rem 0; }
  .receipt code, p, span { overflow-wrap: anywhere; }
  .error { color: #b91c1c; }
  .guidance, small { color: #57534e; font-size: .875rem; }
  @media (max-width: 640px) { .lists { grid-template-columns: minmax(0, 1fr); } }
</style>
