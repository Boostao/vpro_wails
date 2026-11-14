<script lang="ts">
  import { Plus, Trash2 } from '@lucide/svelte';
  import type { VegRecord } from '../../bindings/github.com/boostao/vpro-wails';

  let {
    vegList,
    totalTreeCover,
    totalShrubCover,
    totalHerbCover,
    totalMossCover,
    onopenspeciesmodal,
    ondeleteveg,
  }: {
    vegList: VegRecord[];
    totalTreeCover: number | null;
    totalShrubCover: number | null;
    totalHerbCover: number | null;
    totalMossCover: number | null;
    onopenspeciesmodal: () => void;
    ondeleteveg: (id: number) => Promise<void>;
  } = $props();
</script>

<details class="mt-4 border border-stone-200 rounded p-3">
  <summary class="text-xs font-semibold cursor-pointer">Vegetation record preview (read-only)</summary>
  <div class="space-y-4">
    <!-- Live Stratum Cover Summaries -->
    <div class="grid grid-cols-4 gap-3 bg-stone-50 border border-stone-200 rounded p-3 text-xs">
      <div class="flex flex-col">
        <span class="text-stone-500 font-medium">Tree Layer (A)</span>
        <span class="font-bold text-stone-900 text-sm">{totalTreeCover === null ? 'unavailable' : `${totalTreeCover.toFixed(1)}%`}</span>
      </div>
      <div class="flex flex-col">
        <span class="text-stone-500 font-medium">Shrub Layer (B)</span>
        <span class="font-bold text-stone-900 text-sm">{totalShrubCover === null ? 'unavailable' : `${totalShrubCover.toFixed(1)}%`}</span>
      </div>
      <div class="flex flex-col">
        <span class="text-stone-500 font-medium">Herb Layer (C)</span>
        <span class="font-bold text-stone-900 text-sm">{totalHerbCover === null ? 'unavailable' : `${totalHerbCover.toFixed(1)}%`}</span>
      </div>
      <div class="flex flex-col">
        <span class="text-stone-500 font-medium">Moss/Lichen (D)</span>
        <span class="font-bold text-stone-900 text-sm">{totalMossCover === null ? 'unavailable' : `${totalMossCover.toFixed(1)}%`}</span>
      </div>
    </div>

    <div class="flex justify-between items-center">
      <h3 class="font-bold text-xs uppercase tracking-wider text-stone-600">Vegetation Entries ({vegList.length})</h3>
      <button
        type="button"
        onclick={onopenspeciesmodal}
        disabled
        class="px-2.5 py-1 bg-emerald-700 hover:bg-emerald-800 text-white rounded text-xs flex items-center gap-1"
      >
        <Plus size={14} /> Add Species (BC Catalog)
      </button>
    </div>

    <div class="border border-stone-200 rounded overflow-x-auto bg-white">
      <table class="w-full text-left text-xs">
        <thead class="bg-stone-100 border-b border-stone-200 text-stone-600 font-semibold">
          <tr>
            <th class="p-2">Species</th>
            <th class="p-2">Layer</th>
            <th class="p-2">Cover 1</th>
            <th class="p-2">Cover 2</th>
            <th class="p-2">Total A</th>
            <th class="p-2">Cover 4</th>
            <th class="p-2">Cover 5</th>
            <th class="p-2">Total B</th>
            <th class="p-2">Cover 6</th>
            <th class="p-2">Cover 7</th>
            <th class="p-2">Action</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-stone-100">
          {#each vegList as row (row.id)}
            <tr class="hover:bg-stone-50">
              <td class="p-2 font-mono font-bold text-emerald-950">{row.species}</td>
              <td class="p-2"><input type="text" value={row.layer ?? ''} disabled class="w-10 border border-stone-200 rounded px-1 py-0.5 text-xs text-center" /></td>
              <td class="p-2"><input type="number" step="0.1" value={row.cover1 ?? ''} disabled class="w-14 border border-stone-200 rounded px-1 py-0.5 text-xs text-right" /></td>
              <td class="p-2"><input type="number" step="0.1" value={row.cover2 ?? ''} disabled class="w-14 border border-stone-200 rounded px-1 py-0.5 text-xs text-right" /></td>
              <td class="p-2 font-bold text-stone-800 text-right">{row.totalA?.toFixed(1) ?? '—'}</td>
              <td class="p-2"><input type="number" step="0.1" value={row.cover4 ?? ''} disabled class="w-14 border border-stone-200 rounded px-1 py-0.5 text-xs text-right" /></td>
              <td class="p-2"><input type="number" step="0.1" value={row.cover5 ?? ''} disabled class="w-14 border border-stone-200 rounded px-1 py-0.5 text-xs text-right" /></td>
              <td class="p-2 font-bold text-stone-800 text-right">{row.totalB?.toFixed(1) ?? '—'}</td>
              <td class="p-2"><input type="number" step="0.1" value={row.cover6 ?? ''} disabled class="w-14 border border-stone-200 rounded px-1 py-0.5 text-xs text-right" /></td>
              <td class="p-2"><input type="number" step="0.1" value={row.cover7 ?? ''} disabled class="w-14 border border-stone-200 rounded px-1 py-0.5 text-xs text-right" /></td>
              <td class="p-2">
                <button type="button" onclick={() => void ondeleteveg(row.id)} disabled class="text-red-600 hover:text-red-800 p-1" title="Delete entry">
                  <Trash2 size={14} />
                </button>
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="11" class="p-4 text-center text-stone-400">No vegetation records for this plot.</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  </div>
</details>
