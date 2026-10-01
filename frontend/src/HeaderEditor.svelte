<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { FS882Header, ListItem } from '../bindings/github.com/boostao/vpro-wails';
  import { paperPage, type PaperControl } from './paperLayout';
  import { controlLabel, presentationGroups, wideControl } from './formPresentation';
  import CoordinateFields from './CoordinateFields.svelte';
  import BECFields from './BECFields.svelte';
  import WorkingUnitFields from './WorkingUnitFields.svelte';
  import QualityFields from './QualityFields.svelte';
  import SubstrateFields from './SubstrateFields.svelte';
  import SiteCodeFields from './SiteCodeFields.svelte';
  import { siteCodeKey, type SiteCodes } from './siteCodeEditor';
  import RegionCodeFields from './RegionCodeFields.svelte';
  import { regionCodeKey, type RegionCodes } from './regionCodeEditor';
  import { substrateKey, type SubstrateValues } from './substrateEditor';
  import { qualityKey, type QualityCodes } from './qualityEditor';
  import { becKey, type BECCodes } from './becEditor';
  import { isWorkingUnitControl, type WorkingUnitCodes, type WorkingUnitSession } from './workingUnitEditor';
  import { isCoordinateControl, type CoordinateDisplayMode } from './coordinateEditor';
  import { nullableParentField } from './ordinaryEditor';

  type TextKey = { [K in keyof FS882Header]: FS882Header[K] extends string | null ? K : never }[keyof FS882Header];
  type NullableTextKey = Exclude<TextKey, 'plotNumber'>;
  type NumberKey = { [K in keyof FS882Header]: FS882Header[K] extends number | null ? K : never }[keyof FS882Header];
  type Field = { key: NullableTextKey; label: string; kind: 'text' | 'notes'; list?: boolean } |
    { key: NumberKey; label: string; kind: 'number'; step?: string };

  let { draft = $bindable(), original, capabilities, disabled, existing, lists, onchange, onerror, onvalidation, onCoordinateBusyChange, onWorkingUnitBusyChange, onQualityBusyChange, onSiteCodeBusyChange, onRegionCodeBusyChange, workingUnitSession, masterAllowed = false, editor, additionalEditor }: {
    draft: FS882Header;
    original: (BECCodes & WorkingUnitCodes & QualityCodes & SubstrateValues & SiteCodes & RegionCodes) | null;
    capabilities: Record<string, boolean | undefined>;
    disabled: boolean;
    existing: boolean;
    lists: Partial<Record<NullableTextKey, ListItem[]>>;
    onchange: () => void;
    onerror: (message: string) => void;
    onvalidation: (key: NumberKey | 'bec' | 'workingUnit' | 'masterBEC' | 'quality' | 'substrate' | 'siteCodes' | 'regionCodes', message: string | null) => void;
    masterAllowed?: boolean;
    onCoordinateBusyChange?: (busy: boolean) => void;
    onWorkingUnitBusyChange?: (busy: boolean) => void;
    onQualityBusyChange?: (busy: boolean) => void;
    onSiteCodeBusyChange?: (busy: boolean) => void;
    onRegionCodeBusyChange?: (busy: boolean) => void;
    workingUnitSession: WorkingUnitSession;
    editor?: { columns: readonly string[]; input: Snippet<[PaperControl, string]> };
    additionalEditor?: { columns: readonly string[]; input: Snippet<[PaperControl, string]> };
  } = $props();

  let coordinateMode = $state<CoordinateDisplayMode>('dd');
  const page = $derived(paperPage('Site', 'initial', coordinateMode));
  const groups = $derived(presentationGroups('Site', page.controls));

  const sections: { title: string; fields: Field[] }[] = [
    { title: 'Identity and survey', fields: [
      { key: 'projectId', label: 'Project ID', kind: 'text' },
      { key: 'fieldNo', label: 'Field number', kind: 'text' },
      { key: 'surveyor', label: 'Site surveyor', kind: 'text' },
      { key: 'date', label: 'Survey date', kind: 'text' },
      { key: 'startDate', label: 'Start year', kind: 'number' }
    ] },
    { title: 'Location and coordinates', fields: [
      { key: 'generalLocation', label: 'Location', kind: 'text' },
      { key: 'mapSheet', label: 'NTS map sheet', kind: 'text' },
      { key: 'utmZone', label: 'UTM zone', kind: 'text' },
      { key: 'easting', label: 'UTM easting', kind: 'number', step: 'any' },
      { key: 'northing', label: 'UTM northing', kind: 'number', step: 'any' },
      { key: 'accuracy', label: 'Location accuracy', kind: 'number' }
    ] },
    { title: 'Classification', fields: [
      { key: 'plotRepresenting', label: 'Plot representing', kind: 'text' },
      { key: 'transition', label: 'Transition distribution', kind: 'text' },
      { key: 'mapUnit', label: 'Map unit', kind: 'text' },
      { key: 'moistureRegime', label: 'Moisture regime', kind: 'text', list: true },
      { key: 'nutrientRegime', label: 'Nutrient regime', kind: 'text', list: true }
    ] },
    { title: 'Stand information', fields: [
      { key: 'successional', label: 'Successional status', kind: 'text' },
      { key: 'structuralStage', label: 'Structural stage', kind: 'text' },
      { key: 'standAge', label: 'Stand age', kind: 'number' }
    ] },
    { title: 'Topography', fields: [
      { key: 'elevation', label: 'Elevation (m)', kind: 'number' },
      { key: 'slope', label: 'Slope gradient (%)', kind: 'number', step: 'any' },
      { key: 'aspect', label: 'Aspect (degrees)', kind: 'number' },
      { key: 'mesoSlopePos', label: 'Meso slope position', kind: 'text', list: true },
      { key: 'surfaceShape', label: 'Surface shape', kind: 'text', list: true },
      { key: 'microtopType', label: 'Surface topography type', kind: 'text' },
      { key: 'microtopSize', label: 'Surface topography size', kind: 'text' }
    ] },
    { title: 'Notes', fields: [
      { key: 'fieldNotes', label: 'Site notes', kind: 'notes' },
      { key: 'officeNotes', label: 'Office notes', kind: 'notes' }
    ] }
  ];

  const columns: Partial<Record<Exclude<keyof FS882Header, 'locked'>, string>> = {
    plotNumber: 'PlotNumber', projectId: 'ProjectID', surveyor: 'SiteSurveyor',
    date: 'Date', fieldNo: 'FieldNumber', generalLocation: 'Location', mapSheet: 'NtsMapSheet',
    utmZone: 'UTMZone', easting: 'UTMEasting', northing: 'UTMNorthing', accuracy: 'LocationAccuracy',
    plotRepresenting: 'PlotRepresenting', siteSeries: 'SiteSeries', transition: 'TransDistrib',
    mapUnit: 'MapUnit', moistureRegime: 'MoistureRegime', nutrientRegime: 'NutrientRegime',
    successional: 'SuccessionalStatus', structuralStage: 'StructuralStage', standAge: 'StandAge',
    elevation: 'Elevation', slope: 'SlopeGradient', aspect: 'Aspect', mesoSlopePos: 'MesoSlopePosition',
    surfaceShape: 'SurfaceShape', microtopType: 'SurfaceTopographyType',
    microtopSize: 'SurfaceTopographySize', fieldNotes: 'SiteNotes', officeNotes: 'OfficeNotes',
    startDate: 'StartDate'
  };
  const byColumn = new Map(sections.flatMap(section => section.fields).map(field => [columns[field.key], field]));
  const storedValues = $derived(new Map(Object.entries(draft).map(([key, value]) => [key.toLowerCase(), value])));
  const ordered = $derived(page.controls);
  function position(control: PaperControl) {
    if (['CheckBox', 'OptionButton'].includes(control.type)) return 'box-sizing:border-box;';
    return 'box-sizing:border-box;width:100%;min-width:0;min-height:40px;font:inherit;';
  }
  function inputId(control: PaperControl): string {
    if (isCoordinateControl(control.controlName)) return `header-coordinate-${control.controlName}`;
    if (isWorkingUnitControl(control.controlName)) {
      if (control.column === 'BECSiteUnit') return 'header-becSiteUnit';
      if (control.column === 'UserSiteUnit') return 'header-userSiteUnit';
      return `header-working-unit-${control.controlName}`;
    }
    const key = becKey(control.column) ?? qualityKey(control.column) ?? substrateKey(control.column) ??
      siteCodeKey(control.column) ?? regionCodeKey(control.column) ??
      (additionalEditor?.columns.includes(control.column ?? '') ? nullableParentField(control.column)?.key : undefined) ??
      byColumn.get(control.column ?? '')?.key;
    return key ? `header-${key}` : control.column === 'RealmClass' && editor?.columns.includes('RealmClass')
      ? 'header-realmClass' : control.column === 'PlotNumber' ? 'header-plotNumber' : `header-source-${control.controlId}`;
  }

  function setText(key: NullableTextKey, value: string) {
    draft[key] = value === '' ? null : value;
    onchange();
  }

  function setNumber(key: NumberKey, input: HTMLInputElement) {
    if (input.validity.badInput || input.validity.stepMismatch) {
      const message = `Enter a valid ${input.step === 'any' ? 'number' : 'integer'} for ${key}.`;
      onvalidation(key, message);
      onchange();
      onerror(message);
      return;
    }
    if (input.value === '') {
      draft[key] = null;
    } else if (Number.isFinite(input.valueAsNumber)) {
      draft[key] = input.valueAsNumber;
    } else {
      const message = `Enter a valid number for ${key}.`;
      onvalidation(key, message);
      onchange();
      onerror(message);
      return;
    }
    onvalidation(key, null);
    onchange();
  }
</script>

<p class="form-status">Verified workflows are editable. Grey fields and actions remain unavailable.</p>
<WorkingUnitFields bind:draft {original} controls={ordered} {position} {capabilities} {disabled}
  {onchange} {onerror} {onvalidation} {masterAllowed} onbusy={onWorkingUnitBusyChange} session={workingUnitSession}>
{#snippet children(workingUnitInputs)}
<QualityFields bind:draft {original} controls={ordered} {position} {capabilities} {disabled}
  {onchange} {onvalidation} onbusy={onQualityBusyChange}>
{#snippet children(qualityInputs)}
<SiteCodeFields bind:draft {original} controls={ordered} {position} {capabilities} {disabled}
  {onchange} {onvalidation} onbusy={onSiteCodeBusyChange}>
{#snippet children(siteCodeInputs)}
<RegionCodeFields bind:draft {original} controls={ordered} {position} {capabilities} {disabled}
  {onchange} {onvalidation} onbusy={onRegionCodeBusyChange}>
{#snippet children(regionCodeInputs)}
<SubstrateFields bind:draft {original} controls={ordered} {position} {capabilities} {disabled}
  {onchange} {onvalidation}>
{#snippet children(substrateInputs)}
<BECFields bind:draft {original} controls={ordered} {position} {capabilities} {disabled} {onchange}   {onvalidation}>
{#snippet children(becInputs)}
<CoordinateFields bind:draft bind:mode={coordinateMode} controls={ordered} {position} {capabilities} {disabled}
  {onchange} {onerror} {onvalidation} onbusy={onCoordinateBusyChange}>
{#snippet children(coordinateInputs)}
<div class="responsive-form header-editor" data-source-page="Site">
  {#each groups as group (group.title)}
    <fieldset class="form-group">
      <legend>{group.title}</legend>
      <div class="field-grid">
      {#each group.controls as control (control.controlId)}
        {@const field = byColumn.get(control.column ?? '')}
        {@const stored = control.column ? storedValues.get(control.column.toLowerCase()) : undefined}
        {@const unavailable = disabled || !control.enabled || control.locked}
        {@const action = control.type === 'CommandButton' || control.type === 'ToggleButton' || control.type === 'Subform'}
        <div class="form-field" class:full-width={wideControl(control)} class:action-field={action}>
        {#if !action}<label class="field-label" for={inputId(control)}>{controlLabel(control)}</label>{/if}
        {#if isCoordinateControl(control.controlName)}
          {@render coordinateInputs(control)}
        {:else if isWorkingUnitControl(control.controlName)}
          {@render workingUnitInputs(control)}
        {:else if becKey(control.column)}
          {@render becInputs(control)}
        {:else if qualityKey(control.column)}
          {@render qualityInputs(control)}
        {:else if substrateKey(control.column)}
          {@render substrateInputs(control)}
        {:else if siteCodeKey(control.column)}
          {@render siteCodeInputs(control)}
        {:else if regionCodeKey(control.column)}
          {@render regionCodeInputs(control)}
        {:else if additionalEditor && control.column && additionalEditor.columns.includes(control.column)}
          {@render additionalEditor.input(control, position(control))}
        {:else if editor && control.column && editor.columns.includes(control.column)}
          {@render editor.input(control, position(control))}
        {:else if control.column === 'PlotNumber'}
          <input class="paper-control" id="header-plotNumber" data-column={control.column} data-source-control={control.controlName} aria-label="Plot number"
            title={existing ? 'Existing plot identities cannot be renamed here' : 'Plot number'}
            value={draft.plotNumber} readonly={existing} disabled={unavailable || capabilities.plotNumber !== true}
            oninput={(event) => { draft.plotNumber = event.currentTarget.value; onchange(); }} />
        {:else if field}
          {#if field.kind === 'number'}
            <input class="paper-control" id={`header-${field.key}`} data-column={control.column} data-source-control={control.controlName} aria-label={field.label}
              type="number" step={field.step ?? '1'} value={draft[field.key] ?? ''}
              disabled={unavailable || capabilities[field.key] !== true}
              oninput={(event) => setNumber(field.key, event.currentTarget)} />
          {:else if field.kind === 'notes'}
            <textarea class="paper-control" id={`header-${field.key}`} data-column={control.column} data-source-control={control.controlName} aria-label={field.label}
              value={draft[field.key] ?? ''} disabled={unavailable || capabilities[field.key] !== true}
              oninput={(event) => setText(field.key, event.currentTarget.value)}></textarea>
          {:else if field.list}
            <select class="paper-control" id={`header-${field.key}`} data-column={control.column} data-source-control={control.controlName} aria-label={field.label}
              value={draft[field.key] ?? ''} disabled={unavailable || capabilities[field.key] !== true || !lists[field.key]?.length}
              onchange={(event) => setText(field.key, event.currentTarget.value)}>
              <option value="">-</option>
              {#if draft[field.key] && !lists[field.key]?.some(item => item.item === draft[field.key])}
                <option value={draft[field.key] ?? ''}>{draft[field.key]}</option>
              {/if}
              {#each lists[field.key] ?? [] as item (item.item)}
                <option value={item.item}>{item.item} - {item.itemDescription}</option>
              {/each}
            </select>
          {:else}
            <input class="paper-control" id={`header-${field.key}`} data-column={control.column} data-source-control={control.controlName} aria-label={field.label}
              value={draft[field.key] ?? ''} disabled={unavailable || capabilities[field.key] !== true}
              oninput={(event) => setText(field.key, event.currentTarget.value)} />
          {/if}
        {:else if control.type === 'CommandButton' || control.type === 'ToggleButton'}
          <button class="paper-control pending" data-source-control={control.controlName} disabled title="Workflow not yet migrated">{controlLabel(control)}</button>
        {:else if control.type === 'CheckBox' || control.type === 'OptionButton'}
          <input class="paper-control pending" id={inputId(control)} data-column={control.column} data-source-control={control.controlName} type={control.type === 'OptionButton' ? 'radio' : 'checkbox'} checked={stored === true} disabled aria-label={controlLabel(control)} />
        {:else if control.type === 'Subform'}
          <div class="pending embedded" data-source-control={control.controlName} title="Embedded workflow not yet migrated">Pictures: pending</div>
        {:else if control.type === 'TextBox' || control.type === 'ComboBox'}
          <input class="paper-control pending" id={inputId(control)} data-column={control.column} data-source-control={control.controlName} disabled value={typeof stored === 'string' || typeof stored === 'number' ? stored : ''} placeholder={stored !== undefined ? '' : 'Pending'} title={`${controlLabel(control)}: workflow not yet verified`} aria-label={controlLabel(control)} />
        {/if}
        </div>
      {/each}
      </div>
    </fieldset>
  {/each}
</div>
{/snippet}
</CoordinateFields>
{/snippet}
</BECFields>
{/snippet}
</SubstrateFields>
{/snippet}
</RegionCodeFields>
{/snippet}
</SiteCodeFields>
{/snippet}
</QualityFields>
{/snippet}
</WorkingUnitFields>

<style>
  .paper-control { box-sizing: border-box; width: 100%; min-width: 0; min-height: 40px; border: 1px solid #b7cfc0; border-radius: 4px; padding: .5rem .65rem; background: white; font: inherit; line-height: 1.4; }
  .paper-control:focus { outline: 2px solid #047857; outline-offset: 1px; }
  .paper-control:disabled { color: #78716c; background: #f5f5f4; }
  .pending { opacity: .65; }
  .embedded { display: flex; align-items: center; justify-content: center; }
</style>
