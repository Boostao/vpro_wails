<script lang="ts">
  import type { VegetationReferences } from './twoPageVegetationReferences';
  let { references }: { references: VegetationReferences } = $props();
  let masterPage = $state(0);
  let personalPage = $state(0);
  const pageSize = 20;
  const tables = $derived([
    { role: 'master', label: 'Master', table: references.master, page: Math.min(masterPage, Math.max(0, Math.ceil(references.master.rows.length / pageSize) - 1)) },
    { role: 'personal', label: 'Personal', table: references.personal, page: Math.min(personalPage, Math.max(0, Math.ceil(references.personal.rows.length / pageSize) - 1)) },
  ]);
  function page(role: string, value: number) {
    if (role === 'master') masterPage = value;
    else personalPage = value;
  }
</script>

<div class="min-w-0 space-y-3" data-vegetation-species-references={references.form}>
  <p class="break-words text-sm">Read-only physical Species metadata for {references.project}, parent {references.plot}.
    Duplicate codes and NULL/empty metadata are retained. These definitions are not source UNION choices, layer membership or Species editing.</p>
  {#each tables as entry (entry.role)}
    <details class="min-w-0 rounded border p-2" data-species-reference-source={entry.role}>
      <summary class="break-words">{entry.label} Species definitions: {entry.table.rows.length} physical row(s)</summary>
      <p class="break-words text-xs">{entry.role === 'master' ? 'VLists.USysAllSpecs; Lifeform: nullable signed32 LONG.' : 'VUser.USysUserSpp; LifeForm: nullable signed16 INTEGER.'}
        Pages retain physical reader order, not Access Species sorting.</p>
      <div class="flex flex-wrap items-center gap-2 py-2">
        <button type="button" disabled={entry.page === 0} onclick={() => page(entry.role, entry.page - 1)} aria-label={`Previous ${entry.label} Species page`}>Previous</button>
        <span>{entry.table.rows.length ? `Page ${entry.page + 1} of ${Math.ceil(entry.table.rows.length / pageSize)}` : 'No physical definitions'}</span>
        <button type="button" disabled={(entry.page + 1) * pageSize >= entry.table.rows.length}
          onclick={() => page(entry.role, entry.page + 1)} aria-label={`Next ${entry.label} Species page`}>Next</button>
      </div>
      {#each entry.table.rows.slice(entry.page * pageSize, (entry.page + 1) * pageSize) as row (row.rowId)}
        <section class="my-2 min-w-0 rounded border p-2" aria-label={`${entry.label} Species physical row ${row.rowId}`}>
          <h5 class="break-words text-sm font-semibold">Physical row {row.rowId}</h5>
          <dl class="grid min-w-0 grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
            {#each entry.table.columns as column, index}
              <div class="min-w-0">
                <dt class="break-words text-sm font-medium">{column.name} <span class="text-xs">({column.declaredType || 'untyped'})</span></dt>
                <dd class="whitespace-pre-wrap break-words text-sm">{row.cells[index].storage === 'null' ? 'NULL'
                  : row.cells[index].storage === 'integer' ? row.cells[index].integer : JSON.stringify(row.cells[index].text)}</dd>
              </div>
            {/each}
          </dl>
        </section>
      {/each}
    </details>
  {/each}
</div>
