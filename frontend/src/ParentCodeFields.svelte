<script lang="ts">
  import { onDestroy, untrack, type Snippet } from 'svelte';
  import { ParentCodeService, type FS882Header } from '../bindings/github.com/boostao/vpro-wails';
  import { QualityLookup } from './qualityEditor';
  import type { PaperControl } from './paperLayout';
  import FieldGuidance from './FieldGuidance.svelte';
  import { createParentCodeEditor, parentCodeField, type ParentCodes, type ParentCodeKey,
    type ParentCodeScope, type ParentCodeViews } from './parentCodeEditor';

  let { draft = $bindable(), original, scope, capabilities, disabled, onchange, onvalidation, onbusy, children }: {
    draft: FS882Header; original: ParentCodes | null; scope: ParentCodeScope;
    capabilities: Record<string, boolean | undefined>; disabled: boolean;
    onchange: () => void; onvalidation: (key: string, message: string | null) => void;
    onbusy?: (busy: boolean) => void;
    children: Snippet<[{ columns: readonly string[]; input: Snippet<[PaperControl, string]> } | undefined]>;
  } = $props();
  const editingEnabled = import.meta.env.VITE_PARENT_CODES_EDITING !== 'false';
  const editor = createParentCodeEditor(untrack(() => scope));
  const validationKey = `parentCodes-${untrack(() => scope)}`;
  let views = $state<ParentCodeViews>({});
  const lookups = editor.lists.map(list => new QualityLookup(async (force) => {
    if (force) await ParentCodeService.ReloadCatalogue();
    const rows = await ParentCodeService.ListChoices(list);
    if (rows === null) throw new Error(`${list} service did not return reference rows.`);
    return rows;
  }, next => { views = { ...views, [list]: next }; }, list));
  views = Object.fromEntries(editor.lists.map((list, index) => [list, lookups[index].snapshot()]));
  let acknowledged = $state(false);
  const warnings = $derived(editor.warnings(draft, original, views));
  const validation = $derived(editor.validation(draft, original, views, acknowledged));
  $effect(() => {
    const identity = draft;
    if (editingEnabled && identity) untrack(() => { for (const lookup of lookups) void lookup.refresh(); });
  });
  $effect(() => { acknowledged = editor.acknowledged(draft, draft); });
  $effect(() => { onbusy?.(editingEnabled && editor.busy(draft, original, views)); });
  $effect(() => { const message = editingEnabled ? validation : null; untrack(() => onvalidation(validationKey, message)); });
  onDestroy(() => {
    for (const lookup of lookups) lookup.dispose();
    onbusy?.(false);
    if (editingEnabled) onvalidation(validationKey, editor.validation(draft, original,
      Object.fromEntries(Object.entries(views).map(([list, view]) =>
        [list, view.busy ? { ...view, busy: false, ready: false } : view])),
      editor.acknowledged(draft, draft)));
  });
  function setCode(key: ParentCodeKey, raw: string) {
    const value = raw === '' ? null : raw;
    if (draft[key] === value) return;
    draft[key] = value;
    acknowledged = editor.acknowledged(draft, draft);
    onchange();
    onvalidation(validationKey, editor.validation(draft, original, views, acknowledged));
    onbusy?.(editor.busy(draft, original, views));
  }
  function acknowledge(accepted: boolean) {
    editor.rememberAcknowledgement(draft, draft, accepted);
    acknowledged = accepted;
    onvalidation(validationKey, editor.validation(draft, original, views, accepted));
  }
</script>

{#if editingEnabled}
  <div class="parent-code-toolbar" data-scope={scope} aria-label="Parent code safety controls">
    {#each editor.lists as list, index}
      {#if views[list]?.busy}<p role="status">Refreshing {list} choices...</p>{/if}
      {#if views[list]?.error}
        <p role="alert">{views[list]?.error}</p>
        <button type="button" disabled={disabled || views[list]?.busy} onclick={() => void lookups[index].refresh(true)}>Retry {list} choices</button>
      {/if}
    {/each}
    {#if validation}<p role="alert">{validation}</p>{/if}
    {#if warnings.length}
      <ul>{#each warnings as warning}<li>{warning}</li>{/each}</ul>
      {#if !editor.busy(draft, original, views) && editor.fields.every(({ key }) => editor.valueError(key, draft[key], original?.[key] ?? null) === null)}
        <label><input type="checkbox" checked={acknowledged} disabled={disabled}
          onchange={event => acknowledge(event.currentTarget.checked)} />
          Keep these parent codes exactly as entered; I have reviewed the unmatched or unchecked values.
        </label>
      {/if}
    {/if}
  </div>
{/if}
{@render children(editingEnabled ? { columns: editor.fields.map(field => field.column), input } : undefined)}
{#if editingEnabled}
  <FieldGuidance title="Parent code guidance and definitions">
    <p>Independent nullable codes. Select full Items explicitly; raw casing is preserved without
      completion, truncation or partner autofill. Each field retains its own storage length.</p>
    {#each editor.fields as field}
      <details>
        <summary>{field.label}: choose a full Item (maximum {field.maximum} UTF-16 characters)</summary>
        {#each editor.groups(views[field.list]?.choices ?? [], field.list) as group (group.code)}
          <button type="button" disabled={disabled || views[field.list]?.busy || capabilities[field.key] !== true}
            data-choice-for={field.key} data-choice-code={group.code}
            onclick={() => setCode(field.key, group.code)}>{group.code}: {group.records[0].description ?? 'NULL'}</button>
        {/each}
      </details>
    {/each}
    {#each editor.lists as list}
      <details>
        <summary>All frozen {list} reference rows ({views[list]?.choices.length ?? 0}); NULL and empty Items are metadata</summary>
        {#each views[list]?.choices ?? [] as definition (definition.rowId)}
          <details data-reference-row={definition.rowId} data-reference-list={definition.listName}>
            <summary>{definition.code === null ? 'NULL' : definition.code === '' ? '""' : definition.code}: {definition.description ?? 'NULL'}</summary>
            <dl>{#each Object.entries(definition) as [name, value]}
              <dt>{name}</dt><dd data-property={name} data-value={JSON.stringify(value)}
                data-kind={value === null ? 'null' : typeof value} data-empty={value === '' ? 'true' : undefined}
                data-negative-zero={typeof value === 'number' && Object.is(value, -0) ? 'true' : undefined}>
                {value === null ? 'NULL' : value === '' ? '""' : String(value)}
              </dd>
            {/each}</dl>
          </details>
        {/each}
      </details>
    {/each}
  </FieldGuidance>
{/if}
{#snippet input(control: PaperControl, position: string)}
  {@const field = parentCodeField(control.column)}
  {#if field && control.type === 'ComboBox'}
    <input class="parent-code-control" id={`header-${field.key}`} data-column={field.column}
      data-source-control={control.controlName} style={position} aria-label={field.label}
      value={draft[field.key] ?? ''} disabled={disabled || capabilities[field.key] !== true}
      placeholder={capabilities[field.key] === true ? '' : 'Pending'} autocomplete="off"
      aria-invalid={editor.valueError(field.key, draft[field.key], original?.[field.key] ?? null) !== null}
      oninput={event => setCode(field.key, event.currentTarget.value)} />
  {/if}
{/snippet}
<style>
  .parent-code-toolbar { display: grid; gap: .3rem; margin-bottom: .5rem; padding: .5rem; border: 1px solid #d6d3d1; font-size: .75rem; }
  p { margin: 0; }
  .parent-code-control { border: 1px solid #b7cfc0; border-radius: 4px; padding: .5rem .65rem; background: white; color: #065f46; font: inherit; line-height: 1.4; }
  .parent-code-control:disabled { color: #78716c; background: #f5f5f4; }
  .parent-code-control[aria-invalid="true"] { border-color: #b91c1c; }
</style>
