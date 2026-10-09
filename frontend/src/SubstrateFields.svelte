<script lang="ts">
  import { untrack, type Snippet } from 'svelte';
  import type { FS882Header } from '../bindings/github.com/boostao/vpro-wails';
  import type { PaperControl } from './paperLayout';
  import FieldGuidance from './FieldGuidance.svelte';
  import { substrateKey, substrateLabels, stageSubstrate, substrateErrors, substrateValidation,
    substrateSession, rememberSubstrateSession, type SubstrateValues, type SubstrateDrafts, type SubstrateKey } from './substrateEditor';

  let { draft = $bindable(), original, controls, position, capabilities, disabled, onchange, onvalidation, children }: {
    draft: FS882Header; original: SubstrateValues | null; controls: PaperControl[];
    position: (control: PaperControl) => string; capabilities: Record<string, boolean | undefined>;
    disabled: boolean; onchange: () => void;
    onvalidation: (key: 'substrate', message: string | null) => void; children: Snippet<[Snippet<[PaperControl?]>]>;
  } = $props();
  const editingEnabled = import.meta.env.VITE_SUBSTRATE_EDITING !== 'false';
  let staged = $state<SubstrateDrafts>({});
  const errors = $derived(substrateErrors(staged));
  $effect(() => {
    const identity = draft;
    const baseline = original;
    untrack(() => { staged = substrateSession(identity, baseline); });
  });
  $effect(() => {
    const message = editingEnabled ? substrateValidation(staged, draft, original) : null;
    untrack(() => onvalidation('substrate', message));
  });
  function stage(key: SubstrateKey, raw: string): void {
    const previous = staged[key];
    staged = stageSubstrate(staged, key, raw, original?.[key] ?? null);
    rememberSubstrateSession(draft, staged, original);
    const cell = staged[key];
    if (!cell) return;
    const changed = cell.error !== null || (previous?.error !== null && previous?.error !== undefined) || cell.value !== draft[key];
    if (cell.error === null) draft[key] = cell.value;
    if (changed) onchange();
  }
</script>

{#if editingEnabled}
  <div class="substrate-toolbar" aria-label="Substrate draft safety controls">
    {#each errors as message}<span role="alert">{message}</span>{/each}
  </div>
{/if}
{@render children(substrateInputs)}
{#if editingEnabled}
    <FieldGuidance title="Substrate entry guidance">
      <p>Substrate values are nullable numbers. Negative values, 100, 101 and totals above 100 are permitted; nothing is balanced or changed automatically.</p>
      <p>Save is explicit. Desktop values retain float64 precision within the finite Access Single range instead of rounding to Single.</p>
    </FieldGuidance>
{/if}
{#snippet substrateInputs(only?: PaperControl)}
  {#each (only ? [only] : controls).filter(control => substrateKey(control.column) && control.type === 'TextBox') as control (control.controlId)}
    {@const key = substrateKey(control.column)}
    {#if key}
      <input class="substrate-control" id={`header-${key}`} data-column={control.column} data-source-control={control.controlName}
        style={position(control)} type="text" inputmode="decimal" aria-label={substrateLabels[key]}
        value={staged[key]?.raw ?? String(draft[key] ?? '')}
        aria-invalid={editingEnabled && staged[key]?.error ? 'true' : undefined}
        disabled={!editingEnabled || disabled || capabilities[key] !== true || !control.enabled || control.locked}
        placeholder={capabilities[key] === true ? '' : 'Pending'}
        oninput={event => stage(key, event.currentTarget.value)} />
    {/if}
  {/each}
{/snippet}
<style>
  .substrate-toolbar { display: grid; gap: .3rem; padding: .5rem; margin-bottom: .5rem; border: 1px solid #d6d3d1; font-size: .75rem; }
  .substrate-control { box-sizing: border-box; width: 100%; min-width: 0; min-height: 40px; border: 1px solid #b7cfc0; border-radius: 4px; padding: .5rem .65rem; background: white; font: inherit; line-height: 1.4; }
  .substrate-control:disabled { color: #78716c; background: #f5f5f4; }
  .substrate-control:focus { outline: 2px solid #047857; outline-offset: 1px; }
  .substrate-control[aria-invalid="true"] { border-color: #b91c1c; }
</style>
