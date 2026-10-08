const { loadTypeScript, serverComponent } = require('./svelteTestHelpers.cjs');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const source = require('../../resources/fs1333-sivi-layout.json');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const metadata = loadTypeScript('projectMetadataEditor.ts', {
  './qualityEditor': quality, '../../resources/project-metadata-standard.json': [],
  '../../resources/project-metadata-template.json': [],
});
const restoration = loadTypeScript('projectMetadataRestore.ts', {
  './qualityEditor': quality, './projectMetadataEditor': metadata,
});
const editor = loadTypeScript('siviParentEditor.ts', {
  '../../resources/fs1333-sivi-layout.json': source, './projectMetadataRestore': restoration,
  './projectMetadataEditor': metadata, './qualityEditor': quality, './numericEditor': loadTypeScript('numericEditor.ts'),
});
const { SIVIParentSession } = loadTypeScript('siviParentSession.ts', {
  './siviParentEditor': editor, './qualityEditor': quality,
});
const transport = loadTypeScript('siviParentTransport.ts', {
  './siviParentEditor': editor, './siviParentSession': { SIVIParentSession },
  './projectMetadataRestore': restoration,
});
const writeSession = loadTypeScript('siviParentWriteSession.ts', {
  './siviParentEditor': editor, './siviParentTransport': transport, './projectMetadataRestore': restoration,
});
const actionWriteSession = loadTypeScript('siviParentActionWriteSession.ts', {
  './siviParentEditor': editor, './siviParentWriteSession': writeSession,
});
const sourceSession = loadTypeScript('siviParentSourceSession.ts', {
  './siviParentTransport': transport, './projectMetadataRestore': restoration,
});
const assignmentSession = loadTypeScript('siviProjectAssignmentSession.ts', {
  './siviParentEditor': editor, './siviParentWriteSession': writeSession, './siviParentTransport': transport,
  './siviParentSourceSession': sourceSession, './projectMetadataEditor': metadata, './qualityEditor': quality,
});
const assignmentField = serverComponent(readFileSync(path.join(__dirname, 'SIVIProjectAssignmentField.svelte'), 'utf8'),
  'SIVIProjectAssignmentField.svelte', {
    './siviProjectAssignmentSession': assignmentSession, './projectMetadataEditor': metadata,
  });
const cell = (storage = 'null', value = null) => ({
  storage, text: storage === 'text' ? value : null, integer: storage === 'integer' ? value : null,
  real: storage === 'real' ? value : null, blobHex: storage === 'blob' ? value : null,
});
function original() {
  const review = { ContextID: 'context:owned', Project: 'Project', Plot: 'P', Form: 'frmSIVIsite',
    Query: 'USysEnv', Membership: 'literal-binary-inner-pairs', EnvTable: 'Project_Env', AdminTable: 'Project_Admin',
    EnvColumns: [], AdminColumns: [{ name: 'Plot', declaredType: 'TEXT' }], Bindings: [],
    Rows: [{ Env: { rowId: '9007199254740993', cells: [] },
      Admin: { rowId: '-9223372036854775808', cells: [cell('text', 'P')] } }] };
  for (const field of source.forms[0].fields.filter(field => field.binding)) {
    const owner = ['Plot', 'PlotType', 'StrataCoverTotal'].includes(field.binding) ? 'Admin' : 'Env';
    const columns = review[`${owner}Columns`];
    review.Bindings.push({ ControlID: field.controlId, Binding: field.binding, Table: `Project_${owner}`,
      Column: columns.length, Implicit: false });
    columns.push({ name: field.binding, declaredType: 'TEXT' });
    review.Rows[0][owner].cells.push(['PlotNumber', 'Plot'].includes(field.binding) ? cell('text', 'P') : cell());
  }
  review.Bindings.push({ ControlID: '', Binding: 'SpeciesListComplete', Table: 'Project_Env',
    Column: review.EnvColumns.length, Implicit: true });
  review.EnvColumns.push({ name: 'SpeciesListComplete', declaredType: 'BOOLEAN' });
  review.Rows[0].Env.cells.push(cell());
  return review;
}
function setCell(review, column, value) {
  const binding = review.Bindings.find(binding => binding.Binding === column);
  review.Rows[0][binding.Table.endsWith('_Admin') ? 'Admin' : 'Env'].cells[binding.Column] = value;
}
module.exports = { editor, SIVIParentSession, transport, writeSession, actionWriteSession, assignmentSession, assignmentField, sourceSession,
  source, original, cell, setCell, metadata, restoration };
