<script lang="ts">
  import { onDestroy, onMount } from 'svelte';
  import type { SpeciesCodeCheckOption, SpeciesCodeCheckRow } from '../bindings/github.com/boostao/vpro-wails';
  import type { bindContextPlots } from './contextPlots';
  import type { EditorCloseState } from './closeLifecycle';
  import { ReadRequests } from './readRequests';
  import { beginCodeCheck, stageCodeCheck, acceptCodeCheckTarget, ignoreCodeCheck, codeCheckErrors, codeCheckUpdates, type CodeCheckDraft } from './speciesCodeCheckEditor';
  import { beginCodeCheckPersonalSpecies, matchesCodeCheckPersonalSpeciesSource, personalSpeciesErrors, personalSpeciesRequest,
    personalSpeciesMatches, personalSpeciesCommittedError, type PersonalSpeciesDraft } from './personalSpeciesEditor';
  import { speciesEventError } from './vegetationSpeciesEditor';
  import PersonalSpeciesFields from './PersonalSpeciesFields.svelte';

  let { client, onclosed, onbusy, oncommitted }: {
    client: ReturnType<typeof bindContextPlots>; onclosed: () => void;
    onbusy: (value: boolean) => void; oncommitted: () => Promise<void>;
  } = $props();
  let session = $state<CodeCheckDraft | null>(null);
  let choices = $state<Record<string, { entered: string; options: SpeciesCodeCheckOption[] }>>({});
  let metadata = $state<PersonalSpeciesDraft | null>(null);
  let reading = $state(false);
  let saving = $state(false);
  let committedFailure = $state(false);
  let error = $state<string | null>(null);
  let success = $state<string | null>(null);
  let offset = $state(0);
  let generation = 0;
  const reads = new ReadRequests();
  const busy = $derived(reading || saving);
  const errors = $derived(session ? codeCheckErrors(session) : []);
  const metadataErrors = $derived(metadata ? personalSpeciesErrors(metadata) : []);
  const unresolved = $derived.by(() => {
    const current = session;
    return current ? current.review.rows.filter(row => ['unlisted', 'missing'].includes(row.status) && !current.ignored.includes(row.id)) : [];
  });
  const visible = $derived(unresolved.slice(offset, offset + 25));
  const pending = $derived(session ? Object.values(session.cells).filter(cell => cell.raw !== cell.row.code).length : 0);
  const personalEnabled = import.meta.env.VITE_PERSONAL_SPECIES_EDITING === 'true';

  $effect(() => onbusy(busy));
  onMount(() => { void reload(); });
  onDestroy(() => { generation++; reads.cancelAll(); onbusy(false); });

  export function getCloseState(): EditorCloseState {
    return { unsaved: true, busy, canSave: false, error,
      saveReason: 'Save species-check replacements or personal metadata explicitly, then close the review before saving or closing the plot.' };
  }
  export function undo() {
    if (saving) { error = 'Wait for the current species-check write before Undo.'; return; }
    if (metadata) { metadata = null; error = null; success = 'Personal entry cancelled; species-check proposals and saved definitions remain.'; return; }
    onclosed();
  }

  async function reload() {
    if (saving || pending || metadata) { error = 'Save or discard proposals before reloading the species-check scope.'; return; }
    const request = ++generation;
    reads.cancelAll();
    reading = true; error = null;
    try {
      const result = await reads.track(client.ReviewSpeciesCodes());
      if (request !== generation) return;
      session = beginCodeCheck(result); choices = {}; offset = 0;
    } catch (cause) {
      if (request === generation) error = `Species check unavailable; no data changed: ${String(cause)}`;
    } finally {
      if (request === generation) reading = false;
    }
  }

  function stage(id: number, raw: string) {
    if (!session || busy || metadata || committedFailure) { error = 'Finish the current operation before changing replacements.'; return; }
    try {
      generation++; reads.cancelAll();
      session = stageCodeCheck(session, id, raw);
      const next = { ...choices }; delete next[String(id)]; choices = next;
      error = null; success = null;
    } catch (cause) { error = `Species-check draft failed: ${String(cause)}`; }
  }

  async function reviewTarget(id: number) {
    const cell = session?.cells[String(id)];
    if (!cell || busy || metadata || committedFailure) { error = 'Enter an available replacement before reviewing it.'; return; }
    const request = ++generation;
    reads.cancelAll(); reading = true; error = null;
    try {
      const options = await reads.track(client.LookupSpeciesCodeCheckTarget({ code: cell.raw }));
      if (request !== generation) return;
      if (!session || session.cells[String(id)]?.raw !== cell.raw) throw new Error('Original replacement entry changed during review.');
      if (!options?.length) throw new Error('No exact master/user definition matches; select a registered literal code or Ignore.');
      choices = { ...choices, [String(id)]: { entered: cell.raw, options } };
    } catch (cause) {
      if (request === generation) error = `Replacement review unavailable; draft retained: ${String(cause)}`;
    } finally { if (request === generation) reading = false; }
  }

  function accept(id: number, all: boolean) {
    if (!session || busy || metadata || committedFailure) { error = 'An available replacement review is required.'; return; }
    try {
      const reviewed = choices[String(id)];
      if (!reviewed) throw new Error('Review this replacement before choosing its scope.');
      session = acceptCodeCheckTarget(session, id, reviewed.entered, reviewed.options, all);
      error = null; success = null;
    } catch (cause) { error = `Replacement choice failed; draft retained: ${String(cause)}`; }
  }

  function ignore(id: number, all: boolean) {
    if (!session || busy || metadata || committedFailure) { error = 'Wait for the current operation before ignoring codes.'; return; }
    try {
      session = ignoreCodeCheck(session, id, all); offset = 0;
      error = null; success = 'Ignore is local to this review. No project/user data or history changed.';
    } catch (cause) { error = `Ignore failed: ${String(cause)}`; }
  }

  async function save() {
    if (!session || busy || metadata || committedFailure) { error = 'An available review without personal entry is required.'; return; }
    let committed = false;
    saving = true; error = null; success = null;
    try {
      const updates = codeCheckUpdates(session);
      if (!updates.length) throw new Error('Choose an explicit changed replacement before saving; Ignore never writes.');
      await client.SaveSpeciesCodeCheck(updates);
      committed = true;
      session = null; choices = {};
      await oncommitted();
      session = beginCodeCheck(await client.ReviewSpeciesCodes());
      offset = 0;
      success = `Saved ${updates.length} explicit replacements atomically in the named scope. No user definitions were written or deleted.`;
    } catch (cause) {
      if (committed) committedFailure = true;
      error = committed ? `Replacements committed, but refresh failed. Close and reopen the plot before editing: ${String(cause)}`
        : `Species-check Save failed; proposals retained: ${String(cause)}`;
    } finally { saving = false; }
  }

  async function startPersonal(row: SpeciesCodeCheckRow) {
    if (!personalEnabled || !session || busy || metadata || committedFailure || row.code === null) {
      error = 'Personal entry is unavailable for this source row.'; return;
    }
    const request = ++generation;
    reading = true; error = null;
    try {
      const [aliases, users] = await Promise.all([
        reads.track(client.ListVegetationSpeciesAliases({ code: row.code })),
        reads.track(client.ListVegetationSpeciesUsers({ code: row.code })),
      ]);
      if (request !== generation) return;
      if (!Array.isArray(aliases) || !Array.isArray(users)) throw new Error('Source metadata lookup did not return complete definitions.');
      if (!session.review.rows.some(source => source.id === row.id && source.plotNumber === row.plotNumber && source.code === row.code)) {
        throw new Error('Original code-check source changed during metadata review.');
      }
      metadata = beginCodeCheckPersonalSpecies(row, { form: 'USysCodeCheck', entered: row.code, aliases, users });
    } catch (cause) {
      if (request === generation) error = `Personal metadata unavailable; no data changed: ${String(cause)}`;
    } finally { if (request === generation) reading = false; }
  }

  async function savePersonal() {
    const proposed = metadata;
    if (!personalEnabled || !proposed || proposed.source.kind !== 'codecheck' || busy || committedFailure) {
      error = 'An available code-check personal draft is required before separate Save.'; return;
    }
    let committed = false;
    const source = proposed.source;
    saving = true; error = null; success = null;
    try {
      const current = await client.ReviewSpeciesCodes();
      const rows = current.rows?.filter(row => row.id === source.id) ?? [];
      if (rows.length !== 1 || !matchesCodeCheckPersonalSpeciesSource(proposed, rows[0])) throw new Error('Original physical metadata source changed; cancel and review it again.');
      const request = personalSpeciesRequest(proposed);
      const result = await client.CreatePersonalSpeciesDefinition(request);
      committed = true;
      metadata = null;
      if (!personalSpeciesMatches(result, request)) throw new Error('Committed definition result does not match the original request.');
      const refreshed = await client.ReviewSpeciesCodes();
      const sources = refreshed.rows?.filter(row => row.id === source.id) ?? [];
      if (sources.length !== 1 || !matchesCodeCheckPersonalSpeciesSource(proposed, sources[0])) throw new Error('Definition saved, but the physical proposal source changed.');
      const options = await client.LookupSpeciesCodeCheckTarget({ code: request.entered.toUpperCase() });
      if (!options?.some(option => option.source === 'user' && personalSpeciesMatches(option, request))) throw new Error('Saved definition does not match independently reloaded metadata.');
      if (!session) throw new Error('Definition saved, but the original review was closed.');
      session = stageCodeCheck(session, source.id, request.entered.toUpperCase());
      session = acceptCodeCheckTarget(session, source.id, request.entered.toUpperCase(), options, false);
      success = 'Personal definition saved independently. Any changed project assignment remains an explicit separate proposal; closing/Undo never deletes this reusable definition.';
    } catch (cause) {
      if (committed || personalSpeciesCommittedError(cause)) { metadata = null; committedFailure = true; }
      error = committed || personalSpeciesCommittedError(cause)
        ? `Personal definition committed, but verification failed. Close and reopen before editing: ${String(cause)}`
        : `Personal Save failed; raw metadata retained: ${String(cause)}`;
    } finally { saving = false; }
  }
</script>

<section class="species-code-check mb-3 rounded border border-emerald-300 bg-emerald-50 p-3 text-sm" aria-label="Species code check">
  <h3 class="font-semibold">Species code check - explicit scope and replacements</h3>
  {#if session}
    <p class="mt-2" role="status">Project: {session.review.project}. Working unit: {session.review.su === 'None' ? 'None - entire project' : session.review.su}.
      {session.review.rows.length} physical rows reviewed; {unresolved.length} unresolved; {session.ignored.length} ignored locally.</p>
  {/if}
  {#if busy}<p role="status">Waiting for the owned context operation...</p>{/if}
  {#if error}<p class="mt-2 font-medium text-red-700" role="alert">{error}</p>{/if}
  {#if success}<p class="mt-2 font-medium" role="status">{success}</p>{/if}
  {#each errors as message}<p class="mt-2 text-red-700" role="alert">{message}</p>{/each}
  <div class="my-3 flex flex-wrap gap-2">
    <button type="button" disabled={busy || committedFailure || !!metadata || !pending || errors.length > 0} onclick={() => void save()}>Save reviewed replacements</button>
    <button type="button" disabled={busy || committedFailure || !!metadata || pending > 0} onclick={() => void reload()}>Reload species-check scope</button>
    <button type="button" disabled={saving} onclick={undo}>{metadata ? 'Cancel personal entry' : 'Discard proposals and close review'}</button>
  </div>
  {#if metadata}
    <section class="personal-species-draft rounded border border-amber-300 bg-amber-50 p-3" aria-label="New personal species definition" data-personal-editor="codecheck">
      <p class="mb-2">Separate Save writes only the user definition and its audit. Project replacements have their own transaction.</p>
      {#each metadataErrors as message}<p class="text-red-700" role="alert">{message}</p>{/each}
      <PersonalSpeciesFields draft={metadata} disabled={busy} onchange={value => { metadata = value; error = null; success = null; }} />
      <button type="button" disabled={busy || committedFailure || metadataErrors.length > 0} onclick={() => void savePersonal()}>Save personal definition only</button>
    </section>
  {/if}
  {#if session}
    <div class="overflow-x-auto">
      <table class="w-full">
        <thead><tr><th scope="col">Plot / physical ID</th><th scope="col">Original code</th><th scope="col">Literal replacement and review</th><th scope="col">Actions</th></tr></thead>
        <tbody>
          {#each visible as row (row.id)}
            {@const cell = session.cells[String(row.id)]}
            {@const target = choices[String(row.id)]}
            <tr data-code-check-id={row.id}>
              <td>{row.plotNumber} / {row.id}</td>
              <td>{row.code === null ? 'NULL' : JSON.stringify(row.code)}</td>
              <td>
                <label for={`check-replacement-${row.id}`}>Replacement, plot {row.plotNumber}, row {row.id}
                  <input id={`check-replacement-${row.id}`} type="text" value={cell?.raw ?? ''} disabled={busy || !!metadata || committedFailure}
                    aria-invalid={!!cell?.error} oninput={event => stage(row.id, event.currentTarget.value)} />
                </label>
                <button type="button" disabled={busy || !!metadata || committedFailure || !cell} onclick={() => void reviewTarget(row.id)}>Review literal target</button>
                {#if target && target.entered === cell?.raw}
                  <div aria-label={`Target definitions, row ${row.id}`}>
                    {#each target.options as option}
                      <p>{option.source}: {JSON.stringify(option.code)}; scientific {option.scientificName === null ? 'NULL' : JSON.stringify(option.scientificName)};
                        English {option.englishName === null ? 'NULL' : JSON.stringify(option.englishName)};
                        Lifeform {option.lifeform ?? 'NULL'}; Codetype {option.codeType === null ? 'NULL' : JSON.stringify(option.codeType)}.</p>
                    {/each}
                    <button type="button" disabled={busy || !!metadata || committedFailure} onclick={() => accept(row.id, false)}>Use for this physical row</button>
                    <button type="button" disabled={busy || !!metadata || committedFailure} onclick={() => accept(row.id, true)}>Use for matching originals in this scope</button>
                  </div>
                {/if}
              </td>
              <td>
                <button type="button" disabled={busy || !!metadata || committedFailure} onclick={() => ignore(row.id, false)}>Ignore this row</button>
                <button type="button" disabled={busy || !!metadata || committedFailure} onclick={() => ignore(row.id, true)}>Ignore matching codes locally</button>
                {#if personalEnabled && row.status === 'unlisted' && row.code !== null && !speciesEventError(row.code)}
                  <button type="button" disabled={busy || !!metadata || committedFailure} onclick={() => void startPersonal(row)}>Create personal definition</button>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    <div class="mt-2 flex gap-2">
      <button type="button" disabled={busy || offset === 0} onclick={() => offset = Math.max(0, offset - 25)}>Previous codes</button>
      <button type="button" disabled={busy || offset + 25 >= unresolved.length} onclick={() => offset += 25}>Next codes</button>
    </div>
    <p class="mt-2">Only unresolved rows are shown. Registered master/user codes remain untouched. Ignore never creates or deletes temporary LifeForm999 definitions.
      Replacement matching is literal; no trim, automatic completion or case repair is performed. Personal Save and project Save are separate.</p>
  {/if}
</section>

<style>
  th, td { padding: .5rem; text-align: left; vertical-align: top; border-bottom: 1px solid #b7cfc0; }
  input { display: block; min-height: 40px; width: 100%; padding: .5rem; border: 1px solid #b7cfc0; border-radius: 4px; background: white; }
  button { min-height: 40px; padding: .4rem .6rem; border: 1px solid #b7cfc0; border-radius: 4px; background: white; }
  button:disabled { opacity: .5; }
</style>
