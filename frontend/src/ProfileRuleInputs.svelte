<script lang="ts">
  import type { MetadataDraftCell } from './projectMetadataEditor';
  import type { ProfileField } from './projectProfileRuleEditor';
  let { prefix, identityLabel, fields, onstage }: {
    prefix: string; identityLabel: string;
    fields: { name: ProfileField; cell: MetadataDraftCell; options: string[] }[];
    onstage: (name: ProfileField, raw: string, nullValue: boolean) => void;
  } = $props();
</script>

<div class="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-3">
  {#each fields as { name, cell, options }}
    {@const id = `${prefix}-${name}`}
    <div class="min-w-0">
      <label class="block text-sm font-medium" for={id}>{name}</label>
      <input id={id} class="w-full min-w-0 border rounded px-2 py-1" type="text" value={cell.raw}
        list={`${id}-choices`} disabled={cell.nullValue} aria-invalid={cell.error !== null}
        aria-describedby={cell.error ? `${id}-error` : undefined}
        oninput={event => onstage(name, event.currentTarget.value, false)} />
      <datalist id={`${id}-choices`}>{#each options as option}<option value={option}></option>{/each}</datalist>
      <label class="flex gap-2 items-center text-xs mt-1"><input type="checkbox" checked={cell.nullValue}
        onchange={event => onstage(name, cell.raw, event.currentTarget.checked)} />NULL {name}, {identityLabel}</label>
      {#if cell.error}<p id={`${id}-error`} role="alert" class="text-xs text-red-800 mt-1">{cell.error}</p>{/if}
    </div>
  {/each}
</div>
