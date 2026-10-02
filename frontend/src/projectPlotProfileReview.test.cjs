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
  assert.match(component,/review = null; error = null; lump = null; result = null; reading = true/);
  assert.match(component,/request === generation/);
  assert.match(component,/no rules, counts or filters changed/);
  assert.match(component,/disabled title="Ordered profile execution is not implemented."/);
  assert.match(component,/PlotCount is historical storage/);
  assert.match(component,/aria-label=\{`\$\{item\.table\.columns\[index\]\.name\}, record \$\{row\.rowId\}`\}/);
  assert.doesNotMatch(component,/<select|<textarea|\.Save|\.Create|\.Update|\.Delete|\.Set|\.Restore/);
  assert.match(form,/VITE_PROJECT_PLOT_PROFILE_RUN === 'true'/);
  assert.match(component,/if \(!allowRun \|\| reading \|\| !review\) return/);
  assert.match(component,/\{#if allowRun\}[\s\S]*Run stored profile preview[\s\S]*\{:else\}[\s\S]*Run Profile \(unavailable\)/);
  assert.match(component,/originalRules: source\.rules, projectLump: lump, subvarieties/);
  assert.match(component,/disabled=\{reading \|\| !lump\}/);
  assert.match(component,/lump = null; subvarieties = false; result = null; error = null/);
  assert.match(component,/function cancelRun\(\) \{\s*generation\+\+; reads\.cancelAll\(\);/);
  assert.match(component,/running = false; reading = false; result = null; onbusy\(false\)/);
  assert.match(component,/Profile preview cancelled; reviewed inputs retained/);
});

test('Preview validates exact scoped ordered result identities, counts and NULL transport without applying filters',()=>{
  const source=profile.validateProjectPlotProfileReview(review());
  const result={project:'Sample',table:'Sample_Profile',su:'None',totalPlots:4,plotNumbers:['P','Z'],
    steps:[{rowId:'-1',order:1,operation:'Add plots',plotCount:4,remaining:4},
      {rowId:'2',order:2,operation:'Subtract plots',plotCount:2,remaining:2}]};
  assert.equal(JSON.stringify(profile.validateProfileRunResult(result,source)),JSON.stringify(result));
  const bad=[
    {...result,project:'Else'}, {...result,table:'Else_Profile'}, {...result,su:''},
    {...result,plotNumbers:null}, {...result,plotNumbers:['P','P']}, {...result,plotNumbers:['']},
    {...result,totalPlots:1}, {...result,totalPlots:NaN}, {...result,steps:null}, {...result,steps:[]},
    {...result,steps:[result.steps[0],result.steps[0]]},
    {...result,steps:[result.steps[0],{...result.steps[1],order:1}]},
    {...result,steps:[result.steps[0],{...result.steps[1],plotCount:5}]},
    {...result,steps:[result.steps[0],{...result.steps[1],remaining:1}]},
    {...result,steps:[result.steps[0],{...result.steps[1],operation:'Filter'}]}
  ];
  for(const value of bad) assert.throws(()=>profile.validateProfileRunResult(value,source),/inconsistent scoped result/);
  const lump={columns:['LumpCode','SppCode','Use'].map(name=>({name,declaredType:'TEXT'})),rows:[]};
  assert.equal(JSON.stringify(profile.validateProjectProfileLump(lump)),JSON.stringify(lump));
  assert.throws(()=>profile.validateProjectProfileLump({...lump,columns:lump.columns.slice(1)}),/complete original physical schema/);
});
