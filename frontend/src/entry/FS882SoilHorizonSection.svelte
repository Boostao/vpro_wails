<script lang="ts">
  import { Plus, Trash2 } from '@lucide/svelte';
  import type {
    FS882Header,
    HumusRecord,
    MineralRecord,
    SoilSuggestion,
  } from '../../bindings/github.com/boostao/vpro-wails';
  import DrainageFields from './DrainageFields.svelte';
  import GeologyCodeFields from './GeologyCodeFields.svelte';
  import OrdinaryFields from './OrdinaryFields.svelte';
  import ParentCodeFields from './ParentCodeFields.svelte';
  import SoilCodeFields from './SoilCodeFields.svelte';
  import SoilCodeReference from './SoilCodeReference.svelte';
  import SourceChild from './SourceChild.svelte';
  import SourcePage from './SourcePage.svelte';
  import { embeddedForm } from './paperLayout';
  import type { SoilDrafts, SoilKind } from './soilChildEditor';

  let {
    draft = $bindable(),
    original,
    capabilities,
    sourceHeaderValues,
    ordinarySoilUnavailable,
    historicalSoilUnavailable,
    soilEditingEnabled,
    soilCodeEditingEnabled,
    soilReferenceBusy,
    soilReferenceError,
    soilEditingDisabled,
    soilUnsaved,
    headerInputsDisabled,
    busy,
    headerWorkflowBusy,
    coordinateBusy,
    workingUnitBusy,
    qualityBusy,
    siteCodeBusy,
    regionCodeBusy,
    soilCodeBusy = $bindable(false),
    geologyCodeBusy = $bindable(false),
    parentCodeBusy = $bindable(false),
    drainageBusy = $bindable(false),
    childRevision,
    soilDrafts,
    soilSuggestions,
    humusList,
    mineralList,
    sourceRows,
    markDirty,
    validateHeader,
    loadSoilSuggestions,
    openNewChild,
    stageSoilCell,
    deleteSourceChild,
    deleteHumus,
  }: {
    draft: FS882Header;
    original: FS882Header | null;
    capabilities: Record<string, boolean | undefined>;
    sourceHeaderValues: Map<string, unknown>;
    ordinarySoilUnavailable: string | null;
    historicalSoilUnavailable: string | null;
    soilEditingEnabled: boolean;
    soilCodeEditingEnabled: boolean;
    soilReferenceBusy: boolean;
    soilReferenceError: string | null;
    soilEditingDisabled: boolean;
    soilUnsaved: boolean;
    headerInputsDisabled: boolean;
    busy: boolean;
    headerWorkflowBusy: boolean;
    coordinateBusy: boolean;
    workingUnitBusy: boolean;
    qualityBusy: boolean;
    siteCodeBusy: boolean;
    regionCodeBusy: boolean;
    soilCodeBusy: boolean;
    geologyCodeBusy: boolean;
    parentCodeBusy: boolean;
    drainageBusy: boolean;
    childRevision: number;
    soilDrafts: SoilDrafts;
    soilSuggestions: SoilSuggestion[];
    humusList: HumusRecord[];
    mineralList: MineralRecord[];
    sourceRows: (name: string) => Array<{ id: number; values: Record<string, string | number | boolean | null | undefined> }>;
    markDirty: () => void;
    validateHeader: (key: string, message: string | null) => void;
    loadSoilSuggestions: () => Promise<void>;
    openNewChild: (kind: 'humus' | 'mineral' | 'other') => void;
    stageSoilCell: (kind: SoilKind, id: number, column: string, raw: string) => void;
    deleteSourceChild: (name: string, id: number) => Promise<void>;
    deleteHumus: (id: number) => Promise<void>;
  } = $props();
</script>

{#if ordinarySoilUnavailable}<p role="status" data-soil-ordinary-unavailable>{ordinarySoilUnavailable}</p>{/if}
{#if historicalSoilUnavailable}<p role="status" data-soil-historical-unavailable>{historicalSoilUnavailable}</p>{/if}
{#if soilEditingEnabled && !ordinarySoilUnavailable}
  {#if soilReferenceBusy}<p role="status">Loading soil reference suggestions...</p>{/if}
  {#if soilReferenceError}
    <p role="alert">{soilReferenceError}</p>
    <button type="button" disabled={busy || soilReferenceBusy || headerWorkflowBusy} onclick={() => void loadSoilSuggestions()}>Retry soil references</button>
  {/if}
{/if}
{#if !soilCodeEditingEnabled}<SoilCodeReference disabled={busy || headerWorkflowBusy} />{/if}
<div class="flex gap-2 mb-3 text-xs">
  <button disabled={soilEditingDisabled || soilUnsaved} onclick={() => openNewChild('humus')}>Add Humus Layer</button>
  <button disabled={soilEditingDisabled || soilUnsaved} onclick={() => openNewChild('mineral')}>Add Mineral Layer</button>
</div>
<OrdinaryFields bind:draft {original} {capabilities} scope="soils"
  disabled={headerInputsDisabled}
  onchange={markDirty} onvalidation={validateHeader}>
{#snippet children(ordinaryEditor)}
<DrainageFields bind:draft {original} {capabilities}
  disabled={headerInputsDisabled}
  onchange={markDirty} onvalidation={validateHeader} onbusy={(pending) => drainageBusy = pending}>
{#snippet children(drainageEditor)}
<SoilCodeFields bind:draft {original} {capabilities}
  disabled={headerInputsDisabled || coordinateBusy || workingUnitBusy || qualityBusy || siteCodeBusy || regionCodeBusy || geologyCodeBusy}
  onchange={markDirty} onvalidation={validateHeader} onbusy={(pending) => soilCodeBusy = pending}>
  {#snippet children(editor)}
    <GeologyCodeFields bind:draft {original} {capabilities}
      disabled={headerInputsDisabled || coordinateBusy || workingUnitBusy || qualityBusy || siteCodeBusy || regionCodeBusy || soilCodeBusy}
      onchange={markDirty} onvalidation={validateHeader} onbusy={(pending) => geologyCodeBusy = pending}>
      {#snippet children(geologyEditor)}
        <ParentCodeFields bind:draft {original} {capabilities} scope="soils"
          disabled={headerInputsDisabled}
          onchange={markDirty} onvalidation={validateHeader} onbusy={(pending) => parentCodeBusy = pending}>
        {#snippet children(parentEditor)}
        <SourcePage name="Soil/Terrain" values={sourceHeaderValues} {editor} editors={[...(geologyEditor ? [geologyEditor] : []), ...(parentEditor ? [parentEditor] : []), ...(ordinaryEditor ? [ordinaryEditor] : []), ...(drainageEditor ? [drainageEditor] : [])]}>
          {#snippet embedded(control)}
            {@const child = embeddedForm(control.controlId)}
            {#if ordinarySoilUnavailable}
              <p role="status" data-soil-ordinary-unavailable={child.form}>{ordinarySoilUnavailable}</p>
            {:else if historicalSoilUnavailable}
              <p role="status" data-soil-historical-unavailable={child.form}>{historicalSoilUnavailable}</p>
            {:else}
              <SourceChild name={child.form} rows={sourceRows(child.form)} revision={childRevision} disabled={soilEditingDisabled}
              deleteDisabled={soilUnsaved} onsoilstage={soilEditingEnabled ? stageSoilCell : undefined}
              {soilDrafts} {soilSuggestions} ondelete={deleteSourceChild} />
            {/if}
          {/snippet}
        </SourcePage>
        {/snippet}
        </ParentCodeFields>
      {/snippet}
    </GeologyCodeFields>
  {/snippet}
</SoilCodeFields>
{/snippet}
</DrainageFields>
{/snippet}
</OrdinaryFields>
{#if soilEditingEnabled && !ordinarySoilUnavailable}
  <div class="mt-3 text-xs text-stone-600">
    <p>Suggestions are optional. Raw text is never completed or case-changed automatically; no soil totals are guessed.</p>
    <button type="button" class="mt-2 px-2 py-1 border rounded" disabled={busy || soilReferenceBusy || headerWorkflowBusy}
      onclick={() => void loadSoilSuggestions()}>Reload soil references</button>
  </div>
{/if}
{#if !ordinarySoilUnavailable}
<details class="mt-4 border border-stone-200 rounded p-3">
  <summary class="text-xs font-semibold cursor-pointer">Humus &amp; mineral horizon records</summary>
<div class="space-y-6">
  <!-- Humus Section -->
  <div class="space-y-3" data-help-id="humus.upper_lower_depth">
    <div class="flex justify-between items-center">
      <h3 class="font-bold text-xs uppercase tracking-wider text-stone-600">Humus Layers ({humusList.length})</h3>
      <button
        type="button"
        onclick={() => openNewChild('humus')}
        disabled={soilEditingDisabled || soilUnsaved}
        class="px-2.5 py-1 bg-stone-700 hover:bg-stone-800 text-white rounded text-xs flex items-center gap-1"
      >
        <Plus size={14} /> Add Humus Layer
      </button>
    </div>
    <div class="border border-stone-200 rounded overflow-x-auto bg-white">
      <table class="w-full text-left text-xs">
        <thead class="bg-stone-100 border-b border-stone-200 text-stone-600 font-semibold">
          <tr>
            <th class="p-2">Horizon</th>
            <th class="p-2">Upper Depth</th>
            <th class="p-2">Lower Depth</th>
            <th class="p-2">pH</th>
            <th class="p-2">Comment</th>
            <th class="p-2">Action</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-stone-100">
          {#each humusList as h (h.id)}
            <tr class="hover:bg-stone-50">
              <td class="p-2 font-semibold">{h.horizon ?? '—'}</td>
              <td class="p-2">{h.upperDepth ?? '—'}</td>
              <td class="p-2">{h.lowerDepth ?? '—'}</td>
              <td class="p-2">{h.ph ?? '—'}</td>
              <td class="p-2">{h.comment ?? '—'}</td>
              <td class="p-2">
                <button
                  type="button"
                  onclick={() => void deleteHumus(h.id)}
                  disabled={soilEditingDisabled || soilUnsaved}
                  class="text-red-600 hover:text-red-800 p-1"
                  title="Delete humus row"
                >
                  <Trash2 size={14} />
                </button>
              </td>
            </tr>
          {:else}
            <tr>
              <td colspan="6" class="p-4 text-center text-stone-400">{historicalSoilUnavailable ?? 'No humus layers recorded.'}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  </div>

  <!-- Mineral Section -->
  <div class="space-y-3" data-help-id="mineral.texture_cf">
    <h3 class="font-bold text-xs uppercase tracking-wider text-stone-600">Mineral Layers ({mineralList.length})</h3>
    <div class="border border-stone-200 rounded overflow-x-auto bg-white">
      <table class="w-full text-left text-xs">
        <thead class="bg-stone-100 border-b border-stone-200 text-stone-600 font-semibold">
          <tr>
            <th class="p-2">Horizon</th>
            <th class="p-2">Upper Depth</th>
            <th class="p-2">Lower Depth</th>
            <th class="p-2">Texture</th>
            <th class="p-2">Colour</th>
            <th class="p-2">Comments</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-stone-100">
          {#each mineralList as m (m.id)}
            <tr class="hover:bg-stone-50">
              <td class="p-2 font-semibold">{m.horizon ?? '—'}</td>
              <td class="p-2">{m.upperDepth ?? '—'}</td>
              <td class="p-2">{m.lowerDepth ?? '—'}</td>
              <td class="p-2">{m.texture ?? '—'}</td>
              <td class="p-2">{m.colour ?? '—'}</td>
              <td class="p-2">{m.comments ?? '—'}</td>
            </tr>
          {:else}
            <tr>
              <td colspan="6" class="p-4 text-center text-stone-400">{historicalSoilUnavailable ?? 'No mineral layers recorded.'}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  </div>
</div>
</details>
{/if}
