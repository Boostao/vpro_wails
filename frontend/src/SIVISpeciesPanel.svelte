<script lang="ts">
  import { AuditRestoreAction } from '../bindings/github.com/boostao/vpro-wails';
  import { metadataCellText } from './projectMetadataEditor';
  import { siviSpeciesRows, siviSpeciesListed, siviSpeciesChoices, siviSpeciesDirty, siviSpeciesErrors } from './siviSpeciesEditor';
  import type { SIVISpeciesView } from './siviSpeciesSession';
  import type { SpeciesDecisionKind } from './vegetationSpeciesEditor';
  import { speciesEventError } from './vegetationSpeciesEditor';
  import SIVIChildSourceNotices from './SIVIChildSourceNotices.svelte';

  let { view, disabled, canSave, onstage, oncontext, onchoose, onsave, onundo, onreload, onrestore }: {
    view: SIVISpeciesView; disabled: boolean; canSave: boolean;
    onstage: (rowId: string, raw: string) => void; oncontext: (rowId: string, group: number) => void;
    onchoose: (rowId: string, kind: SpeciesDecisionKind, selected?: string) => void;
    onsave: () => void; onundo: () => void; onreload: () => void; onrestore: (action: AuditRestoreAction) => void;
  } = $props();
  const groups = ['Tree/Shrubs', 'Herb', 'Moss/Lichen'];
  const rows = $derived(view.review?.some(group => group.Rows.length) ? siviSpeciesRows(view.review) : []);
  const errors = $derived(siviSpeciesErrors(view.drafts));
  const dirty = $derived(siviSpeciesDirty(view.drafts));
  const unavailable = $derived(disabled || view.busy || view.blocked || !view.references);
</script>

<section class="space-y-4 rounded border border-stone-200 p-4" data-sivi-species-panel aria-label="SIVI Species source editor">
  <h2 class="font-semibold">SIVI Species</h2>
  {#if view.error}<p class="text-sm text-red-700" role="alert">{view.error}</p>{/if}
  {#if view.blocked}<p class="text-sm text-red-700" role="alert">Acknowledgement or refresh is unresolved. Undo/reload explicitly; never replay Save or restoration.</p>{/if}
  {#each errors as error}<p class="text-sm text-red-700" role="alert">{error}</p>{/each}
  {#if view.busy}<p role="status">Loading or saving owned Species rows and definitions...</p>{/if}
  <div class="flex flex-wrap gap-2">
    <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-species-save
      disabled={unavailable || !dirty || errors.length > 0 || !canSave} onclick={onsave}>Save SIVI Species</button>
    <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-species-undo
      disabled={view.busy || !dirty && !view.blocked} onclick={onundo}>Undo / reload Species</button>
    <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-species-reload
      disabled={disabled || view.busy || view.blocked || dirty} onclick={onreload}>Reload Species rows and definitions</button>
    {#if view.historyId}
      <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-species-restore="retain"
        disabled={unavailable || dirty} onclick={() => onrestore(AuditRestoreAction.AuditRestoreRetain)}>Restore Species, retain audit</button>
      <button type="button" class="rounded border px-3 py-1 disabled:opacity-50" data-sivi-species-restore="prune"
        disabled={unavailable || dirty} onclick={() => onrestore(AuditRestoreAction.AuditRestorePrune)}>Restore Species, prune selected audit</button>
    {/if}
  </div>
  {#if view.review && view.references}
    {#if rows.length === 0}<p class="text-sm text-stone-600">No cover-driven source rows.</p>{/if}
    <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
      {#each rows as { row, contexts } (row.rowId)}
        {@const draft = view.drafts[row.rowId]}
        {@const groupIndex = draft?.group ?? contexts[0].index}
        {@const context = contexts.find(context => context.index === groupIndex)}
        {@const original = row.cells[2]}
        {@const raw = draft?.raw ?? (original.storage === 'text' ? original.text ?? '' : '')}
        {@const choices = draft && !draft.decision && context ? siviSpeciesChoices(view.references, context.group.Form, raw) : null}
        {@const aliases = choices?.aliases.filter(option => option.code !== null) ?? []}
        {@const options = siviSpeciesListed(view.references, groupIndex)}
        {@const id = `sivi-species-${row.rowId}`}
        <article class="min-w-0 space-y-3 rounded border border-stone-200 p-3" data-sivi-species-row={row.rowId}>
          <p class="break-words text-xs text-stone-600">Physical row {row.rowId}; ID {metadataCellText(row.cells[0])}</p>
          <div>
            <label for={`${id}-context`} class="block text-sm font-medium">Source group</label>
            <select id={`${id}-context`} class="w-full min-w-0 rounded border px-2 py-1" value={groupIndex}
              data-sivi-species-context={row.rowId} disabled={unavailable || contexts.length < 2}
              onchange={(event) => oncontext(row.rowId, Number(event.currentTarget.value))}>
              {#each contexts as context}<option value={context.index}>{groups[context.index]}</option>{/each}
            </select>
          </div>
          <div>
            <label for={id} class="block text-sm font-medium">Species</label>
            <input {id} class="w-full min-w-0 rounded border px-2 py-1" value={raw} list={`${id}-list`}
              data-sivi-species-identity={row.rowId} aria-invalid={draft?.error ? 'true' : undefined}
              disabled={unavailable || original.storage !== 'null' && original.storage !== 'text'}
              oninput={(event) => onstage(row.rowId, event.currentTarget.value)} />
            <datalist id={`${id}-list`}>
              {#each options as option}
                <option value={option.code ?? ''}>{option.scientificName ?? 'NULL'}; {option.englishName ?? 'NULL'}</option>
              {/each}
            </datalist>
            <p class="break-words text-xs text-stone-600">Stored Species: {original.storage === 'null' ? 'NULL' : metadataCellText(original)}</p>
            {#if original.storage !== 'null' && original.storage !== 'text'}
              <p class="text-sm text-amber-800">Historical non-text Species is read-only; no value is coerced.</p>
            {/if}
          </div>
          {#if choices}
            {#if aliases.length}
              <fieldset class="space-y-2 rounded border p-2">
                <legend class="text-sm font-medium">Old-code decision for {raw}</legend>
                <button type="button" class="rounded border px-2 py-1" disabled={unavailable || speciesEventError(raw) !== null}
                  data-sivi-species-keep={row.rowId} onclick={() => onchoose(row.rowId, 'keep')}>Keep entered code (source uppercase)</button>
                {#each aliases as option}
                  <button type="button" class="max-w-full break-words rounded border px-2 py-1"
                    disabled={unavailable || speciesEventError(raw) !== null || speciesEventError(option.code) !== null}
                    data-sivi-species-replace={row.rowId} onclick={() => onchoose(row.rowId, 'replace', option.code ?? undefined)}>
                    Replace with {option.code} ({option.scientificName ?? 'NULL'}; {option.englishName ?? 'NULL'})
                  </button>
                {/each}
              </fieldset>
            {:else if choices.users.some(option => option.code !== null)}
              <fieldset class="space-y-2 rounded border p-2">
                <legend class="text-sm font-medium">Personal-code decision for {raw}</legend>
                {#each choices.users.filter(option => option.code !== null) as option}
                  <button type="button" class="max-w-full break-words rounded border px-2 py-1"
                    disabled={unavailable || speciesEventError(raw) !== null || speciesEventError(option.code) !== null}
                    data-sivi-species-user={row.rowId} onclick={() => onchoose(row.rowId, 'user', option.code ?? undefined)}>
                    Use personal {option.code} ({option.scientificName ?? 'NULL'}; {option.englishName ?? 'NULL'})
                  </button>
                {/each}
              </fieldset>
            {:else if draft.error}
              <p class="text-sm text-amber-800">No old-code or existing personal definition resolves this entry. Personal species creation is unavailable.</p>
            {/if}
          {/if}
          {#if draft?.decision}
            <p class="break-words text-sm">Reviewed {draft.decision.kind} decision for {draft.decision.entered}. Source focus target:
              {groupIndex === 0 ? 'Cover1' : groupIndex === 1 ? 'Cover6' : 'Cover7'} (read-only in this editor).</p>
          {/if}
        </article>
      {/each}
    </div>
  {:else if !view.busy}
    <p class="text-sm text-stone-600">Load all three source groups and their owned master/personal definitions before editing.</p>
  {/if}
  <SIVIChildSourceNotices notices={view.sourceNotices ?? []} saved={view.sourceNoticesSaved ?? false} />
  <p class="text-xs text-stone-600">One live Species field owns each physical row. Source groups have different lists; changing group retains your entry.
    Exact listed codes are not trimmed or recased. Explicit old-code/personal decisions use source uppercase only for verified ASCII.
    Covers, heights, Collected, IDs, metadata and creation/deletion are not editable here.</p>
</section>
