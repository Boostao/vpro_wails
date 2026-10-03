<script lang="ts">
  import type { Snippet } from 'svelte';
  import { accessCaption, paperPage, type PaperControl, type VegetationMode } from './paperLayout';
  import { controlLabel, presentationGroups, wideControl } from './formPresentation';
  type Editor = { columns: readonly string[]; input: Snippet<[PaperControl, string]> };
  let { name, embedded, values = new Map<string, unknown>(), vegetationMode = 'initial', onHeightToggle, onSpeciesCheck, speciesCheckDisabled = true, onFindPlot, findPlotDisabled = true, editor, editors = [] }: {
    name: string; embedded: Snippet<[PaperControl]>; values?: Map<string, unknown>;
    vegetationMode?: VegetationMode; onHeightToggle?: () => void;
    onSpeciesCheck?: () => void; speciesCheckDisabled?: boolean;
    onFindPlot?: () => void; findPlotDisabled?: boolean;
    editor?: Editor; editors?: readonly Editor[];
  } = $props();
  const activeEditors = $derived.by(() => {
    const slots = editor ? [editor, ...editors] : editors;
    const columns = new Set<string>();
    for (const slot of slots) for (const column of slot.columns) {
      if (columns.has(column)) throw new Error(`Multiple source editors own ${column}.`);
      columns.add(column);
    }
    return slots;
  });
  const page = $derived(paperPage(name, vegetationMode));
  const sections = $derived(presentationGroups(name, page.controls));
</script>

<p class="form-status">Grey fields and actions remain unavailable. Related fields reflow without changing stored values.</p>
<div class="responsive-form" data-source-page={name}>
  {#each sections as section (section.title)}
    <fieldset class="form-group">
      <legend>{section.title}</legend>
      <div class="field-grid">
        {#each section.controls as control (control.controlId)}
          {@const column = control.column}
          {@const fieldEditor = column ? activeEditors.find(slot => slot.columns.includes(column)) : undefined}
          {@const stored = column ? values.get(column.toLowerCase()) : undefined}
          {#if control.type === 'Subform'}
            <div class="form-field full-width embedded" data-source-control={control.controlName}>
              {@render embedded(control)}
            </div>
          {:else if control.type === 'CommandButton' || control.type === 'ToggleButton'}
            <div class="form-field action-field">
              {#if control.controlName === 'btnCoverAndHeight' && onHeightToggle}
                <button type="button" data-source-control={control.controlName} aria-pressed={vegetationMode === 'height'} onclick={onHeightToggle}>{accessCaption(control.caption)}</button>
              {:else if control.controlName === 'btnCheckSppCodes' && onSpeciesCheck}
                <button type="button" data-source-control={control.controlName} disabled={speciesCheckDisabled} onclick={onSpeciesCheck}>{controlLabel(control)}</button>
              {:else if control.controlName === 'btnFindPlot' && onFindPlot}
                <button type="button" data-source-control={control.controlName} disabled={findPlotDisabled} onclick={onFindPlot}>{controlLabel(control)}</button>
              {:else}
                <button type="button" disabled title="Source workflow not migrated" data-source-control={control.controlName}>{controlLabel(control)}</button>
              {/if}
            </div>
          {:else}
            <label class="form-field" class:full-width={wideControl(control)}>
              <span class="field-label">{controlLabel(control)}</span>
              {#if fieldEditor}
                {@render fieldEditor.input(control, 'box-sizing:border-box;width:100%;min-width:0;min-height:40px;font:inherit;')}
              {:else if control.type === 'CheckBox' || control.type === 'OptionButton'}
                <input data-column={column} data-source-control={control.controlName} type={control.type === 'OptionButton' ? 'radio' : 'checkbox'} checked={stored === true} disabled aria-label={controlLabel(control)} />
              {:else if control.type === 'TextBox' || control.type === 'ComboBox'}
                {#if wideControl(control)}
                  <textarea data-column={column} data-source-control={control.controlName} disabled value={typeof stored === 'string' ? stored : ''} placeholder={stored !== undefined ? '' : 'Pending'} aria-label={controlLabel(control)} rows="3"></textarea>
                {:else}
                  <input data-column={column} data-source-control={control.controlName} disabled value={typeof stored === 'string' || typeof stored === 'number' ? stored : ''} placeholder={stored !==undefined ? '' : 'Pending'} aria-label={controlLabel(control)} />
                {/if}
              {/if}
            </label>
          {/if}
        {/each}
        {#if name === 'Veg Other'}
          <details class="form-group">
            <summary>Column legend</summary>
            <ul>
              {#each page.controls.filter(control => control.type === 'Label') as control (control.controlId)}
                <li>{accessCaption(control.caption)}</li>
              {/each}
            </ul>
          </details>
        {/if}
      </div>
    </fieldset>
  {/each}
</div>
