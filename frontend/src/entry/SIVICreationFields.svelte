<script lang="ts">
  import { siviCoverGroups, siviCoverLabels, type SIVICoverColumn } from './siviCoverEditor';
  import { siviSpeciesListed, siviSpeciesChoices, type SIVISpeciesOwner, type SIVISpeciesReferences } from './siviSpeciesEditor';
  import { speciesEventError, type SpeciesDecisionKind } from './vegetationSpeciesEditor';
  import { siviCreationErrors, stageSIVICreationCover, stageSIVICreationSpecies, chooseSIVICreationSpecies,
    type SIVICreationDraft } from './siviCreationEditor';

  let { draft, owner, references, disabled = false, onchange }: {
    draft: SIVICreationDraft; owner: SIVISpeciesOwner; references: SIVISpeciesReferences;
    disabled?: boolean; onchange: (draft: SIVICreationDraft) => void;
  } = $props();
  const group = $derived(draft.form === 'SubVegC-SIVI' ? 1 : draft.form === 'SubVegD-SIVI' ? 2 : 0);
  const errors = $derived(siviCreationErrors(draft, references, owner));
  const options = $derived(siviSpeciesListed(references, group));
  const choices = $derived(draft.species && !draft.decision ? siviSpeciesChoices(references, draft.form, draft.species) : null);
  let error = $state('');
  function species(raw: string) {
    try { onchange(stageSIVICreationSpecies(draft, raw)); error = ''; }
    catch (cause) { error = String(cause); }
  }
  function cover(column: SIVICoverColumn, raw: string, nullValue: boolean) {
    try { onchange(stageSIVICreationCover(draft, column, raw, nullValue)); error = ''; }
    catch (cause) { error = String(cause); }
  }
  function choose(kind: SpeciesDecisionKind, selected?: string) {
    try { onchange(chooseSIVICreationSpecies(draft, references, owner, kind, selected)); error = ''; }
    catch (cause) { error = String(cause); }
  }
</script>

<section class="space-y-3 rounded border border-stone-200 p-3" data-sivi-creation-fields aria-label="New SIVI source row fields">
  <h3 class="font-semibold">New {draft.form} row</h3>
  {#if error}<p role="alert" class="text-sm text-red-700">{error}</p>{/if}
  {#each errors as message}<p role="alert" class="text-sm text-red-700">{message}</p>{/each}
  <p class="break-words text-xs text-stone-600">Exact parent {JSON.stringify(draft.plot)}. No application or physical identity has been assigned.</p>
  <div>
    <label for={`sivi-creation-${draft.form}-species`} class="mb-1 block text-sm font-medium">Species</label>
    <input id={`sivi-creation-${draft.form}-species`} list={`sivi-creation-${draft.form}-species-list`}
      class="w-full min-w-0 rounded border px-3 py-2" value={draft.species} {disabled}
      oninput={event => species(event.currentTarget.value)} />
    <datalist id={`sivi-creation-${draft.form}-species-list`}>
      {#each options as option}
        <option value={option.code ?? ''}>{option.scientificName ?? 'NULL'}; {option.englishName ?? 'NULL'}</option>
      {/each}
    </datalist>
  </div>
  {#if choices?.aliases.some(option => option.code !== null)}
    <fieldset class="space-y-2 rounded border p-2">
      <legend class="text-sm font-medium">Old-code decision for {draft.species}</legend>
      <button type="button" class="rounded border px-2 py-1" disabled={disabled || speciesEventError(draft.species) !== null}
        onclick={() => choose('keep')}>Keep entered code (source uppercase)</button>
      {#each choices.aliases.filter(option => option.code !== null) as option}
        <button type="button" class="max-w-full break-words rounded border px-2 py-1"
          disabled={disabled || speciesEventError(draft.species) !== null || speciesEventError(option.code) !== null}
          onclick={() => choose('replace', option.code ?? undefined)}>
          Replace with {option.code} ({option.scientificName ?? 'NULL'}; {option.englishName ?? 'NULL'})
        </button>
      {/each}
    </fieldset>
  {:else if choices?.users.some(option => option.code !== null)}
    <fieldset class="space-y-2 rounded border p-2">
      <legend class="text-sm font-medium">Personal-code decision for {draft.species}</legend>
      {#each choices.users.filter(option => option.code !== null) as option}
        <button type="button" class="max-w-full break-words rounded border px-2 py-1"
          disabled={disabled || speciesEventError(draft.species) !== null || speciesEventError(option.code) !== null}
          onclick={() => choose('user', option.code ?? undefined)}>
          Use personal {option.code} ({option.scientificName ?? 'NULL'}; {option.englishName ?? 'NULL'})
        </button>
      {/each}
    </fieldset>
  {/if}
  {#if draft.decision}<p class="text-sm">Reviewed {draft.decision.kind} decision for {draft.decision.entered}.</p>{/if}
  {#each siviCoverGroups(group, draft.form === 'SubVegA-SIVI') as columns}
    <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
      {#each columns as column}
        {@const cell = draft.cells[column]}
        {@const id = `sivi-creation-${draft.form}-${column}`}
        <div class="min-w-0">
          <label for={id} class="mb-1 block text-sm font-medium">{siviCoverLabels[column]}{column.startsWith('Total') ? ' total' : ' cover'}</label>
          <input {id} class="w-full min-w-0 rounded border px-3 py-2" value={cell?.raw ?? ''}
            aria-invalid={Boolean(cell?.error)} disabled={disabled || (cell?.nullValue ?? true)}
            oninput={event => cover(column, event.currentTarget.value, false)} />
          <label class="mt-1 flex items-center gap-2 text-sm">
            <input type="checkbox" checked={cell?.nullValue ?? true} {disabled}
              onchange={event => cover(column, cell?.raw ?? '', event.currentTarget.checked)} />NULL {siviCoverLabels[column]}
          </label>
        </div>
      {/each}
    </div>
  {/each}
  <p class="text-xs text-stone-600">Enter at least one explicit non-NULL source cover/total; zero is valid. Omitted values remain NULL.
    No heights, Layer or totals are inferred. Literal numeric bounds and exact Species decisions match the existing source editors.
    Identity is assigned only by a separately verified transactional creation operation; these fields do not write data.</p>
</section>
