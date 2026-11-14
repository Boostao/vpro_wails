const { cell } = require('./siviParentTestHelpers.cjs');
const defaults = require('../../resources/sivi-new-row-defaults.json');
const owner = { contextId: 'C', project: 'Sample', plot: 'P' };
const token = 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa';
const copy = value => structuredClone(value);
function original() {
  const names = [...defaults.initialNullColumns, 'Flag', 'ID'];
  return { ...owner, form: 'SubVegA-SIVI_BC',
    columns: names.map(name => ({ name, declaredType: name === 'Flag' ? 'BIT' : 'TEXT' })),
    original: { rowId: '9223372036854775807', cells: names.map(name =>
      name === 'PlotNumber' ? cell('text', 'P') : name === 'Species' ? cell('text', '  historical overlength  ') :
      name === 'Flag' ? cell('integer', '1') : name === 'ID' ? cell('integer', '-2147483648') :
      name === 'Cover5a' ? cell('real', 0) : cell()) } };
}
function receipt(plan, lookup = false, strength = 3) {
  const audits = [];
  if (strength === 3) for (const [index, column] of plan.columns.entries()) {
    const value = plan.original.cells[index];
    if (['PlotNumber', 'ID'].includes(column.name) || value.storage === 'null') continue;
    audits.push({ rowId: String(100 + audits.length), project: plan.project, user: ' literal user ',
      plotNumber: plan.plot, table: '_Veg', editField: column.name, editWhen: '2026-10-07 18:30:53',
      beforeEdit: value.storage === 'text' ? value.text : column.name === 'Flag' ? '-1' :
        value.storage === 'integer' ? value.integer : String(value.real),
      afterEdit: null, restore: false, flag: false, id: -2147483648 });
  }
  return { requestId: plan.requestId, ...owner, form: plan.form, rowId: plan.original.rowId, id: -2147483648,
    historyId: plan.requestId, actor: ' literal user ', editWhen: '2026-10-07 18:30:53', auditStrength: strength,
    columns: copy(plan.columns), original: copy(plan.original), request: copy(plan), audits,
    didCommit: !lookup, replayed: lookup };
}
module.exports = { owner, token, original, receipt };
