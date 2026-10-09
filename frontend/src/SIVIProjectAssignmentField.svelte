<script lang="ts">
  import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
  import type { SIVIProjectPhysicalChoices } from './siviParentSourceSession';
  import { siviProjectAssignmentValueError, type SIVIProjectAssignmentDrafts, type SIVIProjectAssignmentInput } from './siviProjectAssignmentSession';
  import { metadataCellText } from './projectMetadataEditor';
  let { original, choices, drafts, disabled, onstage }: {
    original: ProjectMetadataCell; choices: SIVIProjectPhysicalChoices; drafts: SIVIProjectAssignmentDrafts;
    disabled: boolean; onstage: (input: SIVIProjectAssignmentInput) => void;
  } = $props();
  const label = (cell: ProjectMetadataCell) => cell.storage === 'null' ? 'NULL'
    : cell.storage === 'text' && cell.text === '' ? '(empty text)' : metadataCellText(cell);
</script>

<dd data-sivi-project-assignment-field class="min-w-0">
  <select id="sivi-project-assignment" class="w-full min-w-0 rounded border px-2 py-1 text-sm disabled:opacity-50"
    value={drafts.rowId ?? 'original'} {disabled} aria-invalid={drafts.error !== null}
    onchange={event => onstage(event.currentTarget.value === 'original' ? { kind: 'original' }
      : { kind: 'selection', rowId: event.currentTarget.value })}>
    <option value="original">Retain original: {label(original)} ({original.storage})</option>
    {#each choices.Choices.rows as row (row.rowId)}
      {@const diagnostic = siviProjectAssignmentValueError(original, row.cells[0])}
      <option value={row.rowId} disabled={diagnostic !== null}>
        {label(row.cells[0])} — {label(row.cells[1])} [row {row.rowId}]{diagnostic ? ' (assignment unavailable)' : ''}
      </option>
    {/each}
  </select>
</dd>
