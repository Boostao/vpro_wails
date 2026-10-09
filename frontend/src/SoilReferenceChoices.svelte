<script lang="ts">
  import type { ProjectMetadataCell } from '../bindings/github.com/boostao/vpro-wails';
  import type { SoilReferenceField } from './twoPageSoilReferences';
  import { metadataCellText } from './projectMetadataEditor';

  let { field, label, disabled, onchoose }: {
    field: SoilReferenceField; label: string; disabled: boolean;
    onchoose: (definitionId: string, value: ProjectMetadataCell) => void;
  } = $props();
  function display(cell: ProjectMetadataCell): string {
    if (cell.storage === 'null') return 'NULL';
    if (cell.storage === 'text' && cell.text === '') return '"" (empty text)';
    if (cell.storage === 'real' && Object.is(cell.real, -0)) return '-0';
    return metadataCellText(cell);
  }
</script>

<details class="min-w-0 rounded border p-2 text-sm" data-soil-source-choices={field.column}>
  <summary class="cursor-pointer break-words">Source choices for {label}</summary>
  <p class="mt-2 break-words">Source list {field.listName}. Definitions retain physical identities, NULL/empty metadata and duplicates. Choosing stages a draft only; it does not Save or restrict manual entry.</p>
  <ul class="mt-2 space-y-2">
    {#each field.definitions.rows as row (row.rowId)}
      {@const code = row.cells[3]}
      {@const description = row.cells[4]}
      <li class="min-w-0 space-y-1 rounded border p-2" data-soil-reference-row={row.rowId}>
        <p class="break-words">{display(code)} - {display(description)}; physical definition {row.rowId}</p>
        <button type="button" class="max-w-full whitespace-normal break-words rounded border p-2"
          disabled={disabled || (code.storage !== 'text' && code.storage !== 'null')}
          onclick={() => onchoose(row.rowId, code)}>
          Choose {label}: {display(code)} (definition {row.rowId})
        </button>
        {#if code.storage !== 'text' && code.storage !== 'null'}<p role="alert">Historical nontext Item is retained but cannot be selected.</p>{/if}
        <details class="min-w-0">
          <summary class="cursor-pointer break-words">Definition metadata {row.rowId}</summary>
          <dl class="grid min-w-0 grid-cols-[minmax(0,1fr)_minmax(0,2fr)] gap-1">
            {#each field.definitions.columns as column, index}
              <dt class="break-words">{column.name}</dt>
              <dd class="break-words" data-soil-reference-property={column.name} data-storage={row.cells[index].storage}>
                {display(row.cells[index])} ({row.cells[index].storage})
              </dd>
            {/each}
          </dl>
        </details>
      </li>
    {/each}
  </ul>
</details>
