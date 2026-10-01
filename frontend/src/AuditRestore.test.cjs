const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const ts = require('typescript');
const { parse, compile } = require('svelte/compiler');

const source = readFileSync(path.join(__dirname, 'AuditRestore.svelte'), 'utf8');
const code = source.match(/<script lang="ts" module>([\s\S]*?)<\/script>/)[1];
const exportsObject = {};
vm.runInNewContext(ts.transpileModule(code, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 }
}).outputText, { exports: exportsObject });
const { exactAuditRowId, auditRestoreReason } = exportsObject;
const row = (overrides = {}) => ({
  rowId: '9007199254740993', project: 'Sample', plotNumber: 'RESTUI1', table: '_Env',
  editField: 'Elevation', id: null, ...overrides
});
const caps = {
  header: { plotNumber: true, elevation: true, officeNotes: true, slope: true, microtopType: true, speciesListComplete: true },
  children: {
    Veg: { id: true, plotNumber: true, height1: true, cover1: true },
    Humus: { id: true, plotNumber: true, ph: true },
    Other: { id: true, plotNumber: true, userFlag1: true }
  }
};
const reason = (entry, capabilities = caps) => auditRestoreReason(entry, 'Sample', 'RESTUI1', capabilities);

test('audit identity is canonical signed64 text without float conversion', () => {
  for (const id of ['9007199254740993', '9007199254740994', '9223372036854775807', '-9223372036854775808', '0']) assert.equal(exactAuditRowId(id), true);
  for (const id of ['9007199254740993.0', '01', '-0', '+1', ' 1', '1e3', '9223372036854775808', '-9223372036854775809']) assert.equal(exactAuditRowId(id), false);
});
test('header aliases and actual Admin table mapping are supported', () => {
  for (const entry of [row(), row({ editField: 'SlopeGradient' }), row({ editField: 'SurfaceTopographyType' }),
    row({ editField: 'SpeciesListComplete' }), row({ table: 'Sample_Admin', editField: 'OfficeNotes' })]) assert.equal(reason(entry), null);
  assert.match(reason(row({ editField: 'OfficeNotes' })), /parent table/);
  assert.match(reason(row({ table: '_Admin', editField: 'Elevation' })), /parent table/);
});
test('child NULL/boolean/height fields use mapped capabilities and exact child identities', () => {
  for (const entry of [row({ table: '_Humus', editField: 'HumusFormpH', id: 0 }),
    row({ table: '_Other', editField: 'UserFlag1', id: -2147483648 }),
    row({ table: '_Veg', editField: 'Height1', id: 2147483647 })]) assert.equal(reason(entry), null);
  for (const id of [null, 1.25, 2147483648, -2147483649]) assert.match(reason(row({ table: '_Veg', editField: 'Height1', id })), /child ID/);
});
test('unsupported covers, foreign ownership, identity fields and missing schemas fail explicitly', () => {
  assert.match(reason(row({ table: '_Veg', editField: 'Cover1', id: 1 })), /unsupported.*not be pruned/);
  for (const editField of ['ID', 'Plot', 'PlotNumber', 'Locked']) assert.match(reason(row({ editField })), /cannot be restored/);
  assert.match(reason(row({ project: 'Other' })), /another project/);
  assert.match(reason(row({ plotNumber: 'OTHER' })), /another project/);
  assert.match(reason(row({ table: 'Other_Env' })), /Unsupported audit table/);
  assert.match(reason(row({ editField: 'Unmapped' })), /active project schema/);
  assert.match(reason(row(), { header: {}, children: {} }), /identity columns/);
});
test('component compiles with accessible native dialog and explicit actions', () => {
  const result = compile(source, { filename: 'AuditRestore.svelte', generate: 'client' });
  assert.equal(result.warnings.filter(item => item.code.startsWith('a11y')).length, 0);
  assert.ok(parse(source).html);
  assert.match(source, /dialog\.showModal\(\)/);
  assert.match(source, /aria-labelledby="audit-confirm-title"/);
  assert.match(source, /selected = \[\]/);
  assert.doesNotMatch(source, /Number\(.*rowId|parseInt\(.*rowId/);
});
test('form gates restoration and refreshes both header and child revision after all outcomes', () => {
  const form = readFileSync(path.join(__dirname, 'FS882Form.svelte'), 'utf8');
  assert.match(form, /VITE_AUDIT_RESTORE !== 'false'/);
  assert.match(form, /auditBlocked = \$derived\(childEditingDisabled \|\| Object\.keys\(headerValidation\)\.length > 0\)/);
  assert.match(form, /await PlotService\.SetAuditRestoreSelection\(plot, \[\]\)/);
  assert.match(form, /await refreshAfterRestore\(plot, request\)/);
  assert.match(form, /const locked = draft\.locked/);
  assert.match(form, /cleanedVegRows !== 0/);
});
