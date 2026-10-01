<script lang="ts">
  import { untrack, type Snippet } from 'svelte';
  import type { FS882Header } from '../bindings/github.com/boostao/vpro-wails';
  import type { PaperControl } from './paperLayout';
  import { ordinaryFields, ordinaryField, ordinaryTextError, ordinaryNumberValue, ordinaryValidation,
    ordinarySession, rememberOrdinarySession, type OrdinaryScope, type OrdinaryNumberField,
    type OrdinaryTextField, type OrdinaryDrafts } from './ordinaryEditor';
  let { draft = $bindable(), original, scope, capabilities, disabled, onchange, onvalidation, onVegNotesTab, children }: {
    draft: FS882Header; original: FS882Header | null; scope: OrdinaryScope;
    capabilities: Record<string, boolean | undefined>; disabled: boolean; onchange: () => void;
    onvalidation: (key: string, message: string | null) => void; onVegNotesTab?: () => void;
    children: Snippet<[{ columns: readonly string[]; input: Snippet<[PaperControl, string]> } | undefined]>;
  } = $props();
  const editingEnabled = import.meta.env.VITE_ORDINARY_PARENT_EDITING !== 'false';
  let staged = $state<OrdinaryDrafts>({});
  const fields = $derived(ordinaryFields.filter(field => field.scope === scope));
  const validation = $derived(ordinaryValidation(scope, draft, original, staged));
  $effect(() => {
    const identity = draft, baseline = original;
    untrack(() => { staged = ordinarySession(identity, baseline); });
  });
  $effect(() => {
    const message = editingEnabled ? validation : null;
    untrack(() => onvalidation(`ordinary-${scope}`, message));
  });
  function setText(field: OrdinaryTextField, raw: string): void {
    const value = raw === '' ? null : raw;
    if (draft[field.key] === value) return;
    draft[field.key] = value;
    onchange();
  }
  function stage(field: OrdinaryNumberField, raw: string): void {
    const previous = staged[field.key];
    const cell = ordinaryNumberValue(field, raw, original?.[field.key] ?? null);
    staged = { ...staged, [field.key]: cell };
    rememberOrdinarySession(draft, original, staged);
    const changed = cell.error !== null || !!previous?.error || cell.value !== draft[field.key];
    if (cell.error === null) draft[field.key] = cell.value;
    if (changed) onchange();
  }
</script>

{#if editingEnabled && validation}<p role="alert">{validation}</p>{/if}
{@render children(editingEnabled ? { columns: fields.map(field => field.column), input } : undefined)}
{#snippet input(control: PaperControl, position: string)}
  {@const field = ordinaryField(control.column)}
  {#if field && control.type === 'TextBox'}
    {#if field.kind === 'memo'}
      <textarea class="ordinary-control" id={`header-${field.key}`} data-column={field.column}
        data-source-control={control.controlName} style={position} aria-label={field.label} rows="4"
        value={draft[field.key] ?? ''} disabled={disabled || capabilities[field.key] !== true || !control.enabled || control.locked}
        aria-invalid={ordinaryTextError(field, draft[field.key], original?.[field.key] ?? null) !== null}
        oninput={event => setText(field, event.currentTarget.value)}
        onkeydown={event => {
          if (field.key === 'vegNotes' && event.key === 'Tab' && onVegNotesTab) {
            event.preventDefault(); onVegNotesTab();
          }
        }}></textarea>
    {:else if field.kind === 'text'}
      <input class="ordinary-control" id={`header-${field.key}`} data-column={field.column}
        data-source-control={control.controlName} style={position} aria-label={field.label}
        value={draft[field.key] ?? ''} disabled={disabled || capabilities[field.key] !== true || !control.enabled || control.locked}
        aria-invalid={ordinaryTextError(field, draft[field.key], original?.[field.key] ?? null) !== null}
        oninput={event => setText(field, event.currentTarget.value)} />
    {:else}
      <input class="ordinary-control" id={`header-${field.key}`} data-column={field.column}
        data-source-control={control.controlName} style={position} type="text" inputmode="decimal" aria-label={field.label}
        value={staged[field.key]?.raw ?? String(draft[field.key] ?? '')}
        disabled={disabled || capabilities[field.key] !== true || !control.enabled || control.locked}
        aria-invalid={staged[field.key]?.error ? 'true' : undefined}
        oninput={event => stage(field, event.currentTarget.value)} />
    {/if}
  {/if}
{/snippet}
<style>
  .ordinary-control { width: 100%; min-width: 0; min-height: 40px; border: 1px solid #b7cfc0; border-radius: 4px; padding: .5rem .65rem; background: white; font: inherit; line-height: 1.4; }
  .ordinary-control:disabled { color: #78716c; background: #f5f5f4; }
  .ordinary-control:focus { outline: 2px solid #047857; outline-offset: 1px; }
  .ordinary-control[aria-invalid="true"] { border-color: #b91c1c; }
  p { color: #b91c1c; font-size: .75rem; }
</style>
