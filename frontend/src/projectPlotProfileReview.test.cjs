const assert = require('node:assert/strict');
const {test} = require('node:test');
const {readFileSync} = require('node:fs');
const path = require('node:path');
const {compile} = require('svelte/compiler');
const {loadTypeScript} = require('./svelteTestHelpers.cjs');
const read = name => readFileSync(path.join(__dirname,name),'utf8');
const quality = loadTypeScript('qualityEditor.ts', {'./becEditor':loadTypeScript('becEditor.ts')});
const resource = name => JSON.parse(readFileSync(path.join(__dirname,'..','..','resources',name),'utf8'));
const metadata = loadTypeScript('projectMetadataEditor.ts', {
  './qualityEditor':quality,
  '../../resources/project-metadata-standard.json':resource('project-metadata-standard.json'),
  '../../resources/project-metadata-template.json':resource('project-metadata-template.json')
});
const profile = loadTypeScript('projectPlotProfileReview.ts', {'./projectMetadataEditor':metadata});
const cell = (storage,value) => ({storage,text:null,integer:null,real:null,blobHex:null,
  ...(storage==='null'?{}:{[storage==='blob'?'blobHex':storage]:value})});
const columns = Array.from(profile.plotProfileFields,name=>({name,declaredType:'TEXT'}));
const row = id => ({rowId:id,cells:columns.map(()=>cell('null'))});
const review = () => ({project:'Sample',table:'Sample_Profile',rules:{columns,rows:[row('-1'),row('2')]},
  descriptions:{columns:[{name:'table_name',declaredType:'TEXT'},{name:'description',declaredType:'TEXT'}],
    rows:[{rowId:'1',cells:[cell('text','Sample_Profile'),cell('text','VP05-2')]}]}});

test('Profile typed labels distinguish NULL, empty, whitespace, literal operations and exact stored counts without coercion',()=>{
  for(const [source,label] of [
    [cell('null'),'NULL'],[cell('text',''),'text: ""'],[cell('text','  Add plots  '),'text: "  Add plots  "'],
    [cell('integer','9007199254740993'),'integer: 9007199254740993'],[cell('integer','-1'),'integer: -1'],
    [cell('real',1.25),'real: 1.25'],[cell('blob','00ff'),'blob: hex:00ff']
  ]) assert.equal(profile.profileCellLabel(source),label);
  for(const source of [cell('integer',1),cell('integer','1.0'),cell('real',Infinity),cell('blob','fff'),
    cell('text',null),cell('unknown',''),{...cell('null'),text:''},{...cell('text','x'),integer:'1'}]) {
    assert.throws(()=>profile.profileCellLabel(source),/inconsistent typed/);
  }
});

test('Readonly profile review preserves physical duplicate Orders and historical columns/descriptions without implicit execution',()=>{
  const source=review();
  source.rules.columns=[...columns,{name:'HistoricalExtra',declaredType:''}];
  for(const record of source.rules.rows) record.cells.push(cell('blob',''));
  source.descriptions.rows.push({rowId:'2',cells:[cell('text','Sample_Profile'),cell('null')]},
    {rowId:'3',cells:[cell('text','Sample_Profile'),cell('text','')]});
  assert.equal(JSON.stringify(profile.validateProjectPlotProfileReview(source)),JSON.stringify(source));
  source.rules.rows=[];
  source.descriptions.rows=[];
  assert.equal(JSON.stringify(profile.validateProjectPlotProfileReview(source)),JSON.stringify(source));
});

test('Profile review rejects mismatched ownership, incomplete schemas, inconsistent rows and ambiguous physical identities',()=>{
  const variants=[
    {...review(),table:'Other_Profile'},
    {...review(),rules:{columns:columns.slice(1),rows:[]}},
    {...review(),rules:{columns:[...columns,columns[0]],rows:[]}},
    {...review(),rules:{columns,rows:[row('1'),row('1')]}},
    {...review(),rules:{columns,rows:[{...row('1'),cells:[]}]}},
    {...review(),rules:{columns,rows:[row('1.5')]}},
    {...review(),descriptions:{columns:[],rows:[]}}
  ];
  for(const source of variants) assert.throws(()=>profile.validateProjectPlotProfileReview(source));
});

test('Project-local profile review is separately gated, resident across tabs, cancellable and strictly read-only',()=>{
  const component=read('ProjectPlotProfileReview.svelte');
  const form=read('FS882Form.svelte');
  assert.equal(compile(component,{filename:'ProjectPlotProfileReview.svelte',generate:'client'}).warnings.length,0);
  assert.match(form,/VITE_PROJECT_PLOT_PROFILE_REVIEW === 'true'/);
  assert.match(form,/headerWorkflowBusy = \$derived\([^;]*profileReviewBusy/);
  assert.match(form,/\{#if profileReviewOpen\}\s*<ProjectPlotProfileReview/);
  assert.match(component,/reads\.track\(client\.ReviewProjectPlotProfile\(\)\)/);
  assert.match(component,/generation\+\+; reads\.cancelAll\(\)/);
  assert.match(component,/review = null; error = null; reading = true/);
  assert.match(component,/request === generation/);
  assert.match(component,/no rules, counts or filters changed/);
  assert.match(component,/disabled title="Ordered profile execution is not implemented."/);
  assert.match(component,/PlotCount is historical storage/);
  assert.match(component,/aria-label=\{`\$\{item\.table\.columns\[index\]\.name\}, record \$\{row\.rowId\}`\}/);
  assert.doesNotMatch(component,/<input|<select|<textarea|\.Save|\.Create|\.Update|\.Delete|\.Set|\.Run|\.Restore/);
});
