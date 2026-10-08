<script lang="ts">
  import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
  import type { SIVIParentDraft, SIVIParentInput } from './siviParentEditor';
  import type { SIVIParentActionColumn } from './siviParentActionWriteSession';
  import { metadataCellText } from './projectMetadataEditor';
  let { column, original, draft, disabled, onstage }: {
    column: SIVIParentActionColumn; original: ProjectMetadataCell; draft?: SIVIParentDraft;
    disabled: boolean; onstage: (input: SIVIParentInput) => void;
  } = $props();
  const id = $derived(`sivi-action-${column}`);
  const selected = $derived(draft?.input.kind === 'option'
    ? draft.input.option === null ? 'null' : String(draft.input.option) : 'original');
</script>

<dd class="space-y-2" data-sivi-action-field={column}>
  <select {id} {disabled} value={selected} class="w-full min-w-0 rounded border p-2"
    aria-describedby={`${id}-original`} aria-invalid={Boolean(draft?.error)}
    onchange={event => onstage(event.currentTarget.value === 'original' ? { kind: 'original' }
      : { kind: 'option', option: event.currentTarget.value === 'null' ? null : Number(event.currentTarget.value) })}>
    <option value="original">Retain original storage (no action)</option>
    {#if column === 'PlotType'}
      <option value="1">Grnd (Ground)</option>
      <option value="2">Visual</option>
      <option value="3">Note</option>
      <option value="4">Full (FS882)</option>
      <option value="5">Other</option>
    {:else}
      <option value="1">Comp. (Yes, -1)</option>
      <option value="2">Part. (No, 0)</option>
      <option value="null">NULL (no selection)</option>
    {/if}
  </select>
  <p id={`${id}-original`} class="text-xs text-stone-600 whitespace-pre-wrap break-words">Original:
    {original.storage === 'null' ? 'NULL' : original.storage === 'text' && original.text === '' ? '(empty text)' : metadataCellText(original)}
    ({original.storage}). Selection stages only this source action; initialization never writes.</p>
  {#if draft?.error}<p role="alert" class="text-sm text-red-700">{draft.error}</p>{/if}
</dd>
