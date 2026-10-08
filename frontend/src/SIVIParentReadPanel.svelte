<script lang="ts">
  import source from '../../resources/fs1333-sivi-layout.json';
  import { metadataCellText } from './projectMetadataEditor';
  import { siviParentActionDisplay } from './siviParentTransport';
  import type { SIVIParentReadView } from './siviParentReadSession';
  import type { SIVIParentSourceView } from './siviParentSourceSession';
  import SIVIParentDirectField from './SIVIParentDirectField.svelte';
  import SIVIParentActionField from './SIVIParentActionField.svelte';
  import SIVIProjectAssignmentField from './SIVIProjectAssignmentField.svelte';
  import { siviProjectAssignmentDirty, type SIVIProjectAssignmentInput, type SIVIProjectAssignmentView } from './siviProjectAssignmentSession';
  import { isSIVIParentDirectColumn, type SIVIParentWriteView } from './siviParentWriteSession';
  import { siviParentDirty, siviParentOriginalCell, type SIVIParentInput } from './siviParentEditor';
  let { view, onreload, oncancel, reloadDisabled, sourceView = null,
    onSourceReload = () => {}, onSourceCancel = () => {}, onSourceChange = (_source: number) => {},
    writeView = null, writeDisabled = true, writeCanSave = false,
    onWriteOperation = (_operation: 'load' | 'save' | 'undo' | 'retain' | 'prune') => {},
    onWriteStage = (_column: string, _input: SIVIParentInput) => {}, onWriteCancel = () => {},
    actionView = null, actionDisabled = true, actionCanSave = false,
    onActionOperation = (_operation: 'load' | 'save' | 'undo' | 'retain' | 'prune') => {},
    onActionStage = (_column: string, _input: SIVIParentInput) => {}, onActionCancel = () => {},
    onMetadata = undefined, metadataDisabled = true,
    assignmentView = null, assignmentDisabled = true, assignmentCanSave = false,
    onAssignmentOperation = (_operation: 'load' | 'save' | 'undo' | 'retain' | 'prune') => {},
    onAssignmentStage = (_input: SIVIProjectAssignmentInput) => {}, onAssignmentCancel = () => {} }: {
    view: SIVIParentReadView; onreload: () => void; oncancel: () => void; reloadDisabled: boolean;
    sourceView?: SIVIParentSourceView | null; onSourceReload?: () => void; onSourceCancel?: () => void;
    onSourceChange?: (source: number) => void;
    writeView?: SIVIParentWriteView | null; writeDisabled?: boolean; writeCanSave?: boolean;
    onWriteOperation?: (operation: 'load' | 'save' | 'undo' | 'retain' | 'prune') => void;
    onWriteStage?: (column: string, input: SIVIParentInput) => void; onWriteCancel?: () => void;
    actionView?: SIVIParentWriteView | null; actionDisabled?: boolean; actionCanSave?: boolean;
    onActionOperation?: (operation: 'load' | 'save' | 'undo' | 'retain' | 'prune') => void;
    onActionStage?: (column: string, input: SIVIParentInput) => void; onActionCancel?: () => void;
    onMetadata?: () => void; metadataDisabled?: boolean;
    assignmentView?: SIVIProjectAssignmentView | null; assignmentDisabled?: boolean; assignmentCanSave?: boolean;
    onAssignmentOperation?: (operation: 'load' | 'save' | 'undo' | 'retain' | 'prune') => void;
    onAssignmentStage?: (input: SIVIProjectAssignmentInput) => void; onAssignmentCancel?: () => void;
  } = $props();
  const fields = source.forms[0].fields.filter(field => field.binding);
  const groups = [
    { name: 'Plot & location', page: null },
    { name: 'Site description', page: 'form:frmSIVIsite/Site' },
    { name: 'Vegetation', page: 'form:frmSIVIsite/Veg' },
  ];
  const displayedOriginal = $derived(assignmentView?.original ?? writeView?.original ?? actionView?.original ?? view.original);
  const writeUnsaved = $derived(writeView?.blocked || writeView && siviParentDirty(writeView.drafts));
  const actionUnsaved = $derived(actionView?.blocked || actionView && siviParentDirty(actionView.drafts));
  const assignmentUnsaved = $derived(Boolean(assignmentView?.blocked || assignmentView && siviProjectAssignmentDirty(assignmentView.drafts)));
  const actions = $derived(displayedOriginal ? siviParentActionDisplay(displayedOriginal) : []);
</script>

<section class="space-y-4 rounded border border-stone-200 bg-white p-4" data-sivi-parent-panel aria-label={writeView ? 'SIVI parent bounded editing' : 'SIVI parent read-only'}>
  <header>
    <h2 class="font-semibold">SIVI / FS1333 parent — {writeView ? 'directly bound fields' : 'read-only'}</h2>
    <p class="text-sm text-stone-600">Persisted original values, not unsaved FS882 drafts.
      {writeView ? 'Only fourteen directly bound fields can be drafted; Save independently rechecks physical source membership.' : 'Parent editing remains unavailable.'}
      {actionView ? 'Two independently enabled source actions use a separate audited Save and restoration history.' : 'Source callback workflows remain unavailable.'}
      {assignmentView ? 'Existing physical ProjectID selection uses its independently enabled audited Save and history.' : 'ProjectID assignment remains unavailable.'}
      The explicit ProjectID choice-source preference only changes YAML, not plot data.</p>
  </header>
  {#if view.error}<p role="alert" class="text-sm text-red-700">{view.error}</p>{/if}
  {#if sourceView?.error}<p role="alert" class="text-sm text-red-700">{sourceView.error}</p>{/if}
  {#if writeView?.error}<p role="alert" class="text-sm text-red-700">{writeView.error}</p>{/if}
  {#if actionView?.error}<p role="alert" class="text-sm text-red-700">{actionView.error}</p>{/if}
  {#if assignmentView?.error}<p role="alert" class="text-sm text-red-700">{assignmentView.error}</p>{/if}
  {#if assignmentView?.original && !assignmentView.available}
    <p role="status" class="text-sm text-amber-800" data-sivi-assignment-unavailable>{assignmentView.diagnostic}</p>
  {/if}
  {#if view.busy}<p role="status">Loading owned SIVI parent originals...</p>{/if}
  <div class="flex flex-wrap gap-2">
    <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-parent-reload
      disabled={reloadDisabled || view.busy} onclick={onreload}>Reload parent originals</button>
    {#if onMetadata}
      <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-metadata-open
        disabled={metadataDisabled || view.busy || Boolean(writeView?.busy || actionView?.busy || assignmentView?.busy || writeUnsaved || actionUnsaved || assignmentUnsaved)}
        onclick={onMetadata}>Edit metadata</button>
    {/if}
    {#if view.busy}
      <button type="button" class="rounded border px-3 py-1" data-sivi-parent-cancel
        disabled={Boolean(writeView?.busy || actionView?.busy || assignmentView?.busy)} onclick={oncancel}>Cancel parent read</button>
    {/if}
    {#if actionView}
      <div class="flex flex-wrap gap-2" data-sivi-action-toolbar>
        <button type="button" data-sivi-action-load disabled={actionDisabled || actionView.busy || Boolean(actionUnsaved)}
          class="rounded border px-3 py-1 disabled:opacity-50" onclick={() => onActionOperation('load')}>Load source-action editor</button>
        <button type="button" data-sivi-action-save disabled={actionDisabled || !actionCanSave}
          class="rounded border px-3 py-1 disabled:opacity-50" onclick={() => onActionOperation('save')}>Save SIVI source actions</button>
        <button type="button" data-sivi-action-undo disabled={actionView.busy || Boolean(writeUnsaved || assignmentUnsaved)}
          class="rounded border px-3 py-1 disabled:opacity-50" onclick={() => onActionOperation('undo')}>Undo / reload source actions</button>
        {#if actionView.historyId}
          <button type="button" data-sivi-action-retain disabled={actionDisabled || !actionCanSave || Boolean(actionUnsaved)}
            class="rounded border px-3 py-1 disabled:opacity-50" onclick={() => onActionOperation('retain')}>Restore actions (retain audits)</button>
          <button type="button" data-sivi-action-prune disabled={actionDisabled || !actionCanSave || Boolean(actionUnsaved)}
            class="rounded border px-3 py-1 disabled:opacity-50" onclick={() => onActionOperation('prune')}>Restore actions (prune audits)</button>
        {/if}
        {#if actionView.busy && actionView.operation === 'read'}
          <button type="button" data-sivi-action-cancel disabled={Boolean(writeView?.busy || assignmentView?.busy)}
            class="rounded border px-3 py-1" onclick={onActionCancel}>Cancel action editor read</button>
        {/if}
      </div>
      {#if actionView.busy}<p role="status">SIVI source-action operation: {actionView.operation}...</p>{/if}
    {/if}
  </div>
  {#if writeView}
    <div class="flex flex-wrap gap-2" data-sivi-direct-toolbar>
      <button type="button" data-sivi-direct-load disabled={writeDisabled || writeView.busy || Boolean(writeUnsaved)}
        class="rounded border px-3 py-1 disabled:opacity-50" onclick={() => onWriteOperation('load')}>Load directly bound editor</button>
      <button type="button" data-sivi-direct-save disabled={writeDisabled || !writeCanSave}
        class="rounded border px-3 py-1 disabled:opacity-50" onclick={() => onWriteOperation('save')}>Save SIVI parent drafts</button>
      <button type="button" data-sivi-direct-undo disabled={writeView.busy || Boolean(actionUnsaved || assignmentUnsaved)}
        class="rounded border px-3 py-1 disabled:opacity-50" onclick={() => onWriteOperation('undo')}>Undo / reload parent</button>
      {#if writeView.historyId}
        <button type="button" data-sivi-direct-retain disabled={writeDisabled || !writeCanSave || Boolean(writeUnsaved)}
          class="rounded border px-3 py-1 disabled:opacity-50" onclick={() => onWriteOperation('retain')}>Restore parent (retain audits)</button>
        <button type="button" data-sivi-direct-prune disabled={writeDisabled || !writeCanSave || Boolean(writeUnsaved)}
          class="rounded border px-3 py-1 disabled:opacity-50" onclick={() => onWriteOperation('prune')}>Restore parent (prune audits)</button>
      {/if}
      {#if writeView.busy && writeView.operation === 'read'}
        <button type="button" data-sivi-direct-cancel disabled={Boolean(actionView?.busy || assignmentView?.busy)}
          class="rounded border px-3 py-1" onclick={onWriteCancel}>Cancel editor read</button>
      {/if}
    </div>
    {#if writeView.busy}<p role="status">SIVI parent operation: {writeView.operation}...</p>{/if}
  {/if}
  {#if assignmentView}
        <div class="flex flex-wrap gap-2" data-sivi-assignment-toolbar>
          <button type="button" data-sivi-assignment-load disabled={assignmentDisabled || assignmentView.busy || assignmentUnsaved}
            class="rounded border px-3 py-1 disabled:opacity-50" onclick={() => onAssignmentOperation('load')}>Load existing ProjectID editor</button>
          <button type="button" data-sivi-assignment-save disabled={assignmentDisabled || !assignmentCanSave}
            class="rounded border px-3 py-1 disabled:opacity-50" onclick={() => onAssignmentOperation('save')}>Save ProjectID selection</button>
          <button type="button" data-sivi-assignment-undo disabled={assignmentView.busy || Boolean(writeUnsaved || actionUnsaved)}
            class="rounded border px-3 py-1 disabled:opacity-50" onclick={() => onAssignmentOperation('undo')}>Undo / reload ProjectID selection</button>
          {#if assignmentView.historyId}
            <button type="button" data-sivi-assignment-retain
              disabled={assignmentDisabled || assignmentView.busy || assignmentUnsaved || !assignmentView.original}
              class="rounded border px-3 py-1 disabled:opacity-50" onclick={() => onAssignmentOperation('retain')}>Restore ProjectID (retain audits)</button>
            <button type="button" data-sivi-assignment-prune
              disabled={assignmentDisabled || assignmentView.busy || assignmentUnsaved || !assignmentView.original}
              class="rounded border px-3 py-1 disabled:opacity-50" onclick={() => onAssignmentOperation('prune')}>Restore ProjectID (prune audits)</button>
          {/if}
          {#if assignmentView.busy && assignmentView.operation === 'read'}
            <button type="button" data-sivi-assignment-cancel disabled={Boolean(writeView?.busy || actionView?.busy)}
              class="rounded border px-3 py-1" onclick={onAssignmentCancel}>Cancel ProjectID editor read</button>
          {/if}
        </div>
        {#if assignmentView.busy}<p role="status">ProjectID assignment operation: {assignmentView.operation}...</p>{/if}
  {/if}
  {#if displayedOriginal}
    {@const original = displayedOriginal}
    <p class="text-sm" data-sivi-parent-owner>Project {original.Project}; plot {original.Plot};
      Env row {original.Rows[0].Env.rowId}; Admin row {original.Rows[0].Admin.rowId}</p>
    {#each groups as group}
      <section class="space-y-2" aria-label={group.name}>
        <h3 class="font-semibold">{group.name}</h3>
        <dl class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {#each fields.filter(field => ('pageId' in field ? field.pageId ?? null : null) === group.page) as field (field.controlId)}
            {@const binding = original.Bindings.find(binding => binding.ControlID === field.controlId)}
            {#if binding}
              {@const cell = original.Rows[0][binding.Table === original.EnvTable ? 'Env' : 'Admin'].cells[binding.Column]}
              <div class="min-w-0 rounded border border-stone-100 p-2" data-sivi-parent-field={binding.Binding}>
                <dt class="text-sm font-medium">
                  {#if assignmentView?.original && assignmentView.choices && binding.Binding === 'ProjectID'}
                    <label for="sivi-project-assignment">{field.caption || field.controlName || binding.Binding}</label>
                  {:else if actionView?.original && binding.Binding === 'PlotType'}
                    <label for="sivi-action-PlotType">Plot Type</label>
                  {:else if writeView?.original && isSIVIParentDirectColumn(binding.Binding)}
                    <label for={`sivi-direct-${binding.Binding}`}>{field.caption || field.controlName || binding.Binding}</label>
                  {:else}{field.caption || field.controlName}{/if}
                </dt>
                {#if assignmentView?.original && assignmentView.choices && binding.Binding === 'ProjectID'}
                  <SIVIProjectAssignmentField original={cell} choices={assignmentView.choices} drafts={assignmentView.drafts}
                    disabled={assignmentDisabled || assignmentView.busy || assignmentView.blocked || !assignmentView.available}
                    onstage={onAssignmentStage} />
                {:else if actionView?.original && binding.Binding === 'PlotType'}
                  <SIVIParentActionField column="PlotType"
                    original={siviParentOriginalCell(actionView.original, 'PlotType')} draft={actionView.drafts.PlotType}
                    disabled={actionDisabled || actionView.busy || actionView.blocked} onstage={input => onActionStage('PlotType', input)} />
                {:else if writeView?.original && isSIVIParentDirectColumn(binding.Binding)}
                  <SIVIParentDirectField column={binding.Binding} original={cell} draft={writeView.drafts[binding.Binding]}
                    disabled={writeDisabled || writeView.busy || writeView.blocked} caption={field.caption || field.controlName || binding.Binding}
                    onstage={input => onWriteStage(binding.Binding, input)} />
                {:else}
                <dd class="whitespace-pre-wrap break-words" data-storage={cell.storage}>{cell.storage === 'null' ? 'NULL'
                  : cell.storage === 'text' && cell.text === '' ? '(empty text)' : metadataCellText(cell)}</dd>
                {/if}
                <dd class="text-xs text-stone-500">{binding.Binding} · {cell.storage}</dd>
              </div>
            {/if}
          {/each}
        </dl>
        {#if group.page === 'form:frmSIVIsite/Veg' && actionView?.original}
          <dl class="rounded border border-stone-100 p-2">
            <dt class="text-sm font-medium"><label for="sivi-action-SpeciesListComplete">Spp List (species list complete)</label></dt>
            <SIVIParentActionField column="SpeciesListComplete"
              original={siviParentOriginalCell(actionView.original, 'SpeciesListComplete')} draft={actionView.drafts.SpeciesListComplete}
              disabled={actionDisabled || actionView.busy || actionView.blocked} onstage={input => onActionStage('SpeciesListComplete', input)} />
          </dl>
        {/if}
      </section>
    {/each}
    <section class="space-y-2" aria-label="Source option display">
      <h3 class="font-semibold">Persisted source option display (not a draft)</h3>
      {#each actions as action}
        <p class="text-sm" data-sivi-parent-action={action.controlId}>
          {action.controlId.endsWith('/optPlotType') ? 'Plot type' : 'Species list complete'}:
          selected option {action.option ?? 'none'}; original {action.expected.storage === 'null' ? 'NULL'
            : action.expected.storage === 'text' && action.expected.text === '' ? '(empty text)' : metadataCellText(action.expected)}
          ({action.expected.storage})</p>
        {#if action.diagnostic}<p class="text-sm text-amber-800" role="status">{action.diagnostic}</p>{/if}
      {/each}
    </section>
    {#if sourceView}
      <section class="space-y-2" aria-label="ProjectID source and parent join review" data-sivi-source-panel>
        <h3 class="font-semibold">ProjectID choice source</h3>
        {#if sourceView.join}
          <p class="text-sm" data-sivi-join-verified={sourceView.join.Verified}>
            {sourceView.join.Diagnostic} Scope: {sourceView.join.Scope}.</p>
        {/if}
        {#if sourceView.busy}
          <p role="status">{sourceView.saving ? 'Committing ProjectID source preference...' : 'Reading owned source and metadata choices...'}</p>
          {#if !sourceView.saving}
            <button type="button" class="rounded border px-3 py-1" data-sivi-source-cancel onclick={onSourceCancel}>Cancel source read</button>
          {/if}
        {/if}
        <div class="flex flex-wrap gap-2">
          <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-source-env
            aria-pressed={sourceView.choices?.SourceOption === 1}
            disabled={reloadDisabled || sourceView.busy || !sourceView.choices} onclick={() => onSourceChange(1)}>Env</button>
          <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-source-master
            aria-pressed={sourceView.choices?.SourceOption === 2}
            disabled={reloadDisabled || sourceView.busy || !sourceView.choices} onclick={() => onSourceChange(2)}>Master</button>
          <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-source-reload
            disabled={reloadDisabled || sourceView.busy} onclick={onSourceReload}>Read ProjectID choices</button>
        </div>
        {#if sourceView.choices}
          <p class="text-sm" data-sivi-source-owner>{sourceView.choices.Source}: {sourceView.choices.Alias}.{sourceView.choices.Table}.
            Physical definitions are read-only; duplicates, NULL and empty text are distinct.</p>
          <div class="max-h-64 overflow-auto">
            <table class="w-full text-left text-sm">
              <thead><tr><th>Physical row</th><th>Project ID</th><th>Project title</th></tr></thead>
              <tbody>
                {#each sourceView.choices.Choices.rows as row (row.rowId)}
                  <tr data-sivi-project-choice-row={row.rowId}>
                    <th class="border p-2" scope="row">{row.rowId}</th>
                    {#each row.cells as cell}
                      <td class="border p-2 whitespace-pre-wrap break-words" data-storage={cell.storage}>
                        {cell.storage === 'null' ? 'NULL' : cell.storage === 'text' && cell.text === '' ? '(empty text)' : metadataCellText(cell)}
                        <span class="block text-xs text-stone-500">{cell.storage}</span>
                      </td>
                    {/each}
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
        <p class="text-xs text-stone-600">Switching Env/Master is an immediate shared source preference, not an unsaved plot draft.
          Metadata creation and automatic NotInList completion remain unavailable.
          {assignmentView ? 'Assignment requires the separately loaded existing-definition editor.' : 'ProjectID assignment remains unavailable.'}</p>
      </section>
    {/if}
    {#if assignmentView?.choices}
      <p class="text-xs text-stone-600">ProjectID selection uses {assignmentView.choices.Source}:
        {assignmentView.choices.Alias}.{assignmentView.choices.Table}. Physical duplicates and NULL/empty titles remain distinct.
        {assignmentView.diagnostic} Availability is not membership or commit authorization.
        Explicit audited Save/reload is a desktop adaptation, not NotInList completion or metadata creation.</p>
    {/if}
    {#if actionView}<p class="text-xs text-stone-600">Desktop adaptation: explicit audited Save, then owned parent reload.
      Plot Type carries the verified source Refresh directive; Spp List does not.
      Neither action implicitly saves other drafts. Normal-form NULL Plot Type remains unavailable; this is not the CHARS Variant callback.</p>{/if}
    <p class="text-xs text-stone-600">Literal SQLite parent membership is a labelled adaptation, not full Access join or write authorization. NULL, empty text and historical storage classes are retained.</p>
  {/if}
</section>
