<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import type { bindContextPlots } from './contextPlots';
  import { ReadRequests } from './readRequests';
  import type { EditorCloseState } from './closeLifecycle';
  import { profileEditableFields, profileRuleValue, profileRuleOptions, stageProfileRule, profileRulesDirty,
    profileRuleErrors, profileRuleEditRequest, validateProfileChoices, type ProfileRulesDraft, type ProfileRuleChoices, type ProfileField } from './projectProfileRuleEditor';
  import { profileCellLabel, validateProjectPlotProfileReview, validateProjectProfileLump, validateProfileRunResult,
    type ValidatedProjectPlotProfileReview, type ProfileReviewTable, type ProfileRunResult } from './projectPlotProfileReview';

  let { client, onclosed, onbusy, onblocked, allowRun = false, allowEditing = false }: {
    client: ReturnType<typeof bindContextPlots>; onclosed: () => void; onbusy: (busy: boolean) => void;
    onblocked: (blocked: boolean) => void; allowRun?: boolean; allowEditing?: boolean;
  } = $props();
  let review = $state<ValidatedProjectPlotProfileReview | null>(null);
  let reading = $state(false);
  let running = $state(false);
  let error = $state<string | null>(null);
  let lump = $state<ProfileReviewTable | null>(null);
  let subvarieties = $state(false);
  let result = $state<ProfileRunResult | null>(null);
  let draft = $state<ProfileRulesDraft | null>(null);
  let saving = $state(false);
  let committedFailure = $state(false);
  let success = $state<string | null>(null);
  let choices = $state<ProfileRuleChoices | null>(null);
  const dirty = $derived(draft ? profileRulesDirty(draft) : false);
  const errors = $derived(draft ? profileRuleErrors(draft) : []);
  const blocked = $derived(dirty || committedFailure);
  const busy = $derived(reading || saving);
  let generation = 0;
  const reads = new ReadRequests();
  onMount(() => { void reload(); });
  $effect(() => onblocked(blocked));
  onDestroy(() => { generation++; reads.cancelAll(); onbusy(false); onblocked(false); });

  export function getCloseState(): EditorCloseState {
    return { unsaved: blocked, busy, canSave: false, error,
      saveReason: 'Save or Undo profile rules explicitly before closing or changing context; ordinary plot Save never writes profile rules.' };
  }
  export function undo() {
    if (busy || committedFailure) { error = 'Wait for the operation or Reload committed rules before Undo.'; return; }
    if (draft) draft = { review: draft.review, cells: {} };
    error = null; success = 'Profile drafts undone; no rules, counts or history changed.';
  }
  function stage(rowId: string, name: ProfileField, raw: string, nullValue: boolean) {
    if (!allowEditing || !draft || busy || committedFailure) { error = 'Reload available profile rules before editing.'; return; }
    try { draft = stageProfileRule(draft, rowId, name, raw, nullValue); error = null; success = null; result = null; }
    catch (cause) { error = `Profile draft failed; existing drafts retained: ${String(cause)}`; }
  }
  async function saveRules() {
    if (!allowEditing || !draft || busy || committedFailure || !dirty || errors.length) {
      error = 'Correct or Undo profile errors before saving reviewed changes.'; return;
    }
    let committed = false;
    saving = true; onbusy(true); error = null; success = null;
    try {
      await client.SaveProjectPlotProfile(profileRuleEditRequest(draft));
      committed = true; draft = null; result = null; lump = null; subvarieties = false;
      const next = validateProjectPlotProfileReview(await client.ReviewProjectPlotProfile());
      review = next; draft = { review: next, cells: {} };
      success = 'Profile assignments and typed technical history committed atomically. Historical PlotCount and plot drafts were not changed.';
    } catch (cause) {
      if (committed || String(cause).includes('Profile rule changes committed, but')) {
        committedFailure = true; draft = null; result = null; lump = null; subvarieties = false;
        error = `Profile rules committed, but refresh/cleanup failed. Reload before editing; completed writes must not be replayed: ${String(cause)}`;
      } else error = `Profile save failed; raw drafts retained for correction or retry: ${String(cause)}`;
    } finally { saving = false; onbusy(false); }
  }
  function close() {
    if (busy || blocked) { error = 'Save or Undo profile drafts, or Reload committed rules, before closing this review.'; return; }
    onclosed();
  }

  async function reload() {
    if (busy || dirty) { error = 'Save or Undo profile drafts before reloading; raw errors were retained.'; return; }
    const request = ++generation;
    draft = null; choices = null;
    review = null; error = null; lump = null; result = null; reading = true; onbusy(true);
    try {
      const [source, suggestions] = await Promise.all([
        reads.track(client.ReviewProjectPlotProfile()),
        allowEditing ? reads.track(client.ListProjectPlotProfileChoices()) : Promise.resolve(null),
      ]);
      const result = validateProjectPlotProfileReview(source);
      const validatedChoices = suggestions ? validateProfileChoices(suggestions) : null;
      if (request === generation) {
        review = result; choices = validatedChoices; draft = allowEditing ? { review: result, cells: {} } : null;
        committedFailure = false; success = null;
      }
    } catch (cause) {
      if (request === generation) error = `Project-local profile review failed; no rules, counts or filters changed: ${String(cause)}`;
    } finally {
      if (request === generation) { reading = false; onbusy(false); }
    }
  }
  async function reviewLump() {
    if ((!allowRun && !allowEditing) || busy || !review) return;
    const request = ++generation;
    error = null; lump = null; result = null; reading = true; onbusy(true);
    try {
      const source = validateProjectProfileLump(await reads.track(client.ReviewProjectPlotProfileLump()));
      if (request === generation) lump = source;
    } catch (cause) {
      if (request === generation) error = `Project-local lump review failed; no definitions changed: ${String(cause)}`;
    } finally {
      if (request === generation) { reading = false; onbusy(false); }
    }
  }
  async function run() {
    if (!allowRun || busy || blocked || !review) return;
    const source = review, request = ++generation;
    result = null; error = null; reading = true; running = true; onbusy(true);
    try {
      const preview = await reads.track(client.RunProjectPlotProfile({
        originalRules: source.rules, projectLump: lump, subvarieties
      }));
      if (request === generation) result = validateProfileRunResult(preview, source);
    } catch (cause) {
      if (request === generation) error = `Profile preview failed; no stored rules, counts or filters changed: ${String(cause)}`;
    } finally {
      if (request === generation) { reading = false; running = false; onbusy(false); }
    }
  }
  function cancelRun() {
    generation++; reads.cancelAll();
    running = false; reading = false; result = null; onbusy(false);
    error = 'Profile preview cancelled; reviewed inputs retained. No stored rules, counts or filters changed.';
  }
</script>

<section class="m-3 p-3 border border-stone-300 rounded-lg bg-stone-50" aria-label="Project-local plot profile review">
  <div class="flex flex-wrap gap-2 items-center justify-between">
    <h2 class="font-semibold text-stone-800">Project-local plot profile {allowEditing ? 'rules' : 'review (read-only)'}</h2>
    <div class="flex flex-wrap gap-2">
      <button type="button" class="px-3 py-2 border rounded bg-white disabled:opacity-50" disabled={busy || dirty} onclick={() => void reload()}>Reload profile review</button>
      {#if allowRun}
        <button type="button" class="px-3 py-2 border rounded bg-white disabled:opacity-50" disabled={busy || blocked || !review} onclick={() => void run()}>Run stored profile preview</button>
        {#if running}<button type="button" class="px-3 py-2 border rounded bg-white" onclick={cancelRun}>Cancel profile preview</button>{/if}
      {:else}
        <button type="button" class="px-3 py-2 border rounded bg-white disabled:opacity-50" disabled title="Ordered profile execution is not implemented.">Run Profile (unavailable)</button>
      {/if}
      <button type="button" class="px-3 py-2 border rounded bg-white disabled:opacity-50" disabled={busy || blocked} onclick={close}>Close profile review</button>
    </div>
  </div>
  {#if error}<p role="alert" class="mt-3 text-red-800">{error}</p>{/if}
  {#if success}<p role="status" class="mt-3 text-emerald-800">{success}</p>{/if}
  {#if reading}<p role="status" class="mt-3">Reading original inputs or evaluating stored profile rules...</p>{/if}
  {#if (allowRun || allowEditing) && review}
    <div class="flex flex-wrap gap-3 items-center mt-3">
      <button type="button" class="px-3 py-2 border rounded bg-white disabled:opacity-50" disabled={busy} onclick={() => void reviewLump()}>Review and select project lump table</button>
      {#if lump}
        <span>{review.project}_Lump explicitly selected: {lump.rows.length} physical definitions.</span>
        <button type="button" class="px-3 py-2 border rounded bg-white disabled:opacity-50" disabled={busy || blocked} onclick={() => { lump = null; subvarieties = false; result = null; error = null; }}>Clear lump selection</button>
      {/if}
      {#if allowRun}
        <label class="flex gap-2 items-center"><input type="checkbox" disabled={busy || blocked || !lump} checked={subvarieties}
          onchange={event => { subvarieties = event.currentTarget.checked; result = null; error = null; }} />Combine source subvarieties</label>
      {/if}
    </div>
  {/if}
  {#if allowEditing && draft}
    <section class="mt-3" aria-label="Stored profile rule editor">
      <div class="flex flex-wrap gap-2">
        <button type="button" class="px-3 py-2 border rounded bg-white disabled:opacity-50"
          disabled={busy || committedFailure || !dirty || errors.length > 0} onclick={() => void saveRules()}>Save profile rules</button>
        <button type="button" class="px-3 py-2 border rounded bg-white disabled:opacity-50"
          disabled={busy || committedFailure || !dirty} onclick={undo}>Undo profile drafts</button>
      </div>
      {#each draft.review.rules.rows as row (row.rowId)}
        <fieldset class="mt-3 p-3 border rounded bg-white" disabled={busy || committedFailure}>
          <legend class="font-semibold">Physical rule {row.rowId}</legend>
          <div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-3">
            {#each profileEditableFields as name}
              {@const cell = profileRuleValue(draft, row.rowId, name)}
              {@const id = `profile-${row.rowId}-${name}`}
              <div class="min-w-0">
                <label class="block text-sm font-medium" for={id}>{name}</label>
                <input id={id} class="w-full min-w-0 border rounded px-2 py-1" type="text" value={cell.raw}
                  list={`${id}-choices`} disabled={cell.nullValue} aria-invalid={cell.error !== null}
                  aria-describedby={cell.error ? `${id}-error` : undefined}
                  oninput={event => stage(row.rowId, name, event.currentTarget.value, false)} />
                <datalist id={`${id}-choices`}>{#each profileRuleOptions(draft, row.rowId, name, choices, lump) as option}<option value={option}></option>{/each}</datalist>
                <label class="flex gap-2 items-center text-xs mt-1"><input type="checkbox" checked={cell.nullValue}
                  onchange={event => stage(row.rowId, name, cell.raw, event.currentTarget.checked)} />NULL {name}, rule {row.rowId}</label>
                {#if cell.error}<p id={`${id}-error`} role="alert" class="text-xs text-red-800 mt-1">{cell.error}</p>{/if}
              </div>
            {/each}
          </div>
        </fieldset>
      {/each}
      <p class="mt-3 text-sm text-stone-600">Existing rules only. Table changes to Veg/Lump visibly propose Field=Species/LumpCode; no Layer, Species or Criteria is cleared. NULL differs from empty text. Only changed assignments are saved; stored PlotCount is read-only. Env field suggestions come from this project; Veg species suggestions use grouped VLists codes, and Lump species suggestions require explicit lump review. Suggestions do not restrict those literal fields.</p>
      {#if choices}
        <p class="mt-1 text-xs text-stone-600">Source species definitions: {choices.species.length} distinct typed codes, including {choices.species.filter(cell => cell.storage === 'null').length} NULL and {choices.species.filter(cell => cell.storage === 'text' && cell.text === '').length} empty. NULL is set explicitly, not by an empty suggestion.</p>
      {/if}
    </section>
  {/if}
  {#if result}
    <section class="mt-3 p-3 border border-emerald-300 rounded bg-white" aria-label="Stored profile preview results">
      <h3 class="font-semibold">Preview only: {result.plotNumbers.length} matching plots / {result.totalPlots} stored scope plots</h3>
      <p>Scope: {result.project}; Working Unit list: {result.su}. No form filter or SU table has been changed.</p>
      <div class="overflow-x-auto">
        <table class="w-full text-xs border-collapse"><caption class="text-left py-2">Run-only counts, not stored PlotCount</caption>
          <thead><tr>{#each ['Physical rule', 'Order', 'Operation', 'Step count', 'Remaining'] as label}<th scope="col" class="border p-2 text-left">{label}</th>{/each}</tr></thead>
          <tbody>{#each result.steps as step (step.rowId)}<tr>
            <th scope="row" class="border p-2">{step.rowId}</th><td class="border p-2">{step.order}</td><td class="border p-2">{step.operation}</td>
            <td class="border p-2" aria-label={`Step count, rule ${step.rowId}`}>{step.plotCount}</td>
            <td class="border p-2" aria-label={`Remaining, rule ${step.rowId}`}>{step.remaining}</td>
          </tr>{/each}</tbody>
        </table>
      </div>
      <p class="mt-2 whitespace-pre-wrap break-words" data-profile-plots>{result.plotNumbers.join(', ') || 'No matching plots.'}</p>
    </section>
  {/if}
  {#if review}
    <p class="my-3">Source: {review.project} / {review.table}. Rules follow stored Order; duplicate Orders retain distinct physical records. PlotCount is historical storage, not a recalculated result.</p>
    {#each [{ title: 'Original profile rules', table: review.rules }, { title: 'Original table-object descriptions', table: review.descriptions },
      ...((allowRun || allowEditing) && lump ? [{ title: 'Explicitly selected project-local lump definitions', table: lump }] : [])] as item}
      <div class="overflow-x-auto mt-3">
        <table class="w-full border-collapse text-xs">
          <caption class="text-left font-semibold py-2">{item.title}</caption>
          <thead><tr><th scope="col" class="border p-2 text-left">Physical row</th>
            {#each item.table.columns as column}<th scope="col" class="border p-2 text-left">{column.name}<span class="block font-normal text-stone-500">{column.declaredType}</span></th>{/each}
          </tr></thead>
          <tbody>{#each item.table.rows as row (row.rowId)}
            <tr><th scope="row" class="border p-2 text-left">{row.rowId}</th>
              {#each row.cells as cell, index}<td class="border p-2 whitespace-pre-wrap min-w-24" aria-label={`${item.table.columns[index].name}, record ${row.rowId}`}>{profileCellLabel(cell)}</td>{/each}
            </tr>
          {/each}</tbody>
        </table>
        {#if item.table.rows.length === 0}<p class="py-2">No physical records in this source table.</p>{/if}
      </div>
    {/each}
  {/if}
  <p class="mt-3 text-sm text-stone-600">This uses only stored project-local rules and data, never unsaved plot drafts. Preview runs use isolated SQLite TEMP results and source-scoped Env/Veg/Lump operations; text matching supports ASCII literals and simple * / ? patterns only. Unsupported rules fail explicitly. Other profile databases, rule creation/deletion, applying filters and Save as SU remain unavailable. No stored PlotCount, Access registry preference or shipped scratch table is changed.</p>
</section>
