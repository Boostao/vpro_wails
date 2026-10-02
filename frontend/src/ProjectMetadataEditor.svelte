<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import type { ProjectMetadataCreate, ProjectMetadataEditorField, ProjectMetadataReview } from '../bindings/github.com/boostao/vpro-wails';
  import type { bindContextPlots } from './contextPlots';
  import type { EditorCloseState } from './closeLifecycle';
  import { ReadRequests } from './readRequests';
  import { beginMetadataDraft, stageMetadata, metadataFieldValue, metadataErrors, metadataDirty, metadataEditRequest,
    metadataCellText, metadataStandardDecisionRequired, metadataStandardDefaults, populateMetadataStandard, metadataBlankRequest,
    beginMetadataTemplate, stageMetadataTemplate, metadataTemplateRequest, metadataTemplateFields, type MetadataDraft, type MetadataTemplateDraft } from './projectMetadataEditor';
  import { metadataGroups, metadataLabel } from './projectMetadataPresentation';

  let { client, plot, onclosed, onbusy, oncommitted, allowCreation = false, allowTemplateCreation = false }: {
    client: ReturnType<typeof bindContextPlots>; plot: string; onclosed: () => void;
    onbusy: (busy: boolean) => void; oncommitted: () => Promise<void>;
    allowCreation?: boolean;
    allowTemplateCreation?: boolean;
  } = $props();
  let review = $state<ProjectMetadataReview | null>(null);
  let fields = $state<ProjectMetadataEditorField[]>([]);
  let draft = $state<MetadataDraft | null>(null);
  let blank = $state<ProjectMetadataCreate | null>(null);
  let template = $state<MetadataTemplateDraft | null>(null);
  let reading = $state(false);
  let ready = $state(false);
  let standardPreview = $state(false);
  let saving = $state(false);
  let committedFailure = $state(false);
  let error = $state<string | null>(null);
  let success = $state<string | null>(null);
  let generation = 0;
  const reads = new ReadRequests();
  const busy = $derived(reading || saving);
  const errors = $derived(draft ? metadataErrors(draft) : template ? metadataErrors(template) : []);
  const dirty = $derived(blank !== null || template !== null || (draft ? metadataDirty(draft) : false));
  const standardDecision = $derived(draft ? metadataStandardDecisionRequired(draft) : false);
  const rows = $derived(review?.projectRecords.rows ?? []);
  const available = $derived.by(() => {
    const columns = new Set(review?.projectRecords.columns?.map(column => column.name));
    return fields.filter(field => columns.has(field.name));
  });
  $effect(() => onbusy(busy));
  onMount(() => { void reload(); });
  onDestroy(() => { generation++; reads.cancelAll(); onbusy(false); });

  export function getCloseState(): EditorCloseState {
    return { unsaved: true, busy, canSave: false, error,
      saveReason: 'Save or Undo metadata explicitly, then close metadata review before saving or closing the plot.' };
  }
  export function undo() {
    if (busy) { error = 'Wait for the current metadata operation before Undo.'; return; }
    if (template) {
      template = null; error = null; success = 'Template creation proposal undone; no record, identity or history was allocated.';
    } else if (blank) {
      blank = null; error = null; success = 'Blank creation proposal undone; no record, identity or history was allocated.';
    } else if (draft && !committedFailure) {
      draft = beginMetadataDraft(draft.review, draft.original.rowId);
      standardPreview = false; error = null; success = 'Metadata drafts undone; no data or history changed.';
    } else onclosed();
  }
  function reviewBlank() {
    if (!allowCreation || !ready || !review || busy || dirty || committedFailure) {
      error = 'Reload an available empty metadata review before proposing blank creation.'; return;
    }
    try { blank = metadataBlankRequest(review); error = null; success = null; }
    catch (cause) { error = `Blank creation proposal unavailable; no data changed: ${String(cause)}`; }
  }
  async function createBlank() {
    if (!allowCreation || !ready || !blank || busy || committedFailure) {
      error = 'Explicitly review the empty metadata proposal before creation.'; return;
    }
    let committed = false;
    saving = true; error = null; success = null;
    try {
      const created = await client.CreateBlankProjectMetadata(blank);
      committed = true; blank = null;
      await oncommitted();
      const next = await client.ReviewProjectMetadata(plot);
      review = next; draft = beginMetadataDraft(next, created.rowId);
      success = 'Blank metadata record and full creation audit committed atomically. All73 ordinary/stamp fields remain NULL; edit and Save separately.';
    } catch (cause) {
      if (committed || String(cause).includes('metadata edit committed, but')) {
        committedFailure = true; blank = null; draft = null;
        error = `Metadata creation committed, but refresh/cleanup failed. Reload before another edit; completed writes must not be replayed: ${String(cause)}`;
      } else error = `Blank metadata creation failed; reviewed proposal retained for retry: ${String(cause)}`;
    } finally { saving = false; }
  }
  function reviewTemplate(rowId: string) {
    if (!allowTemplateCreation || !ready || !review || busy || dirty || committedFailure) {
      error = 'Reload an available empty metadata review before selecting a master template.'; return;
    }
    try { template = beginMetadataTemplate(review, rowId, available); error = null; success = null; }
    catch (cause) { error = `Template proposal unavailable; no data changed: ${String(cause)}`; }
  }
  function stageTemplate(field: ProjectMetadataEditorField, raw: string, nullValue: boolean) {
    if (!ready || !template || busy || committedFailure) { error = 'Reload available metadata before reviewing template assignments.'; return; }
    try { template = stageMetadataTemplate(template, field, raw, nullValue); error = null; success = null; }
    catch (cause) { error = `Template draft failed; proposal retained: ${String(cause)}`; }
  }
  async function createTemplate() {
    if (!allowTemplateCreation || !ready || !template || busy || committedFailure || errors.length) {
      error = 'Resolve all explicit template assignments before creation; raw errors were retained.'; return;
    }
    let committed = false;
    saving = true; error = null; success = null;
    try {
      const created = await client.CreateProjectMetadataFromTemplate(metadataTemplateRequest(template));
      committed = true; template = null;
      await oncommitted();
      const next = await client.ReviewProjectMetadata(plot);
      review = next; draft = beginMetadataDraft(next, created.rowId);
      success = 'One reviewed template record and full creation audit committed atomically. Unmapped fields/stamps remain NULL; ordinary editing and Save are separate.';
    } catch (cause) {
      if (committed || String(cause).includes('metadata edit committed, but')) {
        committedFailure = true; template = null; draft = null;
        error = `Template creation committed, but refresh/cleanup failed. Reload before another edit; completed writes must not be replayed: ${String(cause)}`;
      } else error = `Template creation failed; reviewed assignments retained for retry: ${String(cause)}`;
    } finally { saving = false; }
  }
  async function reload() {
    if (saving || dirty) { error = 'Save or Undo metadata drafts before reloading; raw errors were retained.'; return; }
    const request = ++generation;
    reads.cancelAll(); reading = true; ready = false; error = null;
    try {
      const [next, definitions] = await Promise.all([
        reads.track(client.ReviewProjectMetadata(plot)), reads.track(client.ListProjectMetadataFields()),
      ]);
      if (request !== generation) return;
      if (!next.projectRecords.columns || !next.projectRecords.rows || !definitions?.length) {
        throw new Error('Complete metadata records and source field definitions are required.');
      }
      review = next; fields = definitions; draft = null; committedFailure = false; ready = true; standardPreview = false;
    } catch (cause) {
      if (request === generation) error = `Metadata review/references unavailable; no data changed: ${String(cause)}`;
    } finally { if (request === generation) reading = false; }
  }
  function select(rowId: string) {
    if (!ready || !review || busy || dirty || committedFailure) { error = 'Reload available metadata and references, then Save or Undo the existing draft before selecting another physical record.'; return; }
    try {
      draft = beginMetadataDraft(review, rowId); standardPreview = false; error = null; success = null;
    } catch (cause) { error = `Metadata selection failed: ${String(cause)}`; }
  }
  function stage(field: ProjectMetadataEditorField, raw: string, nullValue: boolean) {
    if (!ready || !draft || busy || committedFailure) { error = 'Reload available metadata and references before editing.'; return; }
    try {
      draft = stageMetadata(draft, field, raw, nullValue);
      if (field.name === 'EcosysCollectionStandard') standardPreview = false;
      error = null; success = null;
    }
    catch (cause) { error = `Metadata draft failed; entry retained: ${String(cause)}`; }
  }
  function populateStandard() {
    if (!ready || !draft || busy || committedFailure || !standardPreview) {
      error = 'Review the available source defaults before applying them to drafts.'; return;
    }
    try {
      draft = populateMetadataStandard(draft, available);
      standardPreview = false; error = null;
      success = '24 source defaults staged, not saved. Other drafts and raw errors were retained; review before Save.';
    } catch (cause) { error = `Source-default proposal failed; drafts retained: ${String(cause)}`; }
  }
  async function save() {
    if (!ready || !draft || busy || committedFailure) { error = 'Reload available metadata and references, then select an existing record before metadata Save.'; return; }
    if (standardPreview) { error = 'Apply or cancel the source-default preview before metadata Save.'; return; }
    let committed = false;
    saving = true; error = null; success = null;
    try {
      const request = metadataEditRequest(draft);
      await client.SaveProjectMetadata(request);
      committed = true; draft = null; standardPreview = false;
      await oncommitted();
      const next = await client.ReviewProjectMetadata(plot);
      review = next; draft = beginMetadataDraft(next, request.original.rowId);
      success = 'Metadata fields, version/date stamps and applicable audits saved atomically. Other candidates and master templates were not copied.';
    } catch (cause) {
      if (committed || String(cause).includes('metadata edit committed, but')) {
        committedFailure = true; draft = null;
        error = `Metadata committed, but refresh/cleanup failed. Reload before another edit; completed writes must not be replayed: ${String(cause)}`;
      } else error = `Metadata Save failed; raw drafts retained for correction or retry: ${String(cause)}`;
    } finally { saving = false; }
  }
  function close() {
    if (busy || dirty || errors.length) { error = 'Save or Undo metadata drafts before closing; invalid raw entries cannot be discarded implicitly.'; return; }
    onclosed();
  }
  function summary(row: import('../bindings/github.com/boostao/vpro-wails').ProjectMetadataRow, master = false) {
    const index = (master ? review?.masterTemplates : review?.projectRecords)?.columns?.findIndex(column => column.name === 'ProjectTitle') ?? -1;
    const value = row.cells?.[index];
    return value?.storage === 'null' ? 'NULL title' : value?.text === '' ? 'Empty title' : value ? metadataCellText(value) : 'Unavailable title';
  }
</script>

<section class="metadata-editor" aria-label="Project metadata editor" data-metadata-editor>
  <header><h2>Project metadata — {review?.project ?? 'loading'}</h2>
    <p>Plot {plot}; Project ID {review?.projectId === null ? 'NULL (existing editing unavailable)' : JSON.stringify(review?.projectId)}</p>
  </header>
  {#if error}<p class="failure" role="alert">{error}</p>{/if}
  {#if success}<p class="success" role="status">{success}</p>{/if}
  {#if errors.length}<ul class="failure" role="alert">{#each errors as message}<li>{message}</li>{/each}</ul>{/if}
  {#if busy}<p role="status">{saving ? 'Saving metadata transaction…' : 'Loading metadata records and source references…'}</p>{/if}
  <div class="toolbar">
    <button type="button" onclick={reload} disabled={busy || dirty}>Reload metadata and references</button>
    <button type="button" onclick={undo} disabled={busy || committedFailure}>Undo metadata drafts</button>
    <button type="button" onclick={save} disabled={!ready || busy || !draft || !dirty || errors.length > 0 || committedFailure || standardPreview || standardDecision && !draft?.standardPopulation}>Save metadata</button>
    <button type="button" onclick={close} disabled={busy || dirty || errors.length > 0}>Close metadata review</button>
  </div>
  <fieldset disabled={!ready || busy || dirty || committedFailure}><legend>Select an existing physical record explicitly</legend>
    {#each rows as row (row.rowId)}
      <button type="button" data-metadata-row={row.rowId} aria-pressed={draft?.original.rowId === row.rowId} onclick={() => select(row.rowId)}>Record {row.rowId}: {summary(row)}</button>
    {:else}<p>No matching existing metadata records.
      {allowCreation || allowTemplateCreation ? 'Review an explicitly enabled creation proposal below.' : 'New/blank/template proposals remain unavailable.'}</p>{/each}
  </fieldset>
  {#if allowCreation && rows.length === 0 && !blank && !template}
    <button type="button" disabled={!ready || busy || dirty || committedFailure} onclick={reviewBlank}>Review blank metadata creation</button>
  {/if}
  {#if allowTemplateCreation && rows.length === 0 && !blank && !template}
    <fieldset disabled={!ready || busy || dirty || committedFailure}><legend>Select one physical master template explicitly</legend>
      {#each review?.masterTemplates.rows ?? [] as row (row.rowId)}
        <button type="button" data-metadata-template={row.rowId} onclick={() => reviewTemplate(row.rowId)}>Review master template {row.rowId}: {summary(row, true)}</button>
      {:else}<p>No matching master templates. No first-row choice, identity assignment or template is inferred.</p>{/each}
    </fieldset>
  {/if}
  {#if template}
    <fieldset data-metadata-template-proposal><legend>Review32 assignments from master record {template.original.rowId}</legend>
      <p>Existing Project ID {JSON.stringify(template.review.projectId)} remains unchanged.
        Source timestamps require an explicit signed16 year or NULL; incompatible codes require literal text or NULL.
        No source value is silently converted. Unmapped fields and version/date stamps remain NULL.
        Only one record is created; all other master candidates remain untouched.</p>
      <div class="metadata-grid">
        {#each metadataTemplateFields as name (name)}
          {@const field = available.find(field => field.name === name)}
          {@const cell = template.cells[name]}
          {@const index = template.review.masterTemplates.columns?.findIndex(column => column.name === name) ?? -1}
          {@const source = template.original.cells?.[index]}
          {#if field && cell}
            <div class="metadata-field" class:wide={name === 'Notes' || name === 'FieldDataCollectionTeam' || name === 'ProjectPurpose'}>
              <label for={'metadata-template-' + name}>{metadataLabel(name)}</label>
              <p>Master source: {source?.storage === 'null' ? 'NULL' : source ? `${source.storage}: ${JSON.stringify(metadataCellText(source))}` : 'Unavailable'}</p>
              {#if name === 'Notes'}
                <textarea id={'metadata-template-' + name} rows="4" value={cell.raw}
                  disabled={!ready || busy || committedFailure || cell.nullValue} aria-invalid={cell.error !== null}
                  oninput={event => stageTemplate(field, event.currentTarget.value, false)}></textarea>
              {:else if field.collection}
                <select id={'metadata-template-' + name} value={cell.raw}
                  disabled={!ready || busy || committedFailure || cell.nullValue} aria-invalid={cell.error !== null}
                  onchange={event => stageTemplate(field, event.currentTarget.value, false)}>
                  {#if !['1', '2', '3'].includes(cell.raw)}<option value={cell.raw}>Unresolved source {cell.raw || 'NULL'}</option>{/if}
                  <option value="1">Complete</option><option value="2">Partial</option><option value="3">None</option>
                </select>
              {:else}
                <input id={'metadata-template-' + name} type="text" inputmode={field.kind === 'integer' ? 'numeric' : 'text'}
                  list={field.referenceList ? 'metadata-template-options-' + name : undefined} value={cell.raw}
                  disabled={!ready || busy || committedFailure || cell.nullValue} aria-invalid={cell.error !== null}
                  oninput={event => stageTemplate(field, event.currentTarget.value, false)} />
                {#if field.referenceList}
                  <datalist id={'metadata-template-options-' + name}>{#each field.options ?? [] as option, i (i)}
                    {#if option.value !== null}<option value={option.value}>{option.description ?? ''}</option>{/if}
                  {/each}</datalist>
                {/if}
              {/if}
              <label class="null-option" for={'metadata-template-null-' + name}>
                <input id={'metadata-template-null-' + name} type="checkbox" checked={cell.nullValue}
                  disabled={!ready || busy || committedFailure} onchange={event => stageTemplate(field, cell.raw, event.currentTarget.checked)} />
                Store NULL
              </label>
              {#if cell.error}<p class="failure">{cell.error}</p>{/if}
            </div>
          {/if}
        {/each}
      </div>
      <button type="button" disabled={!ready || busy || committedFailure || errors.length > 0} onclick={createTemplate}>Create reviewed template metadata record</button>
    </fieldset>
  {/if}
  {#if blank}
    <fieldset data-metadata-blank-proposal><legend>Confirm one blank record for the existing parent identity</legend>
      <p>Project ID {JSON.stringify(blank.projectId)} is copied literally from the selected plot, never assigned to it.
        All73 other source fields, including version/date stamps, remain NULL. No master template is copied.
        A reserved signed32 ID is allocated only by the creation transaction.</p>
      <button type="button" disabled={!ready || busy || committedFailure} onclick={createBlank}>Create reviewed blank metadata record</button>
    </fieldset>
  {/if}
  {#if draft}
    <p>Physical ID {draft.id}; immutable Project ID {JSON.stringify(draft.review.projectId)}. Unchanged historical values are omitted from Save.</p>
    {#if standardDecision}
      <label class="decision"><input type="checkbox" data-metadata-standard-keep disabled={!ready || busy || committedFailure}
        checked={draft.standardPopulation === 'keep'} onchange={event => { if (draft) draft = { ...draft, standardPopulation: event.currentTarget.checked ? 'keep' : '' }; }} />
        Use current field drafts without further source population.</label>
      <button type="button" disabled={!ready || busy || committedFailure} onclick={() => standardPreview = true}>Review 24 source defaults</button>
      {#if draft.standardPopulation === 'populate'}<p>Source defaults were explicitly staged; they remain editable drafts until Save.</p>{/if}
      {#if standardPreview}
        <fieldset data-metadata-standard-preview><legend>Confirm replacement of these 24 field drafts</legend>
          <p>No data has been saved. Apply replaces the listed current drafts, including their raw errors; all other drafts remain unchanged.</p>
          <dl>{#each Object.entries(metadataStandardDefaults) as [name, value] (name)}
            {@const field = available.find(field => field.name === name)}
            <dt>{metadataLabel(name)}</dt><dd>
              Current: {field ? metadataFieldValue(draft, field).nullValue ? 'NULL' : JSON.stringify(metadataFieldValue(draft, field).raw) : 'Unavailable'};
              proposed: {JSON.stringify(value)}
            </dd>
          {/each}</dl>
          <button type="button" disabled={!ready || busy || committedFailure} onclick={populateStandard}>Apply 24 source defaults to drafts</button>
          <button type="button" disabled={busy} onclick={() => standardPreview = false}>Cancel source-default preview</button>
        </fieldset>
      {/if}
    {/if}
    {#each metadataGroups as group (group.title)}
      <fieldset><legend>{group.title}</legend><div class="metadata-grid">
        {#each group.names as name (name)}
          {@const field = available.find(field => field.name === name)}
          {#if field}
            {@const cell = metadataFieldValue(draft, field)}
            <div class="metadata-field" class:wide={name === 'Notes' || name === 'FieldDataCollectionTeam' || name === 'ProjectPurpose'}>
              <label for={'metadata-' + name}>{metadataLabel(name)}</label>
              {#if name === 'Notes'}
                <textarea id={'metadata-' + name} data-metadata-column={name} rows="4" value={cell.raw}
                  disabled={!ready || busy || committedFailure || cell.nullValue} aria-invalid={cell.error !== null}
                  oninput={event => stage(field, event.currentTarget.value, false)}></textarea>
              {:else if field.collection}
                <select id={'metadata-' + name} data-metadata-column={name} value={cell.raw}
                  disabled={!ready || busy || committedFailure || cell.nullValue} aria-invalid={cell.error !== null}
                  onchange={event => stage(field, event.currentTarget.value, false)}>
                  {#if !['1', '2', '3'].includes(cell.raw)}<option value={cell.raw}>Historical {cell.raw || 'NULL'} (unchanged only)</option>{/if}
                  <option value="1">Complete</option><option value="2">Partial</option><option value="3">None</option>
                </select>
              {:else}
                <input id={'metadata-' + name} data-metadata-column={name} type="text" inputmode={field.kind === 'integer' ? 'numeric' : 'text'}
                  list={field.referenceList ? 'metadata-options-' + name : undefined} value={cell.raw}
                  disabled={!ready || busy || committedFailure || cell.nullValue} aria-invalid={cell.error !== null}
                  oninput={event => stage(field, event.currentTarget.value, false)} />
                {#if field.referenceList}<datalist id={'metadata-options-' + name}>
                  {#each field.options ?? [] as option (option.rowId)}
                    {#if option.value !== null}<option value={option.value} label={option.description ?? option.value}></option>{/if}
                  {/each}
                </datalist>{/if}
              {/if}
              <label class="null-control" for={'metadata-null-' + name}>
                <input id={'metadata-null-' + name} type="checkbox" checked={cell.nullValue} disabled={!ready || busy || committedFailure}
                  onchange={event => stage(field, cell.raw, event.currentTarget.checked)} /> Store NULL, not an empty value
              </label>
              {#if cell.error}<p class="failure">{cell.error}</p>{/if}
            </div>
          {/if}
        {/each}
      </div></fieldset>
    {/each}
    <fieldset><legend>Immutable identity and source-managed stamps</legend>
      <dl>{#each ['ID', 'ProjectID', 'AllSpecs', 'TableOfLists', 'DateLastEdited'] as name}
        {@const index = draft.review.projectRecords.columns?.findIndex(column => column.name === name) ?? -1}
        <dt>{name}</dt><dd>{draft.original.cells?.[index]?.storage === 'null' ? 'NULL' : draft.original.cells?.[index] ? metadataCellText(draft.original.cells[index]) : 'Unavailable'}</dd>
      {/each}</dl>
    </fieldset>
  {/if}
  <details><summary>Original master templates (read-only; no automatic conversion)</summary>
    {#each review?.masterTemplates.rows ?? [] as row (row.rowId)}
      <h3>Master physical row {row.rowId}</h3><dl>
        {#each review?.masterTemplates.columns ?? [] as column, index}
          <dt>{column.name}</dt><dd>{row.cells?.[index]?.storage}: {row.cells?.[index] ? metadataCellText(row.cells[index]) || (row.cells[index].storage === 'null' ? 'NULL' : 'Empty text') : 'Unavailable'}</dd>
        {/each}
      </dl>
    {:else}<p>No matching master templates.</p>{/each}
  </details>
</section>

<style>
  .metadata-editor { border: 1px solid #cbd5e1; border-radius: .75rem; background: #fff; padding: 1rem; color: #334155; margin: 1rem 0; }
  h2 { font-size: 1.25rem; font-weight: 600; }
  fieldset { border: 1px solid #e2e8f0; border-radius: .5rem; margin: 1rem 0; padding: 1rem; min-width: 0; }
  legend { font-weight: 600; padding: 0 .4rem; }
  .metadata-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 18rem), 1fr)); gap: 1rem; }
  .metadata-field { display: flex; flex-direction: column; gap: .35rem; min-width: 0; }
  .wide { grid-column: 1 / -1; }
  input:not([type=checkbox]), select, textarea { width: 100%; box-sizing: border-box; min-width: 0; min-height: 2.5rem; padding: .5rem; border: 1px solid #94a3b8; border-radius: .35rem; font: inherit; }
  input:focus, select:focus, textarea:focus { outline: 2px solid #0f766e; outline-offset: 2px; }
  .null-control { font-size: .8rem; }
  .toolbar { display: flex; flex-wrap: wrap; gap: .5rem; margin: 1rem 0; }
  button { padding: .5rem .75rem; border: 1px solid #94a3b8; border-radius: .35rem; margin: .2rem; }
  button:disabled { opacity: .5; cursor: not-allowed; }
  button[aria-pressed=true] { background: #ccfbf1; }
  .failure { color: #b91c1c; }
  .success { color: #047857; }
  .decision { display: block; background: #fef3c7; padding: .75rem; }
  dl { display: grid; grid-template-columns: minmax(8rem, 1fr) 2fr; gap: .5rem; overflow-wrap: anywhere; }
  dt { font-weight: 600; }
  @media (max-width: 600px) { dl { grid-template-columns: 1fr; } }
</style>
