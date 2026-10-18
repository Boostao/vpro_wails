import { lifeformShape, lifeformText } from './lifeformSummary';
import { fieldListCell, fieldListIndex, fieldListTable, type FieldListCell, type FieldListRow, type FieldListTable } from './summaryVegetationFieldList';
import { validateSummaryVegetationAsync, type SummaryVegetationOwner, type SummaryVegetationPreview,
  type SummaryVegetationReferenceMode } from './summaryVegetation';
import { numericEnvironmentalArgument, numericEnvironmentalValue, type SummaryEnvironmentalValue } from './summaryVegetationEnvironmentalNumeric';

export const summaryEnvironmentalSource = 'one owned SQLite snapshot: raw Summary units, physical SU counts, selected field-list and needed Env; literal scope adaptation; no formatted workbook';
const categoricalFunctions = new Set(['SVStructureStage', 'SVSuccessional', 'SVSurfaceTopographyType', 'SVSnr',
  'SVMesoSlopePosition', 'SVHydroGeoSubSystem', 'SVHydroGeoSystem', 'SVSoilDrainage', 'SVHumusForm',
  'SVRootRestrictingType', 'SVSoilClassSubGroup', 'SVSoilClassGroup', 'SVRootZonePartSize',
  'SVSurficialMaterialSurf', 'SVExposure', 'SVBioGeoClimaticZone', 'SVSmr', 'SVBioGeoClimaticUnit', 'SVAspect']);

export interface SummaryEnvironmentalRow<V = FieldListCell> { field: FieldListRow; labelColumn: number; values: V[] }
export interface SummaryEnvironmentalPreview<V = FieldListCell> {
  vegetation: SummaryVegetationPreview; fieldList: FieldListTable; rows: SummaryEnvironmentalRow<V>[];
  sourcePath: string; envSourcePath: string; source: string;
}
export interface SummaryEnvironmentalRead extends PromiseLike<unknown> { cancel?(): void }
export interface SummaryEnvironmentalPort {
  preview(contextId: string): SummaryEnvironmentalRead;
  validationYield?(): PromiseLike<void>;
}
function reject(): never { throw new Error('Combined Summary environment is incomplete, inconsistent or belongs to another owner; no repairs applied.'); }

export async function validateSummaryEnvironmental(value: unknown, owner: SummaryVegetationOwner,
  validationYield: () => PromiseLike<void>, current: () => boolean = () => true): Promise<SummaryEnvironmentalPreview | null> {
  return validateSummaryEnvironmentalRows(value, owner, validationYield, current, false, fieldListCell,
    (value, functionName, count) => {
      if (functionName === 'SVnPlots' ? value.storage !== 'integer' || value.integer !== String(count) || count > 32767 :
          value.storage !== 'text') reject();
    });
}

export const summaryEnvironmentalNumericSource = 'one owned SQLite snapshot: raw Summary units, physical SU counts, selected field-list and needed Env; literal scope adaptation; explicit numeric Empty; no formatted workbook';
export const summaryEnvironmentalLifeformSource = 'one owned SQLite snapshot: raw Summary units/Veg/master, physical SU counts, selected field-list and needed Env; source-qualified QuickVeg/lifeform arguments1 through13; literal scope adaptation; explicit numeric Empty; no formatted workbook';
export interface SummaryEnvironmentalLifeformPreview extends SummaryEnvironmentalPreview<SummaryEnvironmentalValue> {
  masterSourcePath: string;
}
type SummaryEnvironmentalCalculation = {
  average: number; display: number; showClass: number; references?: SummaryVegetationReferenceMode; groupAndOrder?: number;
  constancy?: true;
};
const numericFunctions = new Set(['SVSeepage', 'SVRootingDepth', 'SVStandAge', 'SVElevation', 'SVSlope', 'SVSubstrate', 'SVStrataCover']);
export function validateSummaryEnvironmentalNumeric(value: unknown, owner: SummaryVegetationOwner,
  validationYield: () => PromiseLike<void>, current: () => boolean = () => true): Promise<SummaryEnvironmentalPreview<SummaryEnvironmentalValue> | null> {
  return validateSummaryEnvironmentalTaggedRows(value, owner, validationYield, current, true);
}

export async function validateSummaryEnvironmentalLifeform(value: unknown, owner: SummaryVegetationOwner,
  validationYield: () => PromiseLike<void>, current: () => boolean = () => true): Promise<SummaryEnvironmentalLifeformPreview | null> {
  return validateSummaryEnvironmentalLifeformOptions(value, owner, { average: 1, display: 6, showClass: 0 },
    validationYield, current);
}

export async function validateSummaryEnvironmentalLifeformOptions(value: unknown, owner: SummaryVegetationOwner,
  options: SummaryEnvironmentalCalculation,
  validationYield: () => PromiseLike<void>, current: () => boolean = () => true): Promise<SummaryEnvironmentalLifeformPreview | null> {
  if (!current()) return null;
  if (!lifeformShape(value, ['vegetation', 'fieldList', 'rows', 'sourcePath', 'envSourcePath', 'masterSourcePath', 'source']) ||
      !lifeformText(value.masterSourcePath) || !value.masterSourcePath) reject();
  const masterSourcePath = value.masterSourcePath;
  const result = await validateSummaryEnvironmentalTaggedRows(value, owner, validationYield, current, 'lifeform', options);
  if (!current() || result === null) return null;
  return Object.freeze({ ...result, masterSourcePath });
}

function validateSummaryEnvironmentalTaggedRows(value: unknown, owner: SummaryVegetationOwner,
  validationYield: () => PromiseLike<void>, current: () => boolean, mode: true | 'lifeform',
  options: SummaryEnvironmentalCalculation = { average: 1, display: 6, showClass: 0 }): Promise<SummaryEnvironmentalPreview<SummaryEnvironmentalValue> | null> {
  return validateSummaryEnvironmentalRows<SummaryEnvironmentalValue>(value, owner, validationYield, current, mode,
    (value, functionName, argument) => {
      if (numericFunctions.has(functionName)) return numericEnvironmentalValue(functionName, argument, value);
      if (!lifeformShape(value, ['kind', 'cell']) || value.kind !== 'cell') reject();
      return { kind: 'cell', cell: fieldListCell(value.cell) };
    },
    (value, functionName, count) => {
      if (numericFunctions.has(functionName)) return;
      if (value.kind !== 'cell' || (functionName === 'SVnPlots' ?
          value.cell.storage !== 'integer' || value.cell.integer !== String(count) || count > 32767 : value.cell.storage !== 'text')) reject();
    }, options);
}

async function validateSummaryEnvironmentalRows<V>(value: unknown, owner: SummaryVegetationOwner,
  validationYield: () => PromiseLike<void>, current: () => boolean, mode: boolean | 'lifeform',
  readValue: (value: unknown, functionName: string, argument: FieldListCell) => V,
  checkValue: (value: V, functionName: string, count: number) => void,
  options: SummaryEnvironmentalCalculation = { average: 1, display: 6, showClass: 0 }): Promise<SummaryEnvironmentalPreview<V> | null> {
  if (!current()) return null;
  const lifeform = mode === 'lifeform', numeric = mode !== false;
  const source = lifeform ? summaryEnvironmentalLifeformSource : numeric ? summaryEnvironmentalNumericSource : summaryEnvironmentalSource;
  if (!lifeformShape(value, ['vegetation', 'fieldList', 'rows', 'sourcePath', 'envSourcePath', ...(lifeform ? ['masterSourcePath'] : []), 'source']) ||
      !lifeformText(value.sourcePath) || !value.sourcePath || value.envSourcePath !== owner.projectPath ||
      value.source !== source || !Array.isArray(value.rows)) reject();
  const vegetation = await validateSummaryVegetationAsync(value.vegetation, owner, options.average, options.display, options.showClass, options.groupAndOrder ?? 11,
    { preview: () => Promise.reject(new Error('Validation cannot dispatch a read')), cancel: () => {}, validationYield }, current, options.references, options.constancy);
  if (!current() || vegetation === null) return null;
  const table = fieldListTable(value.fieldList), index = fieldListIndex(table);
  if (index.error || index.selected.length !== value.rows.length) reject();
  const physical = new Map(table.rows.map(row => [row.rowId, row]));
  const counts = new Map(vegetation.units.map(unit => [unit.code, 0]));
  for (let i = 0; i < vegetation.memberships.length; i++) {
    if (i % 256 === 0) {
      await validationYield();
      if (!current()) return null;
    }
    const member = vegetation.memberships[i];
    if (member.siteUnit.text !== null && counts.has(member.siteUnit.text) && member.plotNumber.storage !== 'null')
      counts.set(member.siteUnit.text, counts.get(member.siteUnit.text)! + 1);
  }
  const rows: SummaryEnvironmentalRow<V>[] = [];
  for (let i = 0; i < value.rows.length; i++) {
    if (i % 32 === 0) {
      await validationYield();
      if (!current()) return null;
    }
    const item: unknown = value.rows[i];
    if (!lifeformShape(item, ['field', 'labelColumn', 'values']) ||
        !lifeformShape(item.field, ['rowId', 'cells']) || item.field.rowId !== index.selected[i] ||
        !Array.isArray(item.field.cells) || !Array.isArray(item.values) ||
        typeof item.labelColumn !== 'number' || item.values.length !== vegetation.units.length ||
        Reflect.ownKeys(item.values).length !== item.values.length + 1) reject();
    const field = physical.get(index.selected[i]);
    if (!field || item.field.cells.length !== 7) reject();
    const cells = fieldListTable({ columns: table.columns, rows: [item.field] }).rows[0].cells;
    if (cells.some((cell, n) => JSON.stringify(cell) !== JSON.stringify(field.cells[n]))) reject();
    const functionName = cells[5].text;
    const numericFunction = numeric && functionName !== null && numericFunctions.has(functionName);
    const lifeformFunction = lifeform && functionName === 'SVLifeformCover';
    const dynamic = functionName === 'SVBedrockType' || functionName === 'SVSiteDisturbance';
    const argument = cells[6];
    if (lifeformFunction) {
      if (argument.storage !== 'text' ||
          !['1', '2', '3', '4', '5', '6', '7', '8', '9', '10', '11', '12', '13'].includes(argument.text ?? '')) reject();
    } else if (numericFunction) numericEnvironmentalArgument(functionName!, argument);
    else if (dynamic ? argument.storage !== 'text' ||
        !['1', '2', ...(functionName === 'SVBedrockType' ? ['3'] : [])].includes(argument.text ?? '') :
        argument.storage !== 'null') reject();
    if (cells[5].storage !== 'text' || !functionName ||
        functionName !== 'SVnPlots' && !categoricalFunctions.has(functionName) && !dynamic && !numericFunction && !lifeformFunction ||
        [cells[1], cells[2]].some(cell => cell.storage !== 'text' && cell.storage !== 'null') ||
        item.labelColumn !== (cells[2].text === 'n Plots' ? 3 : 2)) reject();
    const values = Array.from(item.values, value => readValue(value, functionName, argument));
    for (let unit = 0; unit < values.length; unit++) {
      checkValue(values[unit], functionName, counts.get(vegetation.units[unit].code)!);
    }
    rows.push({ field: { rowId: field.rowId, cells }, labelColumn: item.labelColumn, values });
  }
  if (!current()) return null;
  const result = { vegetation, fieldList: table, rows, sourcePath: value.sourcePath, envSourcePath: owner.projectPath,
    source };
  const pending: object[] = [result];
  let frozen = 0;
  while (pending.length) {
    if (frozen++ % 256 === 0) {
      await validationYield();
      if (!current()) return null;
    }
    const item = pending.pop()!;
    for (const child of Object.values(item))
      if (child !== null && typeof child === 'object' && !Object.isFrozen(child)) pending.push(child);
    Object.freeze(item);
  }
  return result;
}

class SummaryEnvironmentalSessionBase<V, P extends SummaryEnvironmentalPreview<V> = SummaryEnvironmentalPreview<V>> {
  private generation = 0;
  private pending: SummaryEnvironmentalRead | null = null;
  private dispatches = 0;
  private busy = false;
  private error = '';
  private result: P | null = null;
  private listeners = new Set<() => void>();
  readonly owner: SummaryVegetationOwner;
  constructor(owner: SummaryVegetationOwner,
    private readonly validator: (value: unknown, owner: SummaryVegetationOwner, validationYield: () => PromiseLike<void>,
      current: () => boolean) => Promise<P | null>) {
    if (!lifeformShape(owner, ['contextId', 'project', 'projectPath', 'su', 'suPath']) ||
        Object.values(owner).some(value => !lifeformText(value) || !value) ||
        owner.su === 'None' || owner.su === 'USysSuTableDynamic') reject();
    this.owner = Object.freeze({ ...owner });
  }
  view() { return { busy: this.busy, settling: this.dispatches > 0, error: this.error, result: this.result }; }
  subscribe(listener: () => void) { this.listeners.add(listener); listener(); return () => { this.listeners.delete(listener); }; }
  private notify() { for (const listener of this.listeners) listener(); }
  cancel() {
    this.generation++;
    this.busy = false;
    this.result = null;
    const pending = this.pending; this.pending = null;
    this.error = 'Combined environmental read cancelled; explicit Preview required.';
    try { pending?.cancel?.(); } catch (cause) { this.error += ` ${String(cause)}`; }
    this.notify();
  }
  async show(port: SummaryEnvironmentalPort): Promise<boolean> {
    if (this.dispatches) throw new Error('Wait for the current combined environmental read and validation to settle.');
    const generation = ++this.generation;
    this.busy = true; this.error = ''; this.result = null; this.dispatches++; this.notify();
    try {
      if (generation !== this.generation) return false;
      const pending = port.preview(this.owner.contextId); this.pending = pending;
      const value = await pending;
      if (generation !== this.generation) return false;
      const reviewed = await this.validator(value, this.owner,
        port.validationYield ?? (() => new Promise<void>(resolve => setTimeout(resolve, 0))),
        () => generation === this.generation);
      if (generation !== this.generation || reviewed === null) return false;
      this.result = reviewed;
      return true;
    } catch (cause) {
      if (generation === this.generation)
        this.error = `Combined environmental preview unavailable; no data/settings saved: ${String(cause)}`;
      return false;
    } finally {
      this.dispatches--;
      if (generation === this.generation) { this.busy = false; this.pending = null; }
      this.notify();
    }
  }

}

export class SummaryEnvironmentalSession extends SummaryEnvironmentalSessionBase<FieldListCell> {
  constructor(owner: SummaryVegetationOwner) { super(owner, validateSummaryEnvironmental); }
}
export class SummaryEnvironmentalNumericSession extends SummaryEnvironmentalSessionBase<SummaryEnvironmentalValue> {
  constructor(owner: SummaryVegetationOwner) { super(owner, validateSummaryEnvironmentalNumeric); }
}
export class SummaryEnvironmentalLifeformSession extends SummaryEnvironmentalSessionBase<SummaryEnvironmentalValue, SummaryEnvironmentalLifeformPreview> {
  constructor(owner: SummaryVegetationOwner) { super(owner, validateSummaryEnvironmentalLifeform); }
}
export function summaryEnvironmentalRegistry(enabled: boolean) {
  return environmentalRegistry(enabled, owner => new SummaryEnvironmentalSession(owner));
}
export function summaryEnvironmentalNumericRegistry(enabled: boolean) {
  return environmentalRegistry(enabled, owner => new SummaryEnvironmentalNumericSession(owner));
}
export function summaryEnvironmentalLifeformRegistry(enabled: boolean) {
  return environmentalRegistry(enabled, owner => new SummaryEnvironmentalLifeformSession(owner));
}
function environmentalRegistry<S extends SummaryEnvironmentalSessionBase<unknown>>(enabled: boolean,
  create: (owner: SummaryVegetationOwner) => S) {
  const sessions = new Map<string, S>();
  const listeners = new Set<() => void>();
  const notify = () => { for (const listener of listeners) listener(); };
  return {
    session(owner: SummaryVegetationOwner) {
      if (!enabled) throw new Error('Combined environmental preview is disabled in this session.');
      const checked = create(owner);
      const key = JSON.stringify([owner.contextId, owner.project, owner.projectPath, owner.su, owner.suPath]);
      const existing = sessions.get(key);
      if (existing) return existing;
      if (sessions.size >= 16) throw new Error('Combined environmental retained-context capacity reached; no state is evicted.');
      sessions.set(key, checked);
      checked.subscribe(notify);
      return checked;
    },
    activity() { return { busy: [...sessions.values()].some(session => session.view().busy || session.view().settling) }; },
    subscribe(listener: () => void) {
      listeners.add(listener); listener();
      return () => { listeners.delete(listener); };
    },
  };
}
