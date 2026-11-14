<script lang="ts">
  import { X } from '@lucide/svelte';
  import type { SpeciesItem, VegetationDeletionReview } from '../../bindings/github.com/boostao/vpro-wails';
  import NewChild from './NewChild.svelte';
  import PersonalSpeciesFields from './PersonalSpeciesFields.svelte';
  import { controlLabel } from './formPresentation';
  import type { HeightDrafts } from './heightEditor';
  import { paperChild } from './paperLayout';
  import type { PersonalSpeciesDraft } from './personalSpeciesEditor';
  import type { VegetationCreationDraft } from './vegetationCreationEditor';
  import {
    speciesEventError,
    type SpeciesChoices,
    type SpeciesDecisionKind,
    type SpeciesDrafts,
    type SpeciesLists,
  } from './vegetationSpeciesEditor';

  let {
    region = 'toolbars',
    busy = false,
    headerWorkflowBusy = false,
    error = null,
    plotNumber = '',
    savedPersonalCodes = [],
    personalSpeciesEnabled = true,
    speciesUnsaved = false,
    speciesEditingDisabled = false,
    speciesInvalid = [],
    speciesNotices = [],
    speciesDecisionBusy = false,
    speciesDrafts = {},
    speciesChoices = {},
    speciesLists = {},
    speciesReferenceReady = false,
    speciesReferenceBusy = false,
    personalDraft = null,
    personalInvalid = [],
    collectedUnsaved = false,
    collectedEditingDisabled = false,
    collectedNotices = [],
    attributeUnsaved = false,
    attributeEditingDisabled = false,
    attributeInvalid = [],
    soilUnsaved = false,
    soilEditingDisabled = false,
    soilInvalid = [],
    otherUnsaved = false,
    otherEditingDisabled = false,
    otherInvalid = [],
    creationDraft = null,
    creationInvalid = [],
    creationSpeciesInvalid = null,
    creationNotice = null,
    creationChoices = null,
    deletionReview = null,
    heightUnsaved = false,
    heightEditingDisabled = false,
    heightInvalid = [],
    heightNotices = [],
    heightDrafts = {},
    newChild = null,
    showSpeciesModal = $bindable(false),
    speciesSearchQuery = $bindable(''),
    speciesSearchResults = [],
    searchingSpecies = false,
    saveSpeciesDrafts = async () => {},
    cancelSpeciesDrafts = async () => {},
    reviewSpeciesChoices = async () => {},
    chooseSpeciesCode = () => {},
    startPersonalSpecies = () => {},
    onPersonalDraftChange = () => {},
    savePersonalSpecies = async () => {},
    cancelPersonalSpecies = () => {},
    saveCollectedDrafts = async () => {},
    cancelCollectedDrafts = async () => {},
    saveAttributeDrafts = async () => {},
    cancelAttributeDrafts = async () => {},
    saveSoilDrafts = async () => {},
    cancelSoilDrafts = async () => {},
    saveOtherDrafts = async () => {},
    cancelOtherDrafts = async () => {},
    stageCreationSpecies = () => {},
    reviewCreationSpeciesChoices = async () => {},
    chooseCreationSpecies = () => {},
    startCreationPersonalSpecies = () => {},
    stageCreationCell = () => {},
    saveVegetationCreation = async () => {},
    cancelVegetationCreation = async () => {},
    confirmVegetationDeletion = async () => {},
    cancelVegetationDeletion = async () => {},
    saveHeightDrafts = async () => {},
    cancelHeightDrafts = async () => {},
    createChild = async () => {},
    cancelNewChild = () => {},
    searchSpecies = async () => {},
    selectSpeciesToAdd = async () => {},
  }: {
    region?: 'toolbars' | 'speciesModal' | 'newChild';
    busy?: boolean;
    headerWorkflowBusy?: boolean;
    error?: string | null;
    plotNumber?: string;
    savedPersonalCodes?: string[];
    personalSpeciesEnabled?: boolean;
    speciesUnsaved?: boolean;
    speciesEditingDisabled?: boolean;
    speciesInvalid?: string[];
    speciesNotices?: string[];
    speciesDecisionBusy?: boolean;
    speciesDrafts?: SpeciesDrafts;
    speciesChoices?: Record<string, SpeciesChoices>;
    speciesLists?: SpeciesLists;
    speciesReferenceReady?: boolean;
    speciesReferenceBusy?: boolean;
    personalDraft?: PersonalSpeciesDraft | null;
    personalInvalid?: string[];
    collectedUnsaved?: boolean;
    collectedEditingDisabled?: boolean;
    collectedNotices?: string[];
    attributeUnsaved?: boolean;
    attributeEditingDisabled?: boolean;
    attributeInvalid?: string[];
    soilUnsaved?: boolean;
    soilEditingDisabled?: boolean;
    soilInvalid?: string[];
    otherUnsaved?: boolean;
    otherEditingDisabled?: boolean;
    otherInvalid?: string[];
    creationDraft?: VegetationCreationDraft | null;
    creationInvalid?: string[];
    creationSpeciesInvalid?: string | null;
    creationNotice?: string | null;
    creationChoices?: SpeciesChoices | null;
    deletionReview?: VegetationDeletionReview | null;
    heightUnsaved?: boolean;
    heightEditingDisabled?: boolean;
    heightInvalid?: string[];
    heightNotices?: string[];
    heightDrafts?: HeightDrafts;
    newChild?: 'humus' | 'mineral' | 'other' | null;
    showSpeciesModal?: boolean;
    speciesSearchQuery?: string;
    speciesSearchResults?: SpeciesItem[];
    searchingSpecies?: boolean;
    saveSpeciesDrafts?: () => Promise<void>;
    cancelSpeciesDrafts?: () => Promise<void>;
    reviewSpeciesChoices?: (id: number) => Promise<void>;
    chooseSpeciesCode?: (id: number, kind: SpeciesDecisionKind, selected?: string) => void;
    startPersonalSpecies?: (id: number) => void;
    onPersonalDraftChange?: (draft: PersonalSpeciesDraft) => void;
    savePersonalSpecies?: () => Promise<void>;
    cancelPersonalSpecies?: () => void;
    saveCollectedDrafts?: () => Promise<void>;
    cancelCollectedDrafts?: () => Promise<void>;
    saveAttributeDrafts?: () => Promise<void>;
    cancelAttributeDrafts?: () => Promise<void>;
    saveSoilDrafts?: () => Promise<void>;
    cancelSoilDrafts?: () => Promise<void>;
    saveOtherDrafts?: () => Promise<void>;
    cancelOtherDrafts?: () => Promise<void>;
    stageCreationSpecies?: (raw: string) => void;
    reviewCreationSpeciesChoices?: () => Promise<void>;
    chooseCreationSpecies?: (kind: SpeciesDecisionKind, selected?: string) => void;
    startCreationPersonalSpecies?: () => void;
    stageCreationCell?: (column: string, raw: string) => void;
    saveVegetationCreation?: () => Promise<void>;
    cancelVegetationCreation?: () => Promise<void>;
    confirmVegetationDeletion?: () => Promise<void>;
    cancelVegetationDeletion?: () => Promise<void>;
    saveHeightDrafts?: () => Promise<void>;
    cancelHeightDrafts?: () => Promise<void>;
    createChild?: (first: string, second: string) => Promise<void>;
    cancelNewChild?: () => void;
    searchSpecies?: () => Promise<void>;
    selectSpeciesToAdd?: (sp: SpeciesItem) => Promise<void>;
  } = $props();
</script>

{#if region === 'toolbars'}
  {#if speciesUnsaved}
    <div class="species-draft-toolbar mb-3 p-2 border border-amber-300 bg-amber-50 text-xs" aria-label="Species draft controls">
      {#if savedPersonalCodes.length}<p role="status">Saved personal definitions: {savedPersonalCodes.join(', ')}. Plot Undo does not remove these user-database records.</p>{/if}
      <span>Species drafts are unsubmitted. Switching tabs or cover/height views never saves them; other editing waits.</span>
      {#each speciesNotices as message}<p class="mt-2 font-medium" role="status">{message}</p>{/each}
      <div class="flex gap-2 mt-2">
        <button type="button" class="px-2 py-1 border rounded bg-emerald-700 text-white disabled:opacity-50" disabled={speciesEditingDisabled || speciesInvalid.length > 0} onclick={() => void saveSpeciesDrafts()}>Save species drafts</button>
        <button type="button" class="px-2 py-1 border border-stone-300 rounded bg-white disabled:opacity-50" disabled={busy || headerWorkflowBusy || personalDraft !== null} onclick={() => void cancelSpeciesDrafts()}>Cancel species drafts</button>
      </div>
      {#each speciesInvalid as message}<p role="alert">{message}</p>{/each}
      {#if speciesDecisionBusy}<p role="status">Looking up source species choices...</p>{/if}
      {#each Object.entries(speciesDrafts) as [id, cell] (id)}
        {#if cell.error}
          {@const choices = speciesChoices[id]}
          <section class="mt-2 border-t border-amber-300 pt-2" aria-label={`Species choices, row ${id}`} data-species-choice-row={id}>
            <p>Row {id}: entered code "{cell.raw}" ({cell.form}). No assignment is saved by reviewing choices.</p>
            <button type="button" class="mt-1 px-2 py-1 border rounded bg-white" disabled={speciesEditingDisabled}
              onclick={() => void reviewSpeciesChoices(Number(id))}>Review species code, row {id}</button>
            {#if choices && choices.entered === cell.raw && choices.form === cell.form}
              {@const hasAlias = choices.aliases.some(option => option.code !== null)}
              {#each choices.aliases as option, index (index)}
                <div class="mt-1">
                  <span>{option.code ?? 'NULL'} | {option.scientificName ?? 'NULL'} | {option.englishName ?? 'NULL'} | Lifeform {option.lifeform ?? 'NULL'} | {option.codeType ?? 'NULL'}</span>
                  <button type="button" class="ml-2 px-2 py-1 border rounded bg-white"
                    disabled={speciesEditingDisabled || speciesEventError(cell.raw) !== null || speciesEventError(option.code) !== null}
                    title={speciesEventError(cell.raw) ?? speciesEventError(option.code) ?? 'Stage the explicit replacement; Save remains separate'}
                    onclick={() => option.code !== null && chooseSpeciesCode(Number(id), 'replace', option.code)}>Use replacement {option.code ?? 'NULL'}, row {id}</button>
                </div>
              {/each}
              {#if hasAlias}
                <button type="button" class="mt-1 px-2 py-1 border rounded bg-white"
                  disabled={speciesEditingDisabled || speciesEventError(cell.raw) !== null}
                  title={speciesEventError(cell.raw) ?? 'Stage the entered code using the source UCase event'}
                  onclick={() => chooseSpeciesCode(Number(id), 'keep')}>Keep entered code, row {id}</button>
              {:else}
                {#each choices.users as option, index (index)}
                  <div class="mt-1">
                    <span>{option.code ?? 'NULL'} | {option.scientificName ?? 'NULL'} | {option.englishName ?? 'NULL'} | Lifeform {option.lifeform ?? 'NULL'} | {option.codeType ?? 'NULL'}</span>
                    <button type="button" class="ml-2 px-2 py-1 border rounded bg-white"
                      disabled={speciesEditingDisabled || speciesEventError(cell.raw) !== null || speciesEventError(option.code) !== null}
                      title={speciesEventError(cell.raw) ?? speciesEventError(option.code) ?? 'Stage the existing personal-list code; no personal-list write'}
                      onclick={() => option.code !== null && chooseSpeciesCode(Number(id), 'user', option.code)}>Use personal code {option.code ?? 'NULL'}, row {id}</button>
                  </div>
                {:else}
                  {#if personalSpeciesEnabled}
                    <button type="button" class="mt-2 rounded border bg-white px-2 py-1" data-personal-create-row={id}
                      disabled={speciesEditingDisabled || speciesEventError(cell.raw) !== null}
                      onclick={() => startPersonalSpecies(Number(id))}>Create personal definition, row {id}</button>
                  {:else}
                    <p role="alert">No source replacement or existing personal code was found. Personal-list creation is unavailable; correct or Cancel the draft.</p>
                  {/if}
                {/each}
              {/if}
              {#if speciesEventError(cell.raw)}<p role="alert">{speciesEventError(cell.raw)}</p>{/if}
            {/if}
          </section>
        {/if}
      {/each}
    </div>
  {/if}
  {#if personalDraft}
    <section class="personal-species-draft mb-3 rounded border border-amber-300 bg-amber-50 p-3 text-xs" aria-label="New personal species definition"
      data-personal-row-id={personalDraft.source.kind === 'existing' ? personalDraft.source.id : undefined} data-personal-editor={personalDraft.source.kind}>
      <h3 class="mb-2 font-semibold">New personal species definition - not yet saved</h3>
      <p class="mb-2" role="status">Saving here creates a reusable user-database definition and audit only. The plot assignment or new-row proposal remains unsaved. A later plot Undo does not delete a saved definition.</p>
      {#each personalInvalid as message}<p class="mb-2 text-red-700" role="alert">{message}</p>{/each}
      <PersonalSpeciesFields draft={personalDraft} disabled={busy || headerWorkflowBusy} onchange={onPersonalDraftChange} />
      <div class="flex gap-2">
        <button type="button" class="rounded border bg-white px-2 py-1 disabled:opacity-50" disabled={busy || headerWorkflowBusy || personalInvalid.length > 0}
          onclick={() => void savePersonalSpecies()}>Save personal definition only</button>
        <button type="button" class="rounded border bg-white px-2 py-1 disabled:opacity-50" disabled={busy || headerWorkflowBusy}
          onclick={cancelPersonalSpecies}>Cancel personal definition entry</button>
      </div>
      <p class="mt-2">Names preserve case and spacing, with a 255-UTF-16-unit bound. NULL and empty text remain distinct. Report, SppNumber and Codetype retain their original user-table defaults; no U/X classification is inferred.</p>
    </section>
  {/if}
  {#if collectedUnsaved}
    <div class="collected-draft-toolbar mb-3 p-2 border border-amber-300 bg-amber-50 text-xs" aria-label="Collected draft controls">
      <span>Collected drafts are unsubmitted. Switching tabs or cover/height views never saves them; other editing waits.</span>
      {#each collectedNotices as message}<p class="mt-2 font-medium" role="status">{message}</p>{/each}
      <div class="flex gap-2 mt-2">
        <button type="button" class="px-2 py-1 border rounded bg-emerald-700 text-white disabled:opacity-50" disabled={collectedEditingDisabled} onclick={() => void saveCollectedDrafts()}>Save Collected drafts</button>
        <button type="button" class="px-2 py-1 border border-stone-300 rounded bg-white disabled:opacity-50" disabled={busy || headerWorkflowBusy} onclick={() => void cancelCollectedDrafts()}>Cancel Collected drafts</button>
      </div>
    </div>
  {/if}
  {#if attributeUnsaved}
    <div class="attribute-draft-toolbar mb-3 p-2 border border-amber-300 bg-amber-50 text-xs" aria-label="Vegetation attribute draft controls">
      <span>Vegetation attribute drafts are unsubmitted. Switching tabs never saves them; other editing waits.</span>
      <div class="flex gap-2 mt-2">
        <button type="button" class="px-2 py-1 border rounded bg-emerald-700 text-white disabled:opacity-50" disabled={attributeEditingDisabled || attributeInvalid.length > 0} onclick={() => void saveAttributeDrafts()}>Save vegetation attributes</button>
        <button type="button" class="px-2 py-1 border border-stone-300 rounded bg-white disabled:opacity-50" disabled={busy || headerWorkflowBusy} onclick={() => void cancelAttributeDrafts()}>Cancel vegetation attributes</button>
      </div>
      {#each attributeInvalid as message}<p role="alert">{message}</p>{/each}
    </div>
  {/if}
  {#if soilUnsaved}
    <div class="soil-draft-toolbar mb-3 p-2 border border-amber-300 bg-amber-50 text-xs" aria-label="Soil draft controls">
      <span>Humus/Mineral drafts are unsubmitted. Switching tabs never saves them; other editing waits.</span>
      <div class="flex gap-2 mt-2">
        <button type="button" class="px-2 py-1 border rounded bg-emerald-700 text-white disabled:opacity-50" disabled={soilEditingDisabled || soilInvalid.length > 0} onclick={() => void saveSoilDrafts()}>Save soil drafts</button>
        <button type="button" class="px-2 py-1 border border-stone-300 rounded bg-white disabled:opacity-50" disabled={busy || headerWorkflowBusy} onclick={() => void cancelSoilDrafts()}>Cancel soil drafts</button>
      </div>
      {#each soilInvalid as message}<p role="alert">{message}</p>{/each}
    </div>
  {/if}
  {#if otherUnsaved}
    <div class="other-draft-toolbar mb-3 p-2 border border-amber-300 bg-amber-50 text-xs" aria-label="Other draft controls">
      <span>Other drafts are unsubmitted. Switching tabs never saves them; header and other child editing wait.</span>
      <div class="flex gap-2 mt-2">
        <button type="button" class="px-2 py-1 border rounded bg-emerald-700 text-white disabled:opacity-50" disabled={otherEditingDisabled || otherInvalid.length > 0} onclick={() => void saveOtherDrafts()}>Save Other drafts</button>
        <button type="button" class="px-2 py-1 border border-stone-300 rounded bg-white disabled:opacity-50" disabled={busy || headerWorkflowBusy} onclick={() => void cancelOtherDrafts()}>Cancel Other drafts</button>
      </div>
      {#each otherInvalid as message}<p role="alert">{message}</p>{/each}
    </div>
  {/if}
  {#if creationDraft}
    <section class="vegetation-creation-draft mb-3 rounded border border-amber-300 bg-amber-50 p-3 text-xs" aria-label="New vegetation record">
      <h3 class="mb-2 font-semibold">New vegetation record - {creationDraft.form} - not yet saved</h3>
      {#if savedPersonalCodes.length}<p class="mb-2" role="status">Saved personal definitions: {savedPersonalCodes.join(', ')}. Cancelling this new-row proposal never removes these user-database records.</p>{/if}
      {#if creationInvalid.length}<p class="mb-2 text-red-700" role="alert">{creationInvalid[0]}</p>{/if}
      {#if creationNotice}<p class="mb-2 font-medium" role="status">{creationNotice}</p>{/if}
      <label class="mb-2 flex flex-col gap-1" for="creation-species">Species code
        <input id="creation-species" class="rounded border bg-white px-2 py-2" type="text" value={creationDraft.species} list="creation-species-list"
          disabled={busy || headerWorkflowBusy || personalDraft !== null} aria-invalid={creationSpeciesInvalid !== null}
          oninput={event => stageCreationSpecies(event.currentTarget.value)} />
      </label>
      <datalist id="creation-species-list">
        {#each speciesLists[creationDraft.form] ?? [] as option, index (index)}
          {#if option.code !== null}<option value={option.code}>{option.scientificName ?? 'NULL'} | {option.englishName ?? 'NULL'} | Lifeform {option.lifeform ?? 'NULL'}</option>{/if}
        {/each}
      </datalist>
      {#if creationDraft.decision}
        <p class="mb-2" role="status">Explicit {creationDraft.decision.kind} decision for "{creationDraft.decision.entered}" stages "{creationDraft.species}". No new row or personal definition is saved yet.</p>
      {:else if creationSpeciesInvalid}
        <section class="mb-3 border-t border-amber-300 pt-2" aria-label="New vegetation species choices">
          <button type="button" class="rounded border bg-white px-2 py-1" disabled={busy || headerWorkflowBusy || personalDraft !== null || !speciesReferenceReady || speciesReferenceBusy || speciesEventError(creationDraft.species) !== null}
            title={speciesEventError(creationDraft.species) ?? 'Review source definitions without saving'}
            onclick={() => void reviewCreationSpeciesChoices()}>Review new vegetation species code</button>
          {#if speciesDecisionBusy}<p role="status">Looking up source species choices...</p>{/if}
          {#if creationChoices && creationChoices.form === creationDraft.form && creationChoices.entered === creationDraft.species}
            {@const hasAlias = creationChoices.aliases.some(option => option.code !== null)}
            {#each creationChoices.aliases as option, index (index)}
              <div class="mt-1">
                <span>{option.code ?? 'NULL'} | {option.scientificName ?? 'NULL'} | {option.englishName ?? 'NULL'} | Lifeform {option.lifeform ?? 'NULL'} | {option.codeType ?? 'NULL'}</span>
                <button type="button" class="ml-2 rounded border bg-white px-2 py-1" disabled={busy || headerWorkflowBusy || personalDraft !== null || speciesEventError(option.code) !== null}
                  title={speciesEventError(option.code) ?? 'Stage explicit replacement; Save remains separate'}
                  onclick={() => option.code !== null && chooseCreationSpecies('replace', option.code)}>Use replacement {option.code ?? 'NULL'} for new row</button>
              </div>
            {/each}
            {#if hasAlias}
              <button type="button" class="mt-1 rounded border bg-white px-2 py-1" disabled={busy || headerWorkflowBusy || personalDraft !== null}
                onclick={() => chooseCreationSpecies('keep')}>Keep entered code for new row</button>
            {:else}
              {#each creationChoices.users as option, index (index)}
                <div class="mt-1">
                  <span>{option.code ?? 'NULL'} | {option.scientificName ?? 'NULL'} | {option.englishName ?? 'NULL'} | Lifeform {option.lifeform ?? 'NULL'} | {option.codeType ?? 'NULL'}</span>
                  <button type="button" class="ml-2 rounded border bg-white px-2 py-1" disabled={busy || headerWorkflowBusy || personalDraft !== null || speciesEventError(option.code) !== null}
                    title={speciesEventError(option.code) ?? 'Stage an existing personal code; no user-database write'}
                    onclick={() => option.code !== null && chooseCreationSpecies('user', option.code)}>Use personal code {option.code ?? 'NULL'} for new row</button>
                </div>
              {:else}
                {#if personalSpeciesEnabled}
                  <button type="button" class="mt-2 rounded border bg-white px-2 py-1" data-personal-create-source={creationDraft.form}
                    disabled={busy || headerWorkflowBusy || personalDraft !== null || speciesEventError(creationDraft.species) !== null}
                    onclick={startCreationPersonalSpecies}>Create personal definition for new row</button>
                {:else}
                  <p role="alert">No source replacement or existing personal code was found. Personal-list creation is unavailable; correct or Cancel this proposal. Personal definitions saved separately remain reusable.</p>
                {/if}
              {/each}
            {/if}
          {/if}
        </section>
      {/if}
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
        {#each Object.entries(creationDraft.cells) as [column, cell] (column)}
          {@const control = paperChild(creationDraft.form).controls.find(control => control.column?.toLowerCase() === column.toLowerCase())}
          <label class="flex flex-col gap-1" for={`creation-${column}`}>{control ? controlLabel(control) : column}
            <input id={`creation-${column}`} class="rounded border bg-white px-2 py-2" type="text" inputmode="decimal" value={cell?.raw ?? ''}
              disabled={busy || headerWorkflowBusy || personalDraft !== null} aria-invalid={cell?.error != null} title={cell?.error ?? 'Blank is NULL; no zero cover is inferred.'}
              oninput={event => stageCreationCell(column, event.currentTarget.value)} />
          </label>
        {/each}
      </div>
      <div class="mt-3 flex gap-2">
        <button type="button" class="rounded border bg-white px-2 py-1 disabled:opacity-50" disabled={busy || headerWorkflowBusy || personalDraft !== null || creationInvalid.length > 0 || !speciesReferenceReady || speciesReferenceBusy} onclick={() => void saveVegetationCreation()}>Save new vegetation record</button>
        <button type="button" class="rounded border bg-white px-2 py-1 disabled:opacity-50" disabled={busy || headerWorkflowBusy || personalDraft !== null} onclick={() => void cancelVegetationCreation()}>Cancel vegetation creation</button>
      </div>
      <p class="mt-2">Select an exact canonical code or explicitly review replacement, keep or existing personal-code decisions. Enter source numeric values explicitly. Blank fields remain NULL; Layer, covers, totals and personal metadata are not inferred. Identity is allocated only by the transactional writer; saving a new row never writes the user database.</p>
    </section>
  {/if}
  {#if deletionReview}
    <section class="vegetation-deletion-review mb-3 rounded border border-red-300 bg-red-50 p-3 text-xs" aria-label="Vegetation deletion review">
      <p role="alert">Delete the entire vegetation record for {deletionReview.species ?? 'NULL'}, row {deletionReview.id}, plot {plotNumber}? This removes every stored field from all cover/height/attribute views, not just {deletionReview.form}. The deleted identity remains reserved. Ordinary Save never confirms this action.</p>
      <details class="mt-2"><summary>Stored columns being removed</summary><p>{deletionReview.columns?.join(', ')}</p></details>
      <div class="mt-2 flex gap-2">
        <button type="button" class="rounded border bg-red-700 px-2 py-1 text-white disabled:opacity-50" disabled={busy || headerWorkflowBusy} onclick={() => void confirmVegetationDeletion()}>Confirm deletion of row {deletionReview.id}</button>
        <button type="button" class="rounded border bg-white px-2 py-1 disabled:opacity-50" disabled={busy || headerWorkflowBusy} onclick={() => void cancelVegetationDeletion()}>Cancel deletion review</button>
      </div>
    </section>
  {/if}
  {#if heightUnsaved}
    <div class="height-draft-toolbar mb-3 p-2 border border-amber-300 bg-amber-50 text-xs" aria-label="Height draft controls">
      <span>Height/cover drafts are unsubmitted. Switching views never saves them; header and other child editing wait.</span>
      {#each heightNotices as message}<p class="mt-2 font-medium" role="status">{message}</p>{/each}
      <div class="flex gap-2 mt-2">
        <button type="button" class="px-2 py-1 border rounded bg-emerald-700 text-white disabled:opacity-50" disabled={heightEditingDisabled || heightInvalid.length > 0} onclick={() => void saveHeightDrafts()}>Save height/cover drafts</button>
        <button type="button" class="px-2 py-1 border border-stone-300 rounded bg-white disabled:opacity-50" disabled={busy || headerWorkflowBusy} onclick={() => void cancelHeightDrafts()}>Cancel height/cover drafts</button>
      </div>
      {#each heightInvalid as message}<p role="alert">{message}</p>{/each}
      {#if Object.values(heightDrafts).some(row => row.cover6?.value === null && row.cover6?.expected !== null)}
        <p>Saving NULL Cover6 removes that row from the C-height view after refresh, but does not delete the vegetation record.</p>
      {/if}
    </div>
  {/if}
{:else if region === 'speciesModal'}
  {#if showSpeciesModal}
    <div class="fixed inset-0 bg-stone-900/40 backdrop-blur-xs flex items-center justify-center z-50 p-4">
      <div class="bg-white rounded-lg shadow-xl border border-stone-200 w-full max-w-lg flex flex-col max-h-[80vh]">
        <div class="p-3 border-b border-stone-200 flex justify-between items-center bg-stone-50 rounded-t-lg">
          <h4 class="font-bold text-sm text-stone-900">BC Master Species Search</h4>
          <button type="button" onclick={() => { showSpeciesModal = false; }} class="text-stone-400 hover:text-stone-700 p-1"><X size={16} /></button>
        </div>
        <div class="p-3 border-b border-stone-100 flex gap-2">
          <input type="text" placeholder="Search code (e.g. ABIE) or name (e.g. fir, spruce)..." bind:value={speciesSearchQuery}
            oninput={() => void searchSpecies()} class="flex-1 border border-stone-300 rounded px-3 py-1.5 text-xs bg-white" />
          <button type="button" onclick={() => void searchSpecies()} class="px-3 py-1.5 bg-emerald-700 text-white rounded text-xs font-medium hover:bg-emerald-800">Search</button>
        </div>
        <div class="p-2 overflow-y-auto flex-1 divide-y divide-stone-100">
          {#if searchingSpecies}
            <div class="p-4 text-center text-xs text-stone-400">Searching catalog...</div>
          {:else if speciesSearchResults.length > 0}
            {#each speciesSearchResults as sp}
              <button type="button" class="w-full p-2 hover:bg-emerald-50 rounded flex justify-between items-center text-left" onclick={() => void selectSpeciesToAdd(sp)}>
                <div>
                  <div class="flex items-center gap-2">
                    <span class="font-mono font-bold text-xs text-emerald-900 bg-emerald-100 px-1.5 py-0.5 rounded">{sp.code}</span>
                    <span class="font-medium text-xs text-stone-900 italic">{sp.scientificName}</span>
                  </div>
                  {#if sp.englishName}<div class="text-[11px] text-stone-500 mt-0.5">{sp.englishName}</div>{/if}
                </div>
                <span class="px-2 py-1 bg-emerald-700 hover:bg-emerald-800 text-white text-[11px] rounded font-medium">Select</span>
              </button>
            {/each}
          {:else if speciesSearchQuery.trim()}
            <div class="p-4 text-center text-xs text-stone-400">No matching species found.</div>
          {:else}
            <div class="p-4 text-center text-xs text-stone-400">Type a code or species name above to search 10,500+ BC flora.</div>
          {/if}
        </div>
      </div>
    </div>
  {/if}
{:else if newChild}
  <NewChild kind={newChild} {busy} {error} oncreate={createChild} oncancel={cancelNewChild} />
{/if}
