<script lang="ts">
  import type { Snippet } from 'svelte';
  import SourcePage from './SourcePage.svelte';
  import type { CoordinateDisplayMode, PaperControl } from './paperLayout';
  import type { TwoPageParentForm } from './twoPageParentSession';
  import { twoPageEntryProjection, type TwoPageEntryField, type TwoPageEntryScope } from './twoPageEntryProjection';
  import { coordinateControls, coordinateOptions } from './coordinateEditor';

  type Projection = ReturnType<typeof twoPageEntryProjection>;
  type Child = Projection['children'][number];
  type Editor = { scope: TwoPageEntryScope; columns: readonly string[]; controls?: readonly string[]; input: Snippet<[PaperControl, string]>; labelled?: boolean };
  let { form, coordinateMode = 'dd', values = new Map<string, unknown>(), editors = [], identity, embedded: childContent }: {
    form: TwoPageParentForm; coordinateMode?: CoordinateDisplayMode; values?: Map<string, unknown>;
    editors?: readonly Editor[]; identity: Snippet<[TwoPageEntryField]>; embedded: Snippet<[Child, PaperControl]>;
  } = $props();
  const projection = $derived(twoPageEntryProjection(form, coordinateMode));
  const slots = $derived.by(() => {
    const owned = new Set<string>();
    const controls = new Set<string>();
    for (const editor of editors) {
      for (const column of editor.columns) {
        const field = projection.fields.find(field => field.column === column);
        if (!field || field.scope !== editor.scope || field.page === null) {
          throw new Error(`Two-page ${column} has no matching ${editor.scope} page editor owner.`);
        }
        if (owned.has(column)) throw new Error(`Multiple two-page editors own ${column}.`);
        owned.add(column);
      }
      for (const id of editor.controls ?? []) {
        const control = projection.pages.flatMap(page => page.controls).find(control => control.controlId === id);
        if (editor.scope !== 'xl' || !control || control.column ||
            !Object.hasOwn(coordinateControls, control.controlName ?? '') && !Object.hasOwn(coordinateOptions, control.controlName ?? '')) {
          throw new Error(`Two-page control ${id} has no matching unbound coordinate editor owner.`);
        }
        if (controls.has(id)) throw new Error(`Multiple two-page editors own control ${id}.`);
        controls.add(id);
      }
    }
    return editors;
  });
  function childLink(control: PaperControl): Child {
    const child = projection.children.find(child => child.controlId === control.controlId);
    if (!child) throw new Error(`Two-page child ${control.controlName} has no exact source link.`);
    return child;
  }
</script>

<section class="space-y-4" aria-label={`${form} source entry layout`} data-two-page-entry={form}>
  <div class="form-group" data-two-page-header>
    {@render identity(projection.header[0])}
  </div>
  {#each projection.pages as page (page.name)}
    <section aria-label={page.name} class="space-y-2">
      <h3 class="font-semibold">{page.name}</h3>
      <SourcePage name={page.name} sourcePage={page} {values} editors={slots}>
        {#snippet embedded(control)}
          {@const child = childLink(control)}
          {#if child.availability === 'unavailable'}
            <div class="rounded border p-3 text-sm text-stone-600" data-source-form={child.form}
              aria-label="Pictures unavailable">
              Pictures: source storage is mapped; the picture workflow remains unavailable.
            </div>
          {:else}
            {@render childContent(child, control)}
          {/if}
        {/snippet}
      </SourcePage>
    </section>
  {/each}
</section>
