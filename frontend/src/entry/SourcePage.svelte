<script lang="ts">
  import type { Snippet } from 'svelte';
  import { accessCaption, paperPage, type PaperControl, type VegetationMode } from './paperLayout';
  import { controlLabel, presentationGroups, wideControl } from './formPresentation';
  type Editor = { columns: readonly string[]; controls?: readonly string[]; input: Snippet<[PaperControl, string]>; labelled?: boolean };
  let { name, sourcePage, embedded, values = new Map<string, unknown>(), vegetationMode = 'initial', onHeightToggle, heightToggleDisabled = false, onSpeciesCheck, speciesCheckDisabled = true, onFindPlot, findPlotDisabled = true, editor, editors = [] }: {
    name: string; embedded: Snippet<[PaperControl]>; values?: Map<string, unknown>;
    sourcePage?: ReturnType<typeof paperPage>;
    vegetationMode?: VegetationMode; onHeightToggle?: () => void; heightToggleDisabled?: boolean;
    onSpeciesCheck?: () => void; speciesCheckDisabled?: boolean;
    onFindPlot?: () => void; findPlotDisabled?: boolean;
    editor?: Editor; editors?: readonly Editor[];
  } = $props();
  const page = $derived(sourcePage ?? paperPage(name, vegetationMode));
  const activeEditors = $derived.by(() => {
    const slots = editor ? [editor, ...editors] : editors;
    const columns = new Set<string>();
    const controls = new Set<string>();
    for (const slot of slots) {
      for (const column of slot.columns) {
        if (columns.has(column)) throw new Error(`Multiple source editors own ${column}.`);
        columns.add(column);
      }
      for (const control of slot.controls ?? []) {
        if (controls.has(control)) throw new Error(`Multiple source editors own control ${control}.`);
        const source = page.controls.find(member => member.controlId === control);
        if (source?.column) throw new Error(`Bound source control ${control} must be owned by its column.`);
        if (source && ['CommandButton', 'ToggleButton', 'Subform'].includes(source.type)) {
          throw new Error(`Source action ${control} cannot be owned by a field editor.`);
        }
        controls.add(control);
      }
    }
    return slots;
  });
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
          {@const label = controlLabel(control, Boolean(sourcePage))}
          {@const fieldEditor = column ? activeEditors.find(slot => slot.columns.includes(column)) :
            activeEditors.find(slot => slot.controls?.includes(control.controlId))}
          {@const stored = column ? values.get(column.toLowerCase()) : undefined}
          {#if control.type === 'Subform'}
            <div class="form-field full-width embedded" data-source-control={control.controlName}>
              {@render embedded(control)}
            </div>
          {:else if control.type === 'CommandButton' || control.type === 'ToggleButton'}
            <div class="form-field action-field">
              {#if control.controlName === 'btnCoverAndHeight' && onHeightToggle}
                <button type="button" data-source-control={control.controlName} disabled={heightToggleDisabled} aria-pressed={vegetationMode === 'height'} onclick={onHeightToggle}>{accessCaption(control.caption)}</button>
              {:else if control.controlName === 'btnCheckSppCodes' && onSpeciesCheck}
                <button type="button" data-source-control={control.controlName} disabled={speciesCheckDisabled} onclick={onSpeciesCheck}>{label}</button>
              {:else if control.controlName === 'btnFindPlot' && onFindPlot}
                <button type="button" data-source-control={control.controlName} disabled={findPlotDisabled} onclick={onFindPlot}>{label}</button>
              {:else}
                <button type="button" disabled title="Source workflow not migrated" data-source-control={control.controlName}>{label}</button>
              {/if}
            </div>
          {:else if fieldEditor?.labelled}
            <div class="form-field" class:full-width={wideControl(control)}>
              {@render fieldEditor.input(control, 'box-sizing:border-box;width:100%;min-width:0;min-height:40px;font:inherit;')}
            </div>
          {:else}
            <label class="form-field" class:full-width={wideControl(control)}>
              <span class="field-label">{label}</span>
              {#if fieldEditor}
                {@render fieldEditor.input(control, 'box-sizing:border-box;width:100%;min-width:0;min-height:40px;font:inherit;')}
              {:else if control.type === 'CheckBox' || control.type === 'OptionButton'}
                <input data-column={column} data-source-control={control.controlName} type={control.type === 'OptionButton' ? 'radio' : 'checkbox'}
                  checked={stored === true || Boolean(sourcePage) && control.type === 'CheckBox' && (stored === -1 || stored === '-1')}
                  disabled aria-label={label} />
                {#if sourcePage && control.type === 'CheckBox' && stored != null && stored !== true && stored !== false
                  && stored !== 0 && stored !== '0' && stored !== -1 && stored !== '-1'}
                  <span class="text-xs text-stone-600">Historical {String(stored)}</span>
                {/if}
              {:else if control.type === 'TextBox' || control.type === 'ComboBox' || control.type === 'OptionGroup'}
                {#if wideControl(control)}
                  <textarea data-column={column} data-source-control={control.controlName} disabled value={typeof stored === 'string' ? stored : ''} placeholder={stored !== undefined ? '' : 'Pending'} aria-label={label} rows="3"></textarea>
                {:else}
                  <input data-column={column} data-source-control={control.controlName} disabled value={typeof stored === 'string' || typeof stored === 'number' ? stored : ''} placeholder={stored !==undefined ? '' : 'Pending'} aria-label={label} />
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
