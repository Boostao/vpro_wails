<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { SoilCodeService } from '../bindings/github.com/boostao/vpro-wails';
  import { QualityLookup } from './qualityEditor';

  let { disabled = false }: { disabled?: boolean } = $props();
  const enabled = import.meta.env.VITE_SOIL_CODES_REFERENCE === 'true';
  const groupLookup = new QualityLookup(async () => {
    const rows = await SoilCodeService.ListGreatGroupChoices();
    if (rows === null) throw new Error('Soil great-group service did not return reference rows.');
    return rows;
  }, next => { group = next; }, 'Soil great group');
  const subgroupLookup = new QualityLookup(async () => {
    const rows = await SoilCodeService.ListSubgroupChoices();
    if (rows === null) throw new Error('Soil subgroup service did not return reference rows.');
    return rows;
  }, next => { subgroup = next; }, 'Soil subgroup');
  let group = $state(groupLookup.snapshot());
  let subgroup = $state(subgroupLookup.snapshot());
  const busy = $derived(group.busy || subgroup.busy);

  onMount(() => {
    if (enabled) { void groupLookup.refresh(); void subgroupLookup.refresh(); }
  });
  onDestroy(() => { groupLookup.dispose(); subgroupLookup.dispose(); });
</script>

{#if enabled}
  <section class="soil-code-reference" aria-label="Readonly soil classification reference">
    <p>Reference only: soil great group and subgroup remain readonly while their source editing behavior is verified.
      Empty Items and descriptions are metadata, not stored selections.</p>
    {#if busy}<p role="status">Refreshing soil classification reference...</p>{/if}
    {#each [group, subgroup] as view, index}
      {@const label = index === 0 ? 'Great group' : 'Soil subgroup'}
      {#if view.error}
        <p role="alert">{view.error}</p>
        <button type="button" disabled={disabled || busy}
          onclick={() => void (index === 0 ? groupLookup : subgroupLookup).refresh()}>Retry {label} reference</button>
      {/if}
      <details>
        <summary>{label}: {view.ready ? view.choices.length : 'Pending'} reference rows</summary>
        {#each view.choices as row (row.rowId)}
          <details data-reference-row={row.rowId} data-reference-list={row.listName}>
            <summary>{row.code === null ? 'NULL' : row.code === '' ? '""' : row.code}
              {row.selectable ? '' : ` - ${row.diagnostic}`}</summary>
            <dl>
              {#each Object.entries(row) as [name, value]}
                <dt>{name}</dt>
                <dd data-property={name} data-value={JSON.stringify(value)}
                  data-kind={value === null ? 'null' : typeof value}
                  data-empty={value === '' ? 'true' : undefined}
                  data-negative-zero={typeof value === 'number' && Object.is(value, -0) ? 'true' : undefined}>
                  {value === null ? 'NULL' : value === '' ? '""' : String(value)}
                </dd>
              {/each}
            </dl>
          </details>
        {/each}
      </details>
    {/each}
  </section>
{/if}

<style>
  .soil-code-reference { display: grid; gap: .3rem; margin-bottom: .5rem; padding: .5rem; border: 1px solid #d6d3d1; font-size: .75rem; }
  .soil-code-reference p { margin: 0; }
  .soil-code-reference button { padding: .2rem .4rem; border: 1px solid #a8a29e; background: white; }
  .soil-code-reference dl { display: grid; grid-template-columns: max-content 1fr; gap: .1rem .6rem; }
  .soil-code-reference dd { margin: 0; overflow-wrap: anywhere; }
</style>
