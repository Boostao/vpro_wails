<script lang="ts">
  import { paperChild } from './paperLayout';
  import { controlLabel } from './formPresentation';
  import { heightField, type HeightDrafts } from './heightEditor';
  import { otherField, type OtherDrafts, type OtherValue } from './otherEditor';
  import { soilField, soilCell, type SoilDrafts, type SoilKind } from './soilChildEditor';
  import { vegetationAttributeField, type VegetationAttributeDrafts } from './vegetationAttributeEditor';
  import type { CollectedDrafts } from './collectedEditor';
  import type { SpeciesDrafts, SpeciesLists } from './vegetationSpeciesEditor';
  import type { SoilSuggestion } from '../bindings/github.com/boostao/vpro-wails';
  type Row = { id: number; values: Record<string, string | number | boolean | null | undefined> };
  let { name, rows, disabled = true, deleteDisabled = false, revision = 0, onedit, ondelete, onstage, drafts = {}, onotherstage, otherDrafts = {}, onsoilstage, soilDrafts = {}, soilSuggestions = [], onattributestage, attributeDrafts = {}, attributeSuggestions = [], oncollectedstage, collectedDrafts = {}, collectedDisabled = true, onspeciesstage, speciesDrafts = {}, speciesLists = {}, speciesDisabled = true }: {
    name: string; rows: Row[]; disabled?: boolean; revision?: number;
    onedit?: (name: string, id: number, column: string, value: string) => Promise<void>;
    ondelete?: (name: string, id: number) => Promise<void>;
    onstage?: (id: number, column: string, value: string) => void;
    drafts?: HeightDrafts;
    onotherstage?: (id: number, column: string, value: OtherValue) => void;
    otherDrafts?: OtherDrafts;
    onsoilstage?: (kind: SoilKind, id: number, column: string, value: string) => void;
    soilDrafts?: SoilDrafts;
    soilSuggestions?: SoilSuggestion[];
    onattributestage?: (id: number, column: string, value: string) => void;
    attributeDrafts?: VegetationAttributeDrafts;
    attributeSuggestions?: SoilSuggestion[];
    oncollectedstage?: (id: number) => void;
    collectedDrafts?: CollectedDrafts;
    collectedDisabled?: boolean;
    onspeciesstage?: (form: string, id: number, raw: string) => void;
    speciesDrafts?: SpeciesDrafts;
    speciesLists?: SpeciesLists;
    speciesDisabled?: boolean;
    deleteDisabled?: boolean;
  } = $props();
  const source = $derived(paperChild(name));
  const soilKind = $derived(name === 'SoilHumusXL' ? 'Humus' : name === 'SoilMineralXL' ? 'Mineral' : null);
  const columns = $derived(source.controls.filter(control => !['Label', 'Rectangle', 'OptionGroup'].includes(control.type))
    .sort((a, b) => a.tabOrder - b.tabOrder));
  const numeric = new Set(['cover1', 'cover2', 'cover3', 'totala', 'cover4', 'cover5', 'totalb', 'cover6', 'cover7', 'cover8', 'cover9', 'upperdepth', 'lowerdepth', 'humusformph']);
  const editable = new Set([...numeric, 'species', 'collected', 'horizon', 'comment', 'comments', 'texture', 'colour', 'dataname', 'dataitem']);
</script>

<div class="source-child" data-source-form={name} aria-label={`${name} records`}>
  <table>
    <caption class="sr-only">{name} records; unavailable columns remain read-only</caption>
    <thead>
      <tr>
        {#each columns as control (control.controlId)}
          <th scope="col">{controlLabel(control)}</th>
        {/each}
        {#if ondelete}<th scope="col">Actions</th>{/if}
      </tr>
    </thead>
    <tbody>
    {#each rows as row, index (`${row.id}/${index}/${revision}`)}
      <tr data-record-id={row.id}>
        {#each columns as control (control.controlId)}
          {@const value = control.column ? row.values[control.column.toLowerCase()] : undefined}
          {@const mapped = control.column !== undefined && Object.hasOwn(row.values, control.column.toLowerCase())}
          {@const stagedField = control.column ? heightField(control.column) : undefined}
          {@const staged = stagedField ? drafts[String(row.id)]?.[stagedField] : undefined}
          {@const other = name === 'SubOtherXL' && control.column ? otherField(control.column) : undefined}
          {@const otherStaged = other ? otherDrafts[String(row.id)]?.[other.key] : undefined}
          {@const soil = soilKind && control.column ? soilField(soilKind, control.column) : undefined}
          {@const soilStaged = soilKind && soil ? soilCell(soilDrafts, soilKind, row.id, soil.key) : undefined}
          {@const attribute = name === 'USysVegOtherXL' && control.column ? vegetationAttributeField(control.column) : undefined}
          {@const attributeStaged = attribute ? attributeDrafts[String(row.id)]?.[attribute.key] : undefined}
          <td>
          {#if mapped && onspeciesstage && control.column?.toLowerCase() === 'species'}
            {@const species = speciesDrafts[String(row.id)]}
            {@const listId = `species-${name}`}
            <input class="source-cell" type="text" value={species ? species.raw : value ?? ''} list={listId}
              disabled={speciesDisabled || control.locked || !control.enabled} aria-invalid={species?.error != null}
              aria-label={`${controlLabel(control)}, row ${row.id}`} data-column={control.column}
              title={species?.error ?? 'Unsubmitted draft: select an exact list code, then Save or Cancel species drafts'}
              oninput={event => onspeciesstage?.(name, row.id, event.currentTarget.value)} />
          {:else if mapped && oncollectedstage && control.column?.toLowerCase() === 'collected'}
            {@const collected = collectedDrafts[String(row.id)]}
            <button class="source-cell" type="button" data-column={control.column}
              disabled={collectedDisabled || control.locked || !control.enabled}
              aria-label={`Cycle Collected, row ${row.id}`}
              title="Click to cycle NULL -> C -> V -> NULL. Other stored values are unchanged."
              onclick={() => oncollectedstage?.(row.id)}>{(collected ? collected.value : value) ?? 'NULL'}</button>
          {:else if mapped && onattributestage && attribute && control.column}
            {@const listId = `veg-attribute-${row.id}-${attribute.key}`}
            <input class="source-cell" type="text" inputmode="numeric"
              value={attributeStaged ? attributeStaged.raw : value ?? ''} list={attribute.group ? listId : undefined}
              disabled={disabled || control.locked || !control.enabled} aria-invalid={attributeStaged?.error != null}
              aria-label={`${controlLabel(control)}, row ${row.id}`} data-column={control.column}
              title={attributeStaged?.error ?? 'Unsubmitted draft: use Save vegetation attributes or Cancel vegetation attributes'}
              oninput={event => onattributestage?.(row.id, control.column ?? '', event.currentTarget.value)} />
            {#if attribute.group}
              <datalist id={listId}>
                {#each attributeSuggestions.filter(entry => entry.listName === attribute.group && entry.item !== null) as entry, index (index)}
                  <option value={entry.item ?? ''}>{entry.itemDescription ?? ''}</option>
                {/each}
              </datalist>
            {/if}
          {:else if mapped && onsoilstage && soilKind && soil && control.column}
            {@const listId = `soil-${name}-${row.id}-${soil.key}`}
            {#if soil.kind === 'memo'}
              <textarea class="source-cell" rows="2" value={soilStaged ? soilStaged.raw : String(value ?? '')}
                disabled={disabled || control.locked || !control.enabled} aria-invalid={soilStaged?.error != null}
                aria-label={`${controlLabel(control)}, row ${row.id}`} data-column={control.column}
                title={soilStaged?.error ?? 'Unsubmitted draft: use Save soil drafts or Cancel soil drafts'}
                oninput={event => onsoilstage?.(soilKind, row.id, control.column ?? '', event.currentTarget.value)}></textarea>
            {:else}
              <input class="source-cell" type="text" inputmode={soil.kind === 'single' ? 'decimal' : soil.kind === 'integer' ? 'numeric' : undefined}
                value={soilStaged ? soilStaged.raw : value ?? ''} list={soil.group ? listId : undefined}
                disabled={disabled || control.locked || !control.enabled} aria-invalid={soilStaged?.error != null}
                aria-label={`${controlLabel(control)}, row ${row.id}`} data-column={control.column}
                title={soilStaged?.error ?? 'Unsubmitted draft: use Save soil drafts or Cancel soil drafts'}
                oninput={event => onsoilstage?.(soilKind, row.id, control.column ?? '', event.currentTarget.value)} />
              {#if soil.group}
                <datalist id={listId}>
                  {#each soilSuggestions.filter(entry => entry.listName === soil.group && entry.item !== null) as entry, index (index)}
                    <option value={entry.item ?? ''}>{entry.itemDescription ?? ''}</option>
                  {/each}
                </datalist>
              {/if}
            {/if}
          {:else if mapped && onotherstage && other && control.column}
            {#if other.kind === 'flag'}
              {@const flagValue = otherStaged ? otherStaged.value : value}
              <input class="source-cell" type="checkbox" checked={flagValue === true} indeterminate={flagValue == null}
                disabled={disabled || control.locked || !control.enabled}
                aria-label={`${controlLabel(control)}, row ${row.id}`} data-column={control.column}
                onchange={event => onotherstage?.(row.id, control.column ?? '', event.currentTarget.checked)} />
              <button type="button" disabled={disabled || control.locked || !control.enabled || flagValue == null}
                aria-label={`Clear ${controlLabel(control)} to NULL, row ${row.id}`}
                onclick={() => onotherstage?.(row.id, control.column ?? '', null)}>Clear to NULL</button>
            {:else}
              <input class="source-cell" type="text" value={otherStaged ? otherStaged.raw ?? '' : value ?? ''}
                disabled={disabled || control.locked || !control.enabled} aria-invalid={otherStaged?.error != null}
                aria-label={`${controlLabel(control)}, row ${row.id}`} data-column={control.column}
                title={otherStaged?.error ?? 'Unsubmitted draft: use Save Other drafts or Cancel Other drafts'}
                oninput={event => onotherstage?.(row.id, control.column ?? '', event.currentTarget.value)} />
            {/if}
          {:else if mapped && onstage && stagedField && control.column}
            <input class="source-cell" type="text" inputmode="decimal"
              value={staged?.raw ?? value ?? ''} disabled={disabled || control.locked || !control.enabled}
              aria-label={`${controlLabel(control)}, row ${row.id}`} aria-invalid={staged?.error != null} data-column={control.column}
              title={staged?.error ?? 'Unsubmitted draft: use Save height drafts or Cancel height drafts'}
              oninput={event => onstage?.(row.id, control.column ?? '', event.currentTarget.value)} />
          {:else if mapped && name.startsWith('SubVeg') && control.column?.toLowerCase() === 'species'}
            <input class="source-cell" type="text" value={value ?? ''} disabled data-column={control.column}
              aria-label={`${controlLabel(control)}, row ${row.id}`} title="Species selection is unavailable in this build" />
          {:else if mapped && onedit && control.column && editable.has(control.column.toLowerCase())}
            <input class="source-cell" type={numeric.has(control.column.toLowerCase()) ? 'number' : 'text'} step="any"
              value={value ?? ''} disabled={disabled || control.locked || !control.enabled}
              aria-label={`${controlLabel(control)}, row ${row.id}`} data-column={control.column} title={control.caption || control.column}
              onchange={(event) => {
                if (!event.currentTarget.validity.valid) { event.currentTarget.reportValidity(); return; }
                void onedit?.(name, row.id, control.column ?? '', event.currentTarget.value);
              }} />
          {:else if mapped && control.type === 'CheckBox'}
            <input class="source-cell" type="checkbox" data-column={control.column} disabled checked={value === true} indeterminate={value == null}
              aria-label={control.column || control.caption} />
          {:else}
          <span class:pending={!mapped} class="source-cell" data-column={control.column}
            title={control.caption || control.column || control.controlName}
            aria-label={control.caption || control.column || control.controlName}>
            {mapped ? value ?? '' : 'Pending'}
          </span>
          {/if}
          </td>
        {/each}
        {#if ondelete}
          <td><button class="delete" type="button" disabled={disabled || deleteDisabled}
            aria-label={`Delete ${name} row ${row.id}`} onclick={() => void ondelete?.(name, row.id)}>Delete</button></td>
        {/if}
      </tr>
    {:else}
      <tr><td class="empty" colspan={columns.length + (ondelete ? 1 : 0)}>No records in this source grid.</td></tr>
    {/each}
    </tbody>
  </table>
  {#if onspeciesstage}
    <datalist id={`species-${name}`}>
      {#each speciesLists[name] ?? [] as option, index (index)}
        {#if option.code !== null}
          <option value={option.code}>{option.scientificName ?? 'NULL'} | {option.englishName ?? 'NULL'} | Lifeform {option.lifeform ?? 'NULL'} | {option.codeType ?? 'NULL'}</option>
        {/if}
      {/each}
    </datalist>
  {/if}
</div>

<style>
  .source-child { width: 100%; max-width: 100%; min-width: 0; overflow-x: auto; background: white; border: 1px solid #d1dcd4; border-radius: 4px; }
  table { width: max-content; min-width: 100%; border-collapse: collapse; font-size: .875rem; }
  th { background: #e9efeb; color: #365c48; text-align: left; font-size: .875rem; font-weight: 600; white-space: normal; }
  th, td { padding: .5rem; min-width: 7rem; max-width: 18rem; border-bottom: 1px solid #d1dcd4; }
  td { overflow: visible; white-space: normal; }
  .source-cell { display: block; box-sizing: border-box; width: 100%; min-height: 40px; border: 1px solid #b7cfc0; border-radius: 4px; padding: .5rem; font: inherit; line-height: 1.4; overflow-wrap: anywhere; }
  input[type="checkbox"] { width: 1.2rem; min-height: 1.2rem; }
  .pending { color: #78716c; background: #f5f5f4; }
  .delete { min-height: 40px; color: #b91c1c; padding: .5rem .75rem; border: 1px solid #d6d3d1; border-radius: 4px; }
  .empty { padding: 1rem; color: #78716c; }
</style>
