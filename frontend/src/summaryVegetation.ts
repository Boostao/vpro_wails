import { lifeformText, lifeformShape, lifeformCell, lifeformRowID, type LifeformTextCell } from './lifeformSummary';

export type SummaryVegetationOwner = { contextId: string; project: string; projectPath: string; su: string; suPath: string };
export type SummaryVegetationReferenceMode = { showCommon: boolean; useGuideNames: boolean };
export type SummaryVegetationReferenceMetadata = {
  masterRows: number; personalRows: number; descriptionRows: number; removedMasterRows: number;
  derivedRows: number; unionRows: number; showCommon: boolean; useGuideNames: boolean;
};
export type SummaryVegetationWitness = {
  stageRowId: string; conversionLayerId: string; membershipId: string; unit: string; layerMapId: string;
  referenceOrigins: { table: string; rowId: string }[]; rawVegRowIds: string[];
  cover: { storage: 'real'; real: number; text: null; integer: null; blobHex: null };
};
export type SummaryVegetationUnit = { code: string; numPlots: number; membershipIds: string[]; rawVegRowIds: string[] };
export type SummaryVegetationField = { name: string; daoType: number; size: number };
export type SummaryVegetationOrder = LifeformTextCell | { storage: 'real'; text: null; integer: null; real: number; blobHex: null };
export type SummaryVegetationGroup = LifeformTextCell | {
  storage: 'integer'; text: null; integer: string; real: null; blobHex: null;
};
export type SummaryVegetationRow = {
  layer: SummaryVegetationGroup; species: LifeformTextCell; order: SummaryVegetationOrder;
  commonName?: LifeformTextCell; rawSpecies?: LifeformTextCell;
  covers: LifeformTextCell[]; witnesses: SummaryVegetationWitness[];
};
export type SummaryVegetationPreview = SummaryVegetationOwner & {
  average: number; display: number; showClass?: number; groupAndOrder?: number;
  groupField?: SummaryVegetationField; orderField?: SummaryVegetationField;
  source: 'literal-normal-su-layer' | 'literal-normal-su-grouped' | 'literal-normal-su-effective-references' | 'literal-normal-su-constancy';
  ordering: 'literal-null-first-layer-species' | 'literal-null-first-group-species' | 'literal-null-first-layer-species-common' | 'literal-null-first-numeric-lifeform-species-raw-code';
  references?: SummaryVegetationReferenceMetadata;
  memberships: { rowId: string; plotNumber: LifeformTextCell; siteUnit: LifeformTextCell }[];
  units: SummaryVegetationUnit[]; columns: string[]; rows: SummaryVegetationRow[];
};

function reject(): never { throw new Error('Summary Vegetation is incomplete, inconsistent or belongs to another context; no repairs applied.'); }
function ids(value: unknown, nonempty = false): value is string[] {
  return Array.isArray(value) && (!nonempty || value.length > 0) && value.every(lifeformRowID) && new Set(value).size === value.length;
}
function literalCompare(a: string, b: string): number {
  const left = [...a], right = [...b];
  for (let i = 0; i < Math.min(left.length, right.length); i++) {
    const difference = left[i].codePointAt(0)! - right[i].codePointAt(0)!;
    if (difference !== 0) return difference;
  }
  return left.length - right.length;
}
function compareCell(a: { text: string | null }, b: { text: string | null }): number {
  if (a.text === null || b.text === null) return a.text === null ? b.text === null ? 0 : -1 : 1;
  return literalCompare(a.text, b.text);
}
function summaryGroupCell(value: unknown, constancy: boolean, limit: number): value is SummaryVegetationGroup {
  if (!constancy) return lifeformCell(value, limit);
  if (!lifeformShape(value, ['storage', 'text', 'integer', 'real', 'blobHex'])) return false;
  if (value.storage === 'null') return lifeformCell(value);
  return value.storage === 'integer' && value.text === null && value.real === null && value.blobHex === null &&
    typeof value.integer === 'string' && /^(?:0|-?[1-9]\d*)$/.test(value.integer) &&
    Number(value.integer) >= -2147483648 && Number(value.integer) <= 2147483647;
}
function compareGroup(a: SummaryVegetationGroup, b: SummaryVegetationGroup, constancy: boolean): number {
  if (!constancy) return compareCell(a, b);
  if (a.integer === null || b.integer === null) return a.integer === null ? b.integer === null ? 0 : -1 : 1;
  return Number(a.integer) - Number(b.integer);
}
export function summaryVegetationReferenceGroupingAvailable(groupAndOrder: number, common: boolean): boolean {
  return groupAndOrder === 11 || groupAndOrder === 12 || !common && (groupAndOrder === 13 || groupAndOrder === 14);
}
export function summaryVegetationOptionError(average: unknown, display: unknown, showClass: unknown = 0, groupAndOrder: unknown = 11): string {
  if (groupAndOrder !== 11 && groupAndOrder !== 12 && groupAndOrder !== 13 && groupAndOrder !== 14)
    return 'Choose a supported Group by option; no settings were saved.';
  if (groupAndOrder !== 11 && groupAndOrder !== 14 && display === 2)
    return 'Strata and Lifeform Presence Class and Significance Class have an original criteria type mismatch; choose another Display Values option. No settings were saved.';
  if ((average !== 1 && average !== 2) || typeof display !== 'number' ||
      ![1, 2, 3, 4, 5, 6].includes(display)) return 'Choose a supported Calculate and Display Values option; no settings were saved.';
  if (display === 4 || display === 5) {
    if (showClass !== 1 && showClass !== 10 && showClass !== 20) return 'Choose a supported Calculate and Display Values option and Show Class or Value; no settings were saved.';
  } else if (showClass !== 0 && showClass !== 1 && showClass !== 10 && showClass !== 20) {
    return 'Choose a supported Show Class or Value option; no settings were saved.';
  }
  return '';
}

function summaryOrderCell(value: unknown, grouped: boolean): value is SummaryVegetationOrder {
  if (!grouped) return lifeformCell(value);
  return lifeformShape(value, ['storage', 'text', 'integer', 'real', 'blobHex']) &&
    value.storage === 'real' && value.text === null && value.integer === null && value.blobHex === null &&
    typeof value.real === 'number' && Number.isFinite(value.real);
}

function* summaryVegetationValidation(value: unknown, owner: SummaryVegetationOwner,
  average: number, display: number, showClass = 0, groupAndOrder = 11,
  references?: SummaryVegetationReferenceMode, constancy = false): Generator<void, SummaryVegetationPreview> {
  const classMode = display === 4 || display === 5;
  const grouped = groupAndOrder !== 11;
  const common = !!references && (references.showCommon || references.useGuideNames);
  const source = constancy ? 'literal-normal-su-constancy' : references ? 'literal-normal-su-effective-references' :
    grouped ? 'literal-normal-su-grouped' : 'literal-normal-su-layer';
  const ordering = constancy ? 'literal-null-first-numeric-lifeform-species-raw-code' : common ? 'literal-null-first-layer-species-common' :
    grouped ? 'literal-null-first-group-species' : 'literal-null-first-layer-species';
  if (references && (!summaryVegetationReferenceGroupingAvailable(groupAndOrder, common) ||
      !lifeformShape(references, ['showCommon', 'useGuideNames']) ||
      typeof references.showCommon !== 'boolean' || typeof references.useGuideNames !== 'boolean')) reject();
  if (constancy && (groupAndOrder !== 13 || display !== 1 || !references || common)) reject();
  if (summaryVegetationOptionError(average, display, showClass, groupAndOrder) || !Object.values(owner).every(lifeformText) ||
      owner.contextId === '' || owner.project === '' || owner.projectPath === '' || owner.suPath === '' ||
      owner.su === '' || owner.su === 'None' || owner.su === 'USysSuTableDynamic' ||
      !lifeformShape(value, ['contextId', 'project', 'projectPath', 'su', 'suPath', 'average', 'display',
        'source', 'ordering', 'memberships', 'units', 'columns', 'rows', ...(classMode ? ['showClass'] : []),
        ...(grouped ? ['groupAndOrder', 'groupField', 'orderField'] : []), ...(references ? ['references'] : [])]) ||
      Object.entries(owner).some(([key, item]) => value[key] !== item) ||
      value.average !== average || value.display !== display || classMode && value.showClass !== showClass ||
      value.source !== source || value.ordering !== ordering || !Array.isArray(value.memberships) ||
      !Array.isArray(value.units) || !Array.isArray(value.columns) || !Array.isArray(value.rows)) reject();
  let referenceMetadata: SummaryVegetationReferenceMetadata | undefined;
  if (references) {
    const raw = value.references;
    if (!lifeformShape(raw, ['masterRows', 'personalRows', 'descriptionRows', 'removedMasterRows', 'derivedRows',
      'unionRows', 'showCommon', 'useGuideNames'])) reject();
    const { masterRows, personalRows, descriptionRows, removedMasterRows, derivedRows, unionRows } = raw;
    const count = (item: unknown): item is number => typeof item === 'number' && Number.isSafeInteger(item) && item >= 0;
    if (!count(masterRows) || !count(personalRows) || !count(descriptionRows) || !count(removedMasterRows) ||
        !count(derivedRows) || !count(unionRows) || removedMasterRows > masterRows || derivedRows > descriptionRows ||
        unionRows > masterRows - removedMasterRows + personalRows + derivedRows ||
        raw.showCommon !== common || raw.useGuideNames !== references.useGuideNames) reject();
    referenceMetadata = { masterRows, personalRows, descriptionRows, removedMasterRows, derivedRows, unionRows,
      showCommon: common, useGuideNames: references.useGuideNames };
  }
  const groupField = constancy ? { name: 'Lifeform', daoType: 4, size: 4 } : grouped ? { name: groupAndOrder === 12 ? 'Layer' : groupAndOrder === 13 ? 'LifeForm' : 'Group',
    daoType: groupAndOrder === 14 ? 9 : 10, size: groupAndOrder === 12 ? 1 : groupAndOrder === 13 ? 3 : 0 } : undefined;
  const orderField = grouped ? { name: 'Order', daoType: display === 6 ? 7 : 10, size: display === 6 ? 8 : 0 } : undefined;
  if (grouped) {
    const observedGroup = value.groupField, observedOrder = value.orderField;
    if (value.groupAndOrder !== groupAndOrder ||
        !lifeformShape(observedGroup, ['name', 'daoType', 'size']) || !lifeformShape(observedOrder, ['name', 'daoType', 'size']) ||
        Object.entries(groupField!).some(([key, field]) => observedGroup[key] !== field) ||
        Object.entries(orderField!).some(([key, field]) => observedOrder[key] !== field)) reject();
  }
  const memberships: SummaryVegetationPreview['memberships'] = [], memberMap = new Map<string, SummaryVegetationPreview['memberships'][number]>();
  for (const item of value.memberships) {
    yield;
    if (!lifeformShape(item, ['rowId', 'plotNumber', 'siteUnit']) || !lifeformRowID(item.rowId) ||
        memberMap.has(item.rowId) || !lifeformCell(item.plotNumber) || !lifeformCell(item.siteUnit)) reject();
    const member = { rowId: item.rowId, plotNumber: { ...item.plotNumber }, siteUnit: { ...item.siteUnit } };
    memberships.push(member); memberMap.set(member.rowId, member);
  }
  const units: SummaryVegetationUnit[] = [], unitMap = new Map<string, SummaryVegetationUnit>();
  for (const item of value.units) {
    yield;
    if (!lifeformShape(item, ['code', 'numPlots', 'membershipIds', 'rawVegRowIds']) || !lifeformText(item.code) ||
        unitMap.has(item.code) || typeof item.numPlots !== 'number' || !Number.isInteger(item.numPlots) ||
        item.numPlots < 1 || item.numPlots > 2147483647 || !ids(item.membershipIds, true) || !ids(item.rawVegRowIds, true)) reject();
    const members = memberships.filter(member => member.siteUnit.text === item.code);
    const membershipIds = item.membershipIds;
    if (members.length !== membershipIds.length || members.some(member => !membershipIds.includes(member.rowId)) ||
        members.filter(member => member.plotNumber.storage === 'text').length !== item.numPlots) reject();
    const unit = { code: item.code, numPlots: item.numPlots, membershipIds: [...item.membershipIds], rawVegRowIds: [...item.rawVegRowIds] };
    units.push(unit); unitMap.set(unit.code, unit);
  }
  if (units.some((unit, i) => i > 0 && literalCompare(units[i - 1].code, unit.code) >= 0)) reject();
  const columns: string[] = [];
  for (const code of value.columns) {
    yield;
    if (!lifeformText(code) || !unitMap.has(code) || columns.includes(code) ||
        columns.length > 0 && literalCompare(columns[columns.length - 1], code) >= 0) reject();
    columns.push(code);
  }
  const rows: SummaryVegetationRow[] = [], stages = new Map<string, string>(), groups = new Set<string>();
  for (const item of value.rows) {
    yield;
    if (!lifeformShape(item, ['layer', 'species', 'order', 'covers', 'witnesses',
        ...(common ? ['commonName'] : []), ...(constancy ? ['rawSpecies'] : [])]) ||
        !summaryGroupCell(item.layer, constancy, groupAndOrder === 12 ? 1 : 3) ||
        groupAndOrder === 14 && item.layer.storage !== 'null' ||
        !lifeformCell(item.species, 255) ||
        !summaryOrderCell(item.order, grouped && display === 6) ||
        !Array.isArray(item.covers) || item.covers.length !== columns.length ||
        !Array.isArray(item.witnesses) || item.witnesses.length === 0 ||
        (display === 2 || display === 6 || classMode) && item.order.storage === 'null') reject();
    let commonName: LifeformTextCell | undefined;
    if (common) {
      if (!lifeformCell(item.commonName, 255)) reject();
      commonName = { ...item.commonName };
    }
    let rawSpecies: LifeformTextCell | undefined;
    if (constancy) {
      if (!lifeformCell(item.rawSpecies, 8)) reject();
      rawSpecies = { ...item.rawSpecies };
    }
    const key = JSON.stringify([item.layer.storage, constancy ? item.layer.integer : item.layer.text,
      item.species.storage, item.species.text, ...(commonName ? [commonName.storage, commonName.text] : []),
      ...(rawSpecies ? [rawSpecies.storage, rawSpecies.text] : [])]);
    if (groups.has(key)) reject();
    groups.add(key);
    const covers: LifeformTextCell[] = [];
    for (const cell of item.covers) {
      yield;
      if (!lifeformCell(cell) || cell.text !== null && (
        classMode ? !summaryVegetationClassText(cell.text, display, showClass) :
        display === 6 ? !/^\d+\.\d{2}$/.test(cell.text) :
        display === 1 ? !/^\d+\.\d{2} - \d+\.\d{2}$/.test(cell.text) :
        display === 2 ? !/^(I|II|III|IV|V) - (\+|[1-9])$/.test(cell.text) :
        !/^(I|II|III|IV|V|) - \d+\.\d{2}$/.test(cell.text))) reject();
      covers.push({ ...cell });
    }
    const witnesses: SummaryVegetationWitness[] = [], occurrenceKeys = new Set<string>();
    for (const witness of item.witnesses) {
      yield;
      if (!lifeformShape(witness, ['stageRowId', 'conversionLayerId', 'membershipId', 'unit', 'layerMapId',
        'referenceOrigins', 'rawVegRowIds', 'cover']) || !lifeformRowID(witness.stageRowId) ||
        typeof witness.conversionLayerId !== 'string' ||
        (grouped ? witness.conversionLayerId !== '' : !lifeformRowID(witness.conversionLayerId)) || !lifeformRowID(witness.membershipId) ||
        !lifeformText(witness.unit) || !columns.includes(witness.unit) ||
        typeof witness.layerMapId !== 'string' || witness.layerMapId !== '' && !lifeformRowID(witness.layerMapId) ||
        !Array.isArray(witness.referenceOrigins) || !ids(witness.rawVegRowIds, true) ||
        !lifeformShape(witness.cover, ['storage', 'text', 'integer', 'real', 'blobHex']) ||
        witness.cover.storage !== 'real' || witness.cover.text !== null || witness.cover.integer !== null ||
        witness.cover.blobHex !== null || typeof witness.cover.real !== 'number' ||
        !Number.isFinite(witness.cover.real) || Math.fround(witness.cover.real) !== witness.cover.real) reject();
      const member = memberMap.get(witness.membershipId), unit = unitMap.get(witness.unit);
      if (!member || member.siteUnit.text !== witness.unit || member.plotNumber.text === null || !unit ||
          witness.rawVegRowIds.some(id => !unit.rawVegRowIds.includes(id))) reject();
      const origins: SummaryVegetationWitness['referenceOrigins'] = [], originKeys = new Set<string>();
      for (const origin of witness.referenceOrigins) {
        yield;
        if (!lifeformShape(origin, ['table', 'rowId']) || !lifeformRowID(origin.rowId) ||
            origin.table !== 'USysAllSpecs' && origin.table !== 'USysUserSpp' &&
              (!references || origin.table !== 'USysSppAttributeDescription')) reject();
        const key = JSON.stringify([origin.table, origin.rowId]);
        if (originKeys.has(key)) reject();
        originKeys.add(key); origins.push({ table: origin.table, rowId: origin.rowId });
      }
      const stageKey = JSON.stringify([member.plotNumber.text, witness.conversionLayerId, witness.layerMapId,
        witness.cover.real, witness.rawVegRowIds]);
      if (stages.has(witness.stageRowId) && stages.get(witness.stageRowId) !== stageKey) reject();
      stages.set(witness.stageRowId, stageKey);
      const occurrence = JSON.stringify([witness.stageRowId, witness.membershipId, [...originKeys].sort()]);
      if (occurrenceKeys.has(occurrence)) reject();
      occurrenceKeys.add(occurrence);
      witnesses.push({
        stageRowId: witness.stageRowId, conversionLayerId: witness.conversionLayerId, membershipId: witness.membershipId,
        unit: witness.unit, layerMapId: witness.layerMapId, referenceOrigins: origins, rawVegRowIds: [...witness.rawVegRowIds],
        cover: { storage: 'real', text: null, integer: null, blobHex: null, real: witness.cover.real },
      });
    }
    for (let i = 0; i < covers.length; i++) {
      if (covers[i].text !== null && !witnesses.some(witness => witness.unit === columns[i])) reject();
    }
    const orders = covers.flatMap(cell => cell.text === null ? [] : [
      display === 1 || display === 3 ? cell.text.split(' - ')[0] : cell.text,
    ]).filter(text => text !== '');
    if (grouped && display === 6) {
      if (orders.length === 0 || item.order.real !== Math.max(...orders.map(Number))) reject();
    } else {
      orders.sort(literalCompare);
      if (item.order.text !== (orders.length ? orders[orders.length - 1] : null)) reject();
    }
    const row: SummaryVegetationRow = { layer: { ...item.layer }, species: { ...item.species },
      ...(commonName ? { commonName } : {}), ...(rawSpecies ? { rawSpecies } : {}),
      order: { ...item.order }, covers, witnesses };
    if (rows.length) {
      const prior = rows[rows.length - 1];
      if ((compareGroup(prior.layer, row.layer, constancy) || compareCell(prior.species, row.species) ||
          (common ? compareCell(prior.commonName!, row.commonName!) : 0) ||
          (constancy ? compareCell(prior.rawSpecies!, row.rawSpecies!) : 0)) >= 0) reject();
    }
    rows.push(row);
  }
  const result: SummaryVegetationPreview = { ...owner, average, display,
    source, ordering, ...(referenceMetadata ? { references: referenceMetadata } : {}),
    ...(grouped ? { groupAndOrder, groupField, orderField } : {}),
    ...(classMode ? { showClass } : {}), memberships, units, columns, rows };
  const pending: object[] = [result];
  while (pending.length) {
    yield;
    const current = pending.pop()!;
    for (const child of Object.values(current)) {
      if (child !== null && typeof child === 'object') pending.push(child);
    }
    Object.freeze(current);
  }
  return result;
}

export function validateSummaryVegetation(value: unknown, owner: SummaryVegetationOwner,
  average: number, display: number, showClass = 0, groupAndOrder = 11): SummaryVegetationPreview {
  const validation = summaryVegetationValidation(value, owner, average, display, showClass, groupAndOrder);
  let step = validation.next();
  while (!step.done) step = validation.next();
  return step.value;
}

export async function validateSummaryVegetationAsync(value: unknown, owner: SummaryVegetationOwner,
  average: number, display: number, showClass: number, groupAndOrder: number, port: SummaryVegetationPort,
  current: () => boolean, references?: SummaryVegetationReferenceMode, constancy = false): Promise<SummaryVegetationPreview | null> {
  const validation = summaryVegetationValidation(value, owner, average, display, showClass, groupAndOrder, references, constancy);
  let count = 0;
  for (;;) {
    if (!current()) return null;
    if (count++ % 256 === 0) {
      await (port.validationYield?.() ?? new Promise<void>(resolve => setTimeout(resolve, 0)));
      if (!current()) return null;
    }
    const step = validation.next();
    if (step.done) return step.value;
  }
}

export interface SummaryVegetationPort {
  preview(average: number, display: number): PromiseLike<unknown>;
  previewClass?(average: number, display: number, showClass: number): PromiseLike<unknown>;
  previewGrouped?(average: number, groupAndOrder: number): PromiseLike<unknown>;
  previewGroupedDisplay?(average: number, groupAndOrder: number, display: number, showClass: number): PromiseLike<unknown>;
  cancel(): void;
  validationYield?(): PromiseLike<void>;
}
export class SummaryVegetationSession {
  private average = 1;
  private display = 1;
  private showClass = 1;
  private groupAndOrder = 11;
  private error = '';
  private busy = false;
  private generation = 0;
  private preview: SummaryVegetationPreview | null = null;
  private listeners = new Set<() => void>();
  private owner: SummaryVegetationOwner;
  constructor(owner: SummaryVegetationOwner) { this.owner = Object.freeze({ ...owner }); }
  subscribe(listener: () => void): () => void { this.listeners.add(listener); listener(); return () => this.listeners.delete(listener); }
  private notify(): void { for (const listener of this.listeners) listener(); }
  view() {
    return { average: this.average, display: this.display, showClass: this.showClass, groupAndOrder: this.groupAndOrder, error: this.error, busy: this.busy,
      draftError: summaryVegetationOptionError(this.average, this.display, this.showClass, this.groupAndOrder), preview: this.preview };
  }
  edit(average: number, display: number, showClass = 0, groupAndOrder = 11): void {
    if (this.busy) throw new Error('Cancel the current Summary Vegetation read before changing its draft.');
    this.average = average; this.display = display; this.showClass = showClass; this.groupAndOrder = groupAndOrder;
    this.preview = null; this.error = ''; this.generation++; this.notify();
  }
  cancel(port: SummaryVegetationPort): void { this.generation++; port.cancel(); this.busy = false; this.notify(); }
  async show(port: SummaryVegetationPort): Promise<void> {
    if (this.busy) throw new Error('Summary Vegetation preview is already running.');
    if (this.view().draftError) { this.error = this.view().draftError; this.notify(); return; }
    const request = ++this.generation, average = this.average, display = this.display, showClass = this.showClass, groupAndOrder = this.groupAndOrder;
    this.busy = true; this.error = ''; this.preview = null; this.notify();
    try {
      if (groupAndOrder === 11 && (display === 4 || display === 5) && !port.previewClass) throw new Error('Summary Vegetation class transport is unavailable.');
      if (groupAndOrder !== 11 && (display === 6 ? !port.previewGrouped : !port.previewGroupedDisplay))
        throw new Error('Summary Vegetation grouped transport is unavailable.');
      const value = await (groupAndOrder !== 11 ?
        display === 6 ? port.previewGrouped!(average, groupAndOrder) :
          port.previewGroupedDisplay!(average, groupAndOrder, display, display === 4 || display === 5 ? showClass : 0) :
        display === 4 || display === 5 ? port.previewClass!(average, display, showClass) : port.preview(average, display));
      if (request !== this.generation) return;
      const result = await validateSummaryVegetationAsync(value, this.owner, average, display, showClass, groupAndOrder, port,
        () => request === this.generation);
      if (request !== this.generation || result === null) return;
      this.preview = result;
    } catch (cause) {
      if (request === this.generation) this.error = `Summary Vegetation unavailable; no data or settings saved: ${String(cause)}`;
    } finally {
      if (request === this.generation) { this.busy = false; this.notify(); }
    }
  }
}

function summaryVegetationClassText(text: string, display: number, showClass: number): boolean {
  if (showClass !== 20) return display === 4 ? /^[1-5]$/.test(text) : /^[0-6]$/.test(text);
  if (!/^-?(?:0|[1-9]\d*)(?:\.\d*[1-9])?(?:E[+-]\d{2})?$/.test(text)) return false;
  const unsigned = text.replace(/^-/, ''), [mantissa, exponentText] = unsigned.split('E');
  const digits = mantissa.replace('.', '').replace(/^0+|0+$/g, '').length;
  if (digits > 7) return false;
  if (exponentText !== undefined) {
    const exponent = Number(exponentText);
    if (!/^[1-9](?:\.\d*[1-9])?$/.test(mantissa) ||
        exponent <= 6 && digits - 1 - exponent <= 7) return false;
  } else if (mantissa.split('.')[0].length > 7 || (mantissa.split('.')[1]?.length ?? 0) > 7) return false;
  const value = Number(text);
  return Number.isFinite(value) && Math.abs(value) <= 3.4028235e38 && (value === 0 || Math.abs(value) >= 1.401298e-45);
}

const sessions = new Map<string, SummaryVegetationSession>();
export function summaryVegetationSession(owner: SummaryVegetationOwner): SummaryVegetationSession {
  const key = JSON.stringify([owner.contextId, owner.project, owner.projectPath, owner.su, owner.suPath]);
  let session = sessions.get(key);
  if (!session) { session = new SummaryVegetationSession(owner); sessions.set(key, session); }
  return session;
}
