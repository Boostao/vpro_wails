<script lang="ts">
  import { metadataCellText } from './projectMetadataEditor';
  import type { SIVIDeletionOriginal } from './siviDeletionEditor';

  let { columns, row }: {
    columns: SIVIDeletionOriginal['columns'];
    row: SIVIDeletionOriginal['original'];
  } = $props();
</script>

<dl class="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3" data-sivi-historical-row>
  {#each columns as column, index (column.name)}
    {@const cell = row.cells[index]}
    <div class="min-w-0 rounded border border-stone-200 p-2">
      <dt class="break-words text-sm font-medium">{column.name}</dt>
      <dd class="whitespace-pre-wrap break-words text-sm" data-sivi-historical-cell={column.name}>{cell.storage === 'null' ? 'NULL' : cell.storage === 'text' && cell.text === '' ? '(empty text)' : metadataCellText(cell)}</dd>
      <dd class="break-words text-xs text-stone-500">Original storage: {cell.storage}</dd>
    </div>
  {/each}
</dl>
