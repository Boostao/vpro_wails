<script lang="ts">
  import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
  import type { SIVIParentDraft, SIVIParentInput } from './siviParentEditor';
  import type { SIVIParentDirectColumn } from './siviParentWriteSession';
  import { metadataCellText } from './projectMetadataEditor';
  let { column, original, draft, disabled, caption, onstage }: {
    column: SIVIParentDirectColumn; original: ProjectMetadataCell; draft?: SIVIParentDraft;
    disabled: boolean; caption: string; onstage: (input: SIVIParentInput) => void;
  } = $props();
  const id = $derived(`sivi-direct-${column}`);
  const boolean = $derived(column === 'SV_FloodPlain');
  const option = $derived(column === 'SV_StandAgeEstMeas' || column === 'SV_StandHeightEstMeas');
  const categorical = $derived(['SnowCoverregime', 'SV_RootZoneTexture', 'SV_AhorizonType'].includes(column));
  const numeric = $derived(!boolean && !option && !categorical && column !== 'SV_PolygonNumber' && column !== 'SV_CanopyComposition');
  const raw = $derived(draft?.input.kind === 'text' ? draft.input.raw
    : draft?.input.kind === 'clear' || draft?.input.kind === 'empty' ? '' : metadataCellText(draft?.value ?? original));
  const selected = $derived.by(() => {
    const value = draft?.value ?? original;
    if (value.storage === 'null') return 'null';
    if (boolean && value.storage === 'integer' && (value.integer === '-1' || value.integer === '0')) return value.integer === '-1' ? 'true' : 'false';
    if (option && value.storage === 'text' && (value.text === '1' || value.text === '2')) return value.text;
    return 'original';
  });
  function select(value: string) {
    if (value === 'original') onstage({ kind: 'original' });
    else if (value === 'null') onstage({ kind: 'clear' });
    else if (boolean) onstage({ kind: 'boolean', value: value === 'true' });
    else onstage({ kind: 'option', option: Number(value) });
  }
</script>

<dd class="space-y-2" data-sivi-direct-field={column}>
  {#if boolean || option}
    <select {id} {disabled} class="w-full min-w-0 rounded border p-2" value={selected}
      aria-describedby={`${id}-original`} aria-invalid={Boolean(draft?.error)}
      onchange={event => select(event.currentTarget.value)}>
      <option value="original">Retain original storage</option>
      <option value="null">NULL</option>
      {#if boolean}<option value="true">Yes (-1)</option><option value="false">No (0)</option>
      {:else}<option value="1">Est.</option><option value="2">Meas.</option>{/if}
    </select>
  {:else}
    <input {id} {disabled} type="text" inputmode={numeric ? 'decimal' : 'text'} value={raw}
      class="w-full min-w-0 rounded border p-2" aria-describedby={`${id}-original`} aria-invalid={Boolean(draft?.error)}
      oninput={event => onstage({ kind: 'text', raw: event.currentTarget.value })} />
  {/if}
  <div class="flex flex-wrap gap-2 text-xs">
    <button type="button" {disabled} class="rounded border px-2 py-1" aria-label={`Set NULL ${caption}`}
      onclick={() => onstage({ kind: 'clear' })}>Set NULL</button>
    {#if categorical}
      <button type="button" {disabled} class="rounded border px-2 py-1" aria-label={`Set empty TEXT ${caption}`}
        onclick={() => onstage({ kind: 'empty' })}>Set empty TEXT</button>
    {/if}
    <button type="button" {disabled} class="rounded border px-2 py-1" aria-label={`Retain original ${caption}`}
      onclick={() => onstage({ kind: 'original' })}>Retain original</button>
  </div>
  <p id={`${id}-original`} class="text-xs text-stone-600 whitespace-pre-wrap break-words">Original:
    {original.storage === 'null' ? 'NULL' : original.storage === 'text' && original.text === '' ? '(empty text)' : metadataCellText(original)}
    ({original.storage})</p>
  {#if draft?.error}<p role="alert" class="text-sm text-red-700">{draft.error}</p>{/if}
</dd>
