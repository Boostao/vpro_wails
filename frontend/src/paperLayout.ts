import definition from '../../resources/fs882-xl-layout.json';

export interface SourceValue {
  value?: string;
  line: number;
  inherited?: boolean;
}

export interface SourceControl {
  controlId: string;
  parentId?: string;
  controlName?: string;
  type: string;
  pageId?: string;
  line: number;
  properties: Record<string, SourceValue | undefined>;
}

interface SourceField extends SourceControl {
  column?: string;
  caption?: string;
}

interface SourceForm {
  name: string;
  source: string;
  sha256: string;
  controls: SourceControl[];
  fields: SourceField[];
  pages: { controlId: string; name?: string; caption?: string; properties: Record<string, SourceValue | undefined> }[];
  embedded: { controlId: string; form?: string; resolved: boolean; masterFields?: string[]; childFields?: string[] }[];
}

interface SourceLayout {
  root: string;
  geometryUnit: string;
  forms: SourceForm[];
}

export interface PaperControl extends SourceControl {
  column?: string;
  caption: string;
  x: number;
  y: number;
  width: number;
  height: number;
  fontSize: number;
  fontName: string;
  textAlign: 'left' | 'center' | 'right' | 'justify';
  bold: boolean;
  italic: boolean;
  locked: boolean;
  enabled: boolean;
  tabOrder: number;
}

const layout: SourceLayout = definition;
function rootForm(): SourceForm {
  const found = layout.forms.find(item => item.name === layout.root);
  if (!found || layout.geometryUnit !== 'twips') throw new Error('Invalid packaged FS882 layout metadata');
  return found;
}
const form = rootForm();

function number(properties: Record<string, SourceValue | undefined>, name: string, fallback: number) {
  const property = properties[name];
  if (property === undefined) return fallback;
  if (property.value === undefined || property.value.trim() === '') {
    throw new Error(`Empty source ${name} at line ${property.line}`);
  }
  const value = Number(property.value);
  if (!Number.isFinite(value)) throw new Error(`Invalid source ${name} at line ${property.line}`);
  return value;
}

function extent(control: SourceControl, dimension: 'Width' | 'Height') {
  const properties = control.properties;
  if (properties[dimension] !== undefined) {
    const size = number(properties, dimension, 0);
    if (size < 0) throw new Error(`Invalid source ${dimension} at line ${control.line}`);
    return size;
  }
  const start = dimension === 'Width' ? 'Left' : 'Top';
  const cached = dimension === 'Width' ? 'LayoutCachedWidth' : 'LayoutCachedHeight';
  if (properties[cached] === undefined) {
    throw new Error(`Missing source ${dimension} for ${control.controlName ?? control.type} at line ${control.line}`);
  }
  const size = number(properties, cached, 0) - number(properties, start, 0);
  if (size < 0) throw new Error(`Invalid cached ${dimension} at line ${control.line}`);
  return size;
}

function typography(control: SourceControl) {
  const fontName = control.properties.FontName?.value ?? 'Arial';
  if (!/^[\p{L}\p{N} -]+$/u.test(fontName)) throw new Error(`Invalid source font at line ${control.line}`);
  const alignment = number(control.properties, 'TextAlign', 1);
  const alignments = ['left', 'left', 'center', 'right', 'justify'] as const;
  if (!Number.isInteger(alignment) || alignment < 0 || alignment >= alignments.length) {
    throw new Error(`Invalid source alignment at line ${control.line}`);
  }
  return { fontName, textAlign: alignments[alignment] };
}

function flag(properties: Record<string, SourceValue | undefined>, name: string, defaultValue: boolean) {
  const value = properties[name]?.value;
  if (value === undefined || value === 'Default') return defaultValue;
  if (value === 'NotDefault') return !defaultValue;
  if (value === '0' || value.toLowerCase() === 'false') return false;
  if (value === '-1' || value === '1' || value.toLowerCase() === 'true') return true;
  throw new Error(`Unknown source ${name} value ${value}`);
}

export type VegetationMode = 'initial' | 'height' | 'cover';
export type CoordinateDisplayMode = 'dd' | 'dm' | 'dms';

function coordinateVisibility(mode: CoordinateDisplayMode): Record<string, boolean> {
  if (!['dd', 'dm', 'dms'].includes(mode)) throw new Error(`Invalid coordinate display mode ${mode}`);
  const groups: [string[], boolean][] = [
    [['Latitude', 'Longitude', 'lblLatitude', 'lblLongitude'], mode === 'dd'],
    [['lblLat', 'lblLon', 'lblLatD', 'lblLonD', 'lblLonM'], mode !== 'dd'],
    [['LatD', 'LatM', 'LatS', 'LonD', 'LonM', 'LonS', 'lblLatM', 'lblLatS', 'lblLonS'], mode === 'dms'],
    [['LatD2', 'LatMD', 'LonD2', 'LonMD', 'lblLatD2', 'lblLatMD', 'lblLonD2', 'lblLonMD'], mode === 'dm']
  ];
  return Object.fromEntries(groups.flatMap(([names, visible]) => names.map(name => [name, visible])));
}

export function accessCaption(value: string) {
  return value.replace(/&&|&/g, marker => marker === '&&' ? '&' : '');
}

function paperControls(source: SourceForm, candidates: SourceControl[], originX: number, originY: number, visibility: Record<string, boolean | undefined> = {}) {
  const byID = new Map(source.controls.map(control => [control.controlId, control]));
  const fields = new Map(source.fields.map(field => [field.controlId, field]));
  function visible(control: SourceControl) {
    const visited = new Set<string>();
    const independentLabel = control.type === 'Label' && visibility[control.controlName ?? ''] === true;
    let current: SourceControl | undefined = control;
    while (current) {
      if (visited.has(current.controlId)) throw new Error('Cycle in source containment');
      visited.add(current.controlId);
      const independentOwner = independentLabel && current !== control && ['TextBox', 'ComboBox'].includes(current.type);
      if (!independentOwner && !(visibility[current.controlName ?? ''] ?? flag(current.properties, 'Visible', true))) return false;
      if (current.parentId && !byID.has(current.parentId)) {
        throw new Error(`Missing source parent ${current.parentId} at line ${current.line}`);
      }
      current = current.parentId ? byID.get(current.parentId) : undefined;
    }
    return true;
  }
  const controls: PaperControl[] = candidates.filter(visible)
    .filter(control => !['Page', 'Group', 'Section', 'Form', 'Tab', 'FormHeader', 'FormFooter'].includes(control.type))
    .map(control => {
      const field = fields.get(control.controlId);
      return {
        ...control,
        column: field?.column,
        caption: field?.caption ?? control.properties.Caption?.value ?? '',
        x: (number(control.properties, 'Left', 0) - originX) / 15,
        y: (number(control.properties, 'Top', 0) - originY) / 15,
        width: extent(control, 'Width') / 15,
        height: extent(control, 'Height') / 15,
        fontSize: number(control.properties, 'FontSize', 8) * 4 / 3,
        ...typography(control),
        bold: control.properties.FontBold !== undefined
          ? flag(control.properties, 'FontBold', false)
          : number(control.properties, 'FontWeight', 400) >= 700,
        italic: flag(control.properties, 'FontItalic', false),
        locked: flag(control.properties, 'Locked', false),
        enabled: flag(control.properties, 'Enabled', true),
        tabOrder: number(control.properties, 'TabIndex', 0)
      };
    })
    .filter(control => control.width > 0 && control.height > 0);
  return controls;
}

export function paperPage(name: string, vegetationMode: VegetationMode = 'initial', coordinateMode: CoordinateDisplayMode = 'dd') {
  const page = form.pages.find(item => item.name?.replaceAll('&', '') === name || item.caption?.replaceAll('&', '') === name);
  if (!page) throw new Error(`Source page ${name} is missing`);
  const originX = number(page.properties, 'Left', 0);
  const originY = number(page.properties, 'Top', 0);
  const switching = name === 'Vegetation' && vegetationMode !== 'initial';
  const isHeight = vegetationMode === 'height';
  const visibility = name === 'Site' ? coordinateVisibility(coordinateMode)
    : switching ? { SubVegA: !isHeight, SubVegAht: isHeight, SubVegC: !isHeight, SubVegCht: isHeight } : {};
  let controls = paperControls(form, form.controls.filter(control => control.pageId === page.controlId), originX, originY, visibility);
  if (switching) {
    // FS882 btnCoverAndHeight_Click; verified on a clean disposable Access form.
    controls = controls.map(control => {
      if (control.controlName === 'SubVegD') return { ...control, x: ((isHeight ? 10900 : 8320) - originX) / 15 };
      if (control.controlName === 'VegNotes' || control.controlName === 'lblNotes') return { ...control, width: (isHeight ? 14250 : 11700) / 15 };
      if (control.controlName === 'btnCoverAndHeight') return { ...control, caption: isHeight ? 'Cover Only' : 'Cover && Height' };
      return control;
    });
  }
  const width = number(page.properties, 'Width', 0) / 15;
  const height = number(page.properties, 'Height', 0) / 15;
  if (width <= 0 || height <= 0) throw new Error(`Missing source dimensions for ${name}`);
  return {
    source: form.source,
    sha256: form.sha256,
    width,
    contentWidth: Math.max(width, ...controls.map(control => control.x + control.width)),
    height,
    controls
  };
}

export function embeddedForm(controlId: string) {
  const embedded = form.embedded.find(item => item.controlId === controlId);
  if (!embedded?.resolved || !embedded.form) throw new Error(`Unresolved source embedded form ${controlId}`);
  return { ...embedded, form: embedded.form };
}

export function paperChild(name: string) {
  const child = layout.forms.find(item => item.name === name);
  if (!child) throw new Error(`Source child form ${name} is missing`);
  const root = child.controls.find(control => control.type === 'Form');
  const detail = child.controls.find(control => control.type === 'Section' && control.controlName?.startsWith('Detail'));
  const header = child.controls.find(control => control.type === 'FormHeader');
  if (!root || !detail) throw new Error(`Source child detail is missing for ${name}`);
  const byID = new Map(child.controls.map(control => [control.controlId, control]));
  function inSection(control: SourceControl, section: SourceControl) {
    const visited = new Set<string>();
    let current: SourceControl | undefined = control;
    while (current) {
      if (current.controlId === section.controlId) return true;
      if (visited.has(current.controlId)) throw new Error('Cycle in source child containment');
      visited.add(current.controlId);
      current = current.parentId ? byID.get(current.parentId) : undefined;
    }
    return false;
  }
  const controls = paperControls(child, child.controls.filter(control => inSection(control, detail) &&
    !['ID', 'PlotNumber'].includes(control.properties.ControlSource?.value ?? '')), 0, 0);
  const headerControls = header ? paperControls(child, child.controls.filter(control => inSection(control, header)), 0, 0) : [];
  const headerHeight = header ? number(header.properties, 'Height', 0) / 15 : 0;
  const width = number(root.properties, 'Width', 0) / 15;
  const rowHeight = number(detail.properties, 'Height', 0) / 15;
  if (width <= 0 || rowHeight <= 0) throw new Error(`Invalid source child dimensions for ${name}`);
  return { name, width, rowHeight, controls, headerControls, headerHeight };
}
