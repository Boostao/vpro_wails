import xl from '../../resources/fs882-xl-layout.json';
import { twoPageParentFields, twoPageParentSource, type TwoPageParentForm } from './twoPageParentSession';
import { twoPageParentCommonFields, validateTwoPageParentCommonOriginal } from './twoPageParentCommonSession';
import { sourcePaperPage, type CoordinateDisplayMode } from './paperLayout';
import type { SIVIParentOriginal } from './siviParentEditor';
import { metadataCellText } from './projectMetadataEditor';

export type TwoPageEntryScope = 'xl' | 'common' | 'extra';
export interface TwoPageEntryField {
  column: string; scope: TwoPageEntryScope; sourceInstances: string[];
  controlId: string; page: string | null;
}
export type TwoPageEntryChild = ReturnType<typeof twoPageEntryProjection>['children'][number];

export function twoPageEntryValues(original: SIVIParentOriginal, form: TwoPageParentForm): Map<string, string | null> {
  const reviewed = validateTwoPageParentCommonOriginal(original, form);
  return new Map(twoPageEntryProjection(form).fields.map(field => {
    const binding = reviewed.Bindings.find(binding => binding.ControlID === field.controlId);
    if (!binding) throw new Error(`Two-page ${field.column} has no original source binding.`);
    const row = binding.Table === reviewed.EnvTable ? reviewed.Rows[0].Env : reviewed.Rows[0].Admin;
    const value = row.cells[binding.Column];
    return [field.column.toLowerCase(), value.storage === 'null' ? null : metadataCellText(value)];
  }));
}

export function twoPageEntryProjection(form: TwoPageParentForm, coordinateMode: CoordinateDisplayMode = 'dd') {
  const source = twoPageParentSource(form);
  const reverseControls = source.controls.filter(control => control.controlName === 'btnSuIntoEnv');
  const reverse = reverseControls[0];
  if (reverseControls.length !== 1 || reverse.type !== 'CommandButton' ||
      !('Caption' in reverse.properties) || reverse.properties.Caption?.value !== 'SU Into Env' ||
      !reverse.events || reverse.events.length !== 1 ||
      reverse.events[0].property !== 'OnClick' || reverse.events[0].binding !== '[Event Procedure]' ||
      reverse.events[0].procedure !== 'btnSuIntoEnv_Click' || !reverse.events[0].resolved) {
    throw new Error('Two-page reverse transfer lost its exact source control/callback.');
  }
  const reverseAction = { controlId: reverse.controlId, caption: reverse.properties.Caption.value };
  const auditControls = source.controls.filter(control => control.controlName === 'btnAudit');
  const audit = auditControls[0];
  if (auditControls.length !== 1 || audit.type !== 'CommandButton' ||
      !('Caption' in audit.properties) || audit.properties.Caption?.value !== 'Audit' || audit.events?.length !== 1 ||
      audit.events[0].property !== 'OnClick' || audit.events[0].binding !== '[Event Procedure]' ||
      audit.events[0].procedure !== 'btnAudit_Click' || !audit.events[0].resolved) {
    throw new Error('Two-page Audit lost its exact source control/callback.');
  }
  const auditAction = { controlId: audit.controlId, caption: audit.properties.Caption.value };
  const googleEarthControls = source.controls.filter(control => control.controlName === 'btnGoogleEarth');
  const googleEarth = googleEarthControls[0];
  if (googleEarthControls.length !== 1 || googleEarth.type !== 'CommandButton' ||
      !('ControlTipText' in googleEarth.properties) || googleEarth.properties.ControlTipText?.value !== 'Plot on Google Earth' ||
      googleEarth.events?.length !== 1 || googleEarth.events[0].property !== 'OnClick' ||
      googleEarth.events[0].binding !== '[Event Procedure]' || googleEarth.events[0].procedure !== 'btnGoogleEarth_Click' ||
      !googleEarth.events[0].resolved) {
    throw new Error('Two-page Google Earth lost its exact source control/tooltip/callback.');
  }
  const googleEarthAction = { controlId: googleEarth.controlId, caption: googleEarth.properties.ControlTipText.value };
  const vegetationTableControls = source.controls.filter(control => control.controlName === 'btnViewVegTable');
  const vegetationTable = vegetationTableControls[0];
  if (vegetationTableControls.length !== 1 || vegetationTable.type !== 'CommandButton' ||
      !('Caption' in vegetationTable.properties) || vegetationTable.properties.Caption?.value !== 'View Veg Table' ||
      vegetationTable.events?.length !== 1 || vegetationTable.events[0].property !== 'OnClick' ||
      vegetationTable.events[0].binding !== '[Event Procedure]' || vegetationTable.events[0].procedure !== 'btnViewVegTable_Click' ||
      !vegetationTable.events[0].resolved) {
    throw new Error('Two-page vegetation table lost its exact source control/callback.');
  }
  const vegetationTableAction = { controlId: vegetationTable.controlId, caption: vegetationTable.properties.Caption.value };
  const environmentTableControls = source.controls.filter(control => control.controlName === 'btnViewEnvTable');
  const environmentTable = environmentTableControls[0];
  if (environmentTableControls.length !== 1 || environmentTable.type !== 'CommandButton' ||
      !('Caption' in environmentTable.properties) || environmentTable.properties.Caption?.value !== 'View Env Table' ||
      environmentTable.events?.length !== 1 || environmentTable.events[0].property !== 'OnClick' ||
      environmentTable.events[0].binding !== '[Event Procedure]' || environmentTable.events[0].procedure !== 'btnViewEnvTable_Click' ||
      !environmentTable.events[0].resolved) {
    throw new Error('Two-page environment table lost its exact source control/callback.');
  }
  const environmentTableAction = { controlId: environmentTable.controlId, caption: environmentTable.properties.Caption.value };
  const metadataControls = source.controls.filter(control => control.controlName === 'btnLoadMetadata');
  const metadata = metadataControls[0];
  const metadataCaption = form === 'FS882-8x6XL-CHARS' ? 'Edit Metadata' : 'Edit Project Metadata';
  if (metadataControls.length !== 1 || metadata.type !== 'CommandButton' ||
      !('Caption' in metadata.properties) || metadata.properties.Caption?.value !== metadataCaption ||
      metadata.events?.length !== 1 || metadata.events[0].property !== 'OnClick' ||
      metadata.events[0].binding !== '[Event Procedure]' || metadata.events[0].procedure !== 'btnLoadMetadata_Click' ||
      !metadata.events[0].resolved) {
    throw new Error('Two-page project metadata lost its exact source control/caption/callback.');
  }
  const metadataAction = { controlId: metadata.controlId, caption: metadataCaption };
  const profilingControls = source.controls.filter(control => control.controlName === 'btnVegProfile');
  const profiling = profilingControls[0];
  if (profilingControls.length !== 1 || profiling.type !== 'CommandButton' ||
      !('Caption' in profiling.properties) || profiling.properties.Caption?.value !== 'Plot Profiling' ||
      profiling.events?.length !== 1 || profiling.events[0].property !== 'OnClick' ||
      profiling.events[0].binding !== '[Event Procedure]' || profiling.events[0].procedure !== 'btnVegProfile_Click' ||
      !profiling.events[0].resolved) {
    throw new Error('Two-page Plot Profiling lost its exact source control/caption/callback.');
  }
  const profilingAction = { controlId: profiling.controlId, caption: profiling.properties.Caption.value };
  const root = xl.forms.find(source => source.name === xl.root);
  if (!root) throw new Error('Accepted XL source closure is unavailable.');
  const xlColumns = new Set(root.fields.flatMap(field => field.binding ? [field.binding] : []));
  const common = twoPageParentCommonFields(form);
  const extra = twoPageParentFields(form);
  const fields = new Map<string, TwoPageEntryField>();
  for (const control of source.fields) {
    if (!control.binding) continue;
    const column = control.binding;
    if (control.column !== column) throw new Error(`Two-page ${column} lost its exact source column.`);
    const commonField = common.find(field => field.column === column);
    const extraField = extra.find(field => field.column === column);
    const scopes = Number(xlColumns.has(column)) + Number(Boolean(commonField)) + Number(Boolean(extraField));
    if (scopes !== 1) throw new Error(`Two-page ${column} requires exactly one accepted field owner.`);
    const scope: TwoPageEntryScope = xlColumns.has(column) ? 'xl' : commonField ? 'common' : 'extra';
    const existing = fields.get(column);
    if (existing) {
      existing.sourceInstances.push(control.controlId);
      if (!('pageId' in control) || !control.pageId) {
        existing.page = null;
        existing.controlId = control.controlId;
      }
      continue;
    }
    fields.set(column, { column, scope, sourceInstances: [control.controlId], controlId: control.controlId,
      page: 'pageId' in control ? control.pageId ?? null : null });
  }
  const chars = form === 'FS882-8x6XL-CHARS';
  const expected = chars ? { xl: 98, common: 14, extra: 8 } : { xl: 96, common: 13, extra: 9 };
  for (const [scope, count] of Object.entries(expected)) {
    if ([...fields.values()].filter(field => field.scope === scope).length !== count) {
      throw new Error(`Two-page ${scope} scope no longer matches its accepted source boundary.`);
    }
  }
  const header = [...fields.values()].filter(field => field.page === null);
  if (header.length !== 1 || header[0].column !== 'PlotNumber') {
    throw new Error('Two-page entry requires its source header PlotNumber identity.');
  }
  const byID = new Map(source.controls.map(control => [control.controlId, control]));
  const boundOptions = new Set(source.fields.filter(control => control.type === 'OptionGroup' && control.binding)
    .map(control => control.controlId));
  function ownedOption(controlId: string): boolean {
    let current = byID.get(controlId);
    const visited = new Set<string>();
    while (current && 'parentId' in current && current.parentId) {
      if (visited.has(current.controlId)) throw new Error('Cycle in two-page option containment.');
      visited.add(current.controlId);
      if (boundOptions.has(current.parentId)) return true;
      current = byID.get(current.parentId);
    }
    return false;
  }
  const pages = ['Site/Veg', 'Soil/Terrain'].map(name => {
    if (source.pages.find(page => page.name === name)?.controlId !== `form:${form}/${name}`) {
      throw new Error('Two-page page identity changed.');
    }
    const paper = sourcePaperPage(source, name, 'initial', coordinateMode);
    const logical = [...fields.values()].filter(field => field.page === `form:${form}/${name}`);
    const seen = new Set<string>();
    const controls = paper.controls.filter(control => {
      if (ownedOption(control.controlId)) return false;
      if (!control.column) return true;
      const field = fields.get(control.column);
      if (!field) throw new Error(`Two-page visible field ${control.column} is outside its exact binding scope.`);
      if (field.page !== `form:${form}/${name}` || seen.has(control.column)) return false;
      seen.add(control.column);
      return true;
    }).map(control => {
      const labelled = common.find(field => field.column === control.column) ?? extra.find(field => field.column === control.column);
      return structuredClone(labelled ? { ...control, caption: labelled.label } : control);
    });
    return { name, ...paper, controls, fields: logical };
  });
  if (pages.reduce((count, page) => count + page.fields.length, header.length) !== fields.size) {
    throw new Error('Two-page field has no exact source header/page placement.');
  }
  const childForms = [chars ? 'SubVegAXL' : 'SubVegAXL_BC', 'SubVegCXL', 'SubVegDXL',
    'frmVPicsXL', 'SoilHumusXL', 'SoilMineralXL'];
  if (source.embedded.length !== childForms.length) throw new Error('Two-page entry requires all six source child links.');
  const children = source.embedded.map((child, index) => {
    if (!child.resolved || child.form !== childForms[index] || child.masterFields?.length !== 1 ||
        child.childFields?.length !== 1 || child.masterFields[0] !== 'PlotNumber' || child.childFields[0] !== 'PlotNumber') {
      throw new Error('Two-page child identity or exact PlotNumber link changed.');
    }
    const control = source.controls.find(control => control.controlId === child.controlId);
    const page = control && 'pageId' in control ? control.pageId : null;
    if (!page || !pages.some(candidate => page === `form:${form}/${candidate.name}`)) {
      throw new Error('Two-page child lost its source page relationship.');
    }
    return structuredClone({ ...child, page,
      availability: child.form === 'frmVPicsXL' ? 'unavailable' as const : 'accepted-xl-child' as const });
  });
  return { form, source: source.source, sha256: source.sha256, reverseAction, auditAction, googleEarthAction, vegetationTableAction, environmentTableAction, metadataAction, profilingAction, header, pages,
    fields: [...fields.values()], children };
}
