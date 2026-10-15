<script lang="ts">
  import { untrack } from 'svelte';
  import { twoPageVegetationSession, twoPageVegetationWritingEnabled, twoPageVegetationSpeciesListEnabled } from './twoPageVegetationSessions';
  import { vegetationColumnIndex, vegetationSourceCell, type TwoPageVegetationForm } from './twoPageVegetationWire';
  import { metadataCellText } from './projectMetadataEditor';
  import { collectedNext } from './collectedEditor';
  import { twoPageVegetationGroups, twoPageVegetationStoredText, twoPageVegetationRowDiagnostic, twoPageVegetationHasColumn } from './twoPageVegetationPresentation';
  import VegetationSpeciesReferences from './VegetationSpeciesReferences.svelte';
  import { vegetationSpeciesListOptions, vegetationSpeciesListHasCommonCover } from './twoPageVegetationSpeciesList';

  let { contextId, project, plot, form, parentBlocked }: {
    contextId: string; project: string; plot: string; form: TwoPageVegetationForm; parentBlocked: boolean;
  } = $props();
  const session = $derived(twoPageVegetationSession({ contextId, project, plot, form }));
  let revision = $state(0);
  let error = $state<string | null>(null);
  let panel = $state<HTMLElement>();
  $effect(() => session.subscribe(() => untrack(() => revision++)));
  const view = $derived.by(() => { revision; return session.view(); });
  const selected = $derived(view.selectedRowId);
  const close = $derived.by(() => { revision; return session.closeState(); });
  const row = $derived(selected === null
    ? view.original?.records.rows.find(row => row.rowId === view.original?.sourceRowIds[0]) ?? view.original?.records.rows[0]
    : view.original?.records.rows.find(row => row.rowId === selected));
  const diagnostic = $derived(view.original && row ? twoPageVegetationRowDiagnostic(view.original, row.rowId) : null);
  const fieldsDisabled = $derived(parentBlocked || !twoPageVegetationWritingEnabled || view.busy ||
    view.blocked || view.acknowledgementRequired || !!diagnostic);
  const speciesListAvailable = $derived(twoPageVegetationSpeciesListEnabled && (form === 'SubVegAXL_BC' || form === 'SubVegAXL'));
  const speciesOptions = $derived(speciesListAvailable && view.references
    ? vegetationSpeciesListOptions(view.references, { contextId, project, plot, form }) : []);
  const speciesWarningUnavailable = $derived(speciesListAvailable && view.original && row
    ? !vegetationSpeciesListHasCommonCover(view.original, row.rowId) : false);

  function pickSpecies(plannedRow: string, event: Event) {
    try {
      const select = event.currentTarget;
      if (!speciesListAvailable || speciesWarningUnavailable || fieldsDisabled || !row || row.rowId !== plannedRow || !panel ||
        panel.dataset.vegetationCurrentRow !== plannedRow || !(select instanceof HTMLSelectElement) ||
        select.disabled || !select.checkVisibility() || !panel.contains(select) ||
        !speciesOptions.some(option => option.code === select.value)) {
        throw new Error('Observed Species list control, physical row or exact code differs from the planned selection.');
      }
      stage('Species', select.value, false);
    } catch (cause) { error = `Species list selection refused without mutation: ${String(cause)}`; }
  }

  function selectRow(rowId: string) {
    try {
      if (!view.original?.records.rows.some(row => row.rowId === rowId)) throw new Error('Observed physical vegetation row is unavailable.');
      session.select(rowId); error = null;
    } catch (cause) { error = `Vegetation selection refused without mutation: ${String(cause)}`; }
  }
  function stage(column: string, raw: string, nullValue: boolean) {
    try {
      if (fieldsDisabled || !row) throw new Error('Resolve parent/other-owner operations and unlock the plot before linked vegetation staging.');
      session.stage(row.rowId, column, raw, nullValue); error = null;
    } catch (cause) { error = `Linked vegetation draft refused; owned state retained: ${String(cause)}`; }
  }
  function cycleCollected(plannedRow: string, event: MouseEvent) {
    try {
      if (fieldsDisabled || !row || row.rowId !== plannedRow || !view.original || !panel ||
        panel.dataset.twoPageVegetationPanel !== form || panel.dataset.vegetationCurrentRow !== plannedRow) {
        throw new Error('Observed vegetation owner/physical row differs from the planned Collected click.');
      }
      const button = event.currentTarget;
      const input = panel.querySelector<HTMLInputElement>('[data-vegetation-column="Collected"] input[type="text"]');
      if (!(button instanceof HTMLButtonElement) || button.disabled || !panel.contains(button) ||
        !button.checkVisibility() || !input || input.disabled || !input.checkVisibility()) {
        throw new Error('Observed Collected controls are unavailable; no source value was cycled.');
      }
      const draft = view.drafts[JSON.stringify([plannedRow, 'Collected'])];
      const source = draft?.value ?? vegetationSourceCell(view.original, plannedRow, 'Collected');
      if (draft?.error || source.storage !== 'null' && source.storage !== 'text') {
        throw new Error('Correct Collected validation/nontext storage manually before its source click.');
      }
      const raw = draft?.raw ?? metadataCellText(source);
      if (input.value !== (source.storage === 'null' ? '' : raw.replace(/[\r\n]/g, ''))) {
        throw new Error('Observed Collected display differs from its retained draft/original.');
      }
      const next = collectedNext(source.storage === 'null' ? null : source.text);
      stage('Collected', next ?? '', next === null);
    } catch (cause) { error = `Source Collected click refused without mutation: ${String(cause)}`; }
  }
  async function operation(action: 'load' | 'references' | 'save' | 'recover' | 'undo' | 'acknowledge' | 'cancel') {
    const owned = session;
    try {
      if (parentBlocked && (action === 'load' || action === 'references' || action === 'save')) throw new Error('Finish actual parent/other-owner drafts and unlock the plot before loading or saving linked vegetation.');
      error = null;
      if (action === 'undo') session.undo();
      else if (action === 'acknowledge') session.acknowledge();
      else if (action === 'cancel') session.cancel();
      else {
        const ok = action === 'load' ? await owned.load()
          : action === 'references' ? await owned.loadReferences()
          : action === 'save' ? await owned.save(crypto.randomUUID()) : await owned.resolve();
        if (owned !== session) return;
        if (!ok) error = action === 'references' ? owned.view().referenceError : owned.view().error;
      }
    } catch (cause) {
      if (owned === session) error = `Linked vegetation operation refused; owned state retained: ${String(cause)}`;
    }
  }
</script>

<section bind:this={panel} class="min-w-0 space-y-3 rounded border bg-white p-3" aria-label={`${form} linked vegetation`}
  data-two-page-vegetation-panel={form} data-vegetation-current-row={row?.rowId}>
  <h4 class="font-semibold">{form === 'SubVegCXL' ? 'Herb' : form === 'SubVegDXL' ? 'Moss/Lichen' : 'Tree/Shrubs'}</h4>
  {#if parentBlocked}<p role="status">Finish actual parent/other-owner drafts and unlock the plot before new vegetation staging.</p>{/if}
  {#if error}<p role="alert" class="break-words text-sm text-red-700">{error}</p>{/if}
  {#if view.error && view.error !== error}<p role="alert" class="break-words text-sm text-red-700">{view.error}</p>{/if}
  {#if view.referenceError && view.referenceError !== error}<p role="alert" class="break-words text-sm text-red-700">{view.referenceError}</p>{/if}
  {#if view.blocked}<p role="alert">Save outcome unresolved. Recover the exact retained request; do not repeat Save.</p>{/if}
  {#if view.noCommitCertificate}<p role="status" class="break-words">Server certified no commit for {view.noCommitCertificate.requestId}. Correct or Undo retained drafts; another Save needs a new request.</p>{/if}
  {#if Object.keys(view.drafts).length}<p role="status">{Object.keys(view.drafts).length} retained field draft(s). Save applies all retained edits together, including other physical rows.</p>{/if}
  <div class="flex flex-wrap gap-2">
    <button type="button" disabled={parentBlocked || close.unsaved || view.busy || close.blocked} onclick={() => operation('load')}>Load vegetation</button>
    <button type="button" disabled={parentBlocked || view.busy || view.blocked || view.acknowledgementRequired}
      onclick={() => operation('references')}>Review Species definitions (read only)</button>
    <button type="button" disabled={parentBlocked || !close.canSave} onclick={() => operation('save')}>Save vegetation</button>
    <button type="button" disabled={view.busy || view.blocked || view.acknowledgementRequired || !Object.keys(view.drafts).length} onclick={() => operation('undo')}>Undo vegetation drafts</button>
    {#if view.busy}<button type="button" onclick={() => operation('cancel')}>Cancel vegetation operation</button>{/if}
    {#if view.blocked}<button type="button" disabled={view.busy} onclick={() => operation('recover')}>Recover vegetation outcome (read only)</button>{/if}
    {#if view.acknowledgementRequired}<button type="button" disabled={view.busy} onclick={() => operation('acknowledge')}>Acknowledge committed vegetation receipt</button>{/if}
  </div>
  {#if view.original}
    <div class="flex flex-wrap gap-2" aria-label={`${form} complete physical rows`}>
      {#each view.original.records.rows as physical (physical.rowId)}
        <button type="button" class="min-w-0 break-words rounded border p-2 text-left" aria-pressed={row?.rowId === physical.rowId}
          disabled={view.busy} onclick={() => selectRow(physical.rowId)}>
          Species {twoPageVegetationHasColumn(view.original, 'Species') ? twoPageVegetationStoredText(view.original, physical.rowId, 'Species') : '(source column unavailable)'};
          ID {twoPageVegetationStoredText(view.original, physical.rowId, 'ID')}; physical row {physical.rowId};
          {view.original.sourceRowIds.includes(physical.rowId) ? 'in source query' : 'physical only, read only'}
        </button>
      {:else}<p role="status">No physical vegetation records for this exact parent. Creation is unavailable.</p>{/each}
    </div>
    {#if selected !== null && !row}<p role="alert">Selected physical row {selected} is no longer observed. Choose an existing physical row explicitly.</p>{/if}
    {#if row}
      <p class="break-words text-sm">Application ID {twoPageVegetationStoredText(view.original, row.rowId, 'ID')}; physical row {row.rowId}; parent {plot}</p>
      {#if diagnostic}<p role="alert">{diagnostic}</p>{/if}
      <p class="break-words text-sm">Species (read only): {twoPageVegetationHasColumn(view.original, 'Species') ? twoPageVegetationStoredText(view.original, row.rowId, 'Species') : '(source column unavailable)'}</p>
      {#if speciesListAvailable && twoPageVegetationHasColumn(view.original, 'Species')}
        {@const speciesDraft = view.drafts[JSON.stringify([row.rowId, 'Species'])]}
        {@const speciesId = `${form}-${row.rowId}-Species-list`}
        {@const plannedRow = row.rowId}
        <div class="min-w-0 space-y-1" data-vegetation-column="Species">
          <label class="block text-sm font-medium" for={speciesId}>Species: existing list selection</label>
          <select id={speciesId} class="w-full min-w-0 rounded border p-2" value={speciesDraft?.raw ?? ''}
            disabled={fieldsDisabled || speciesWarningUnavailable || !view.references || !speciesOptions.length} aria-invalid={!!speciesDraft?.error}
            aria-describedby={speciesDraft?.error ? `${speciesId}-error` : undefined} onchange={event => pickSpecies(plannedRow, event)}>
            <option value="">Select an exact existing list code</option>
            {#each speciesOptions as option (`${option.role}:${option.rowId}`)}
              <option value={option.code}>{option.code} | {option.cells[1].storage === 'null' ? 'NULL' : JSON.stringify(option.cells[1].text)} |
                {option.cells[4].text} | {option.cells[2].integer} |
                {option.cells[3].storage === 'null' ? 'NULL' : JSON.stringify(option.cells[3].text)} | {option.role} row {option.rowId}</option>
            {/each}
          </select>
          {#if speciesDraft?.error}<p id={`${speciesId}-error`} role="alert" class="text-sm text-red-700">{speciesDraft.error}</p>{/if}
          {#if speciesWarningUnavailable}<p role="status" class="text-sm">Species selection is unavailable: all seven original common covers are NULL; the source warning/advance route is not qualified.</p>{/if}
          {#if !view.references}<p role="status" class="text-sm">Review physical Species definitions before selecting a list code.</p>
          {:else if !speciesOptions.length}<p role="status" class="text-sm">No qualified exact A/CHARS list codes are available.</p>{/if}
        </div>
      {/if}
      {#each twoPageVegetationGroups(form) as group}
        <fieldset class="min-w-0 rounded border p-3">
          <legend class="text-sm font-semibold">{group.label}</legend>
          <div class="grid min-w-0 grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
            {#each group.fields as [column, label]}
              {#if twoPageVegetationHasColumn(view.original, column)}
                {@const source = row.cells[vegetationColumnIndex(view.original, column)]}
                {@const draft = view.drafts[JSON.stringify([row.rowId, column])]}
                {@const raw = draft?.raw ?? (source.storage === 'null' ? '' : metadataCellText(source))}
                {@const nullValue = draft?.nullValue ?? source.storage === 'null'}
                {@const id = `${form}-${row.rowId}-${column}`}
                {@const plannedRow = row.rowId}
                <div class="min-w-0 space-y-1 focus-within:bg-amber-50" data-vegetation-column={column}>
                  <label class="block text-sm font-medium" for={id}>{label}</label>
                  <input {id} class="w-full min-w-0 rounded border p-2" type="text" value={nullValue ? '' : raw}
                    disabled={fieldsDisabled} placeholder={nullValue ? 'NULL' : undefined}
                    aria-invalid={!!draft?.error} aria-describedby={draft?.error ? `${id}-error` : undefined}
                    oninput={event => stage(column, event.currentTarget.value, false)} />
                  <label class="flex items-center gap-2 text-sm">
                    <input type="checkbox" checked={nullValue} disabled={fieldsDisabled} onchange={event => stage(column, raw, event.currentTarget.checked)} />
                    {label}: NULL (not empty text)
                  </label>
                  {#if draft?.error}<p id={`${id}-error`} role="alert" class="text-sm text-red-700">{draft.error}</p>{/if}
                  {#if !draft}<p class="text-xs text-gray-600">Stored {source.storage}</p>{/if}
                  {#if column === 'Collected'}
                    <button type="button" disabled={fieldsDisabled || !!draft?.error ||
                      (draft?.value ?? source).storage !== 'null' && (draft?.value ?? source).storage !== 'text'}
                      onclick={event => cycleCollected(plannedRow, event)}>Cycle Collected: NULL / C / V</button>
                    <p class="text-xs text-gray-600">C = Collected; V = Verified. Manual values remain distinct.</p>
                  {/if}
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
  {#if view.receipts.length}<p role="status">{view.receipts.length} immutable committed vegetation receipt(s) retained across remounts.</p>{/if}
  {#if view.references}<VegetationSpeciesReferences references={view.references} />{/if}
  <p class="text-xs text-gray-600">Experimental existing-row cover/Collected review and physical Species metadata.
    {#if speciesListAvailable}Species selection is limited to exact existing A/CHARS list codes; common-seven-cover-empty warning/advance remains unavailable.{/if}
    New numeric drafts use calibrated SINGLE conversion before validation; unchanged historical values are not silently repaired.
    Physical reader order is a desktop adaptation, not source Species sorting. Manual/alias/NotInList/personal creation decisions,
    automatic Current/Enter navigation, creation, deletion and restoration remain unavailable.
    Save is explicit, never triggered by changing rows, selecting Species or cycling Collected. NULL is not empty text.</p>
</section>
