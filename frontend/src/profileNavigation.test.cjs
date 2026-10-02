const assert = require('node:assert/strict');
const {test} = require('node:test');
const {readFileSync} = require('node:fs');
const path = require('node:path');
const {loadTypeScript} = require('./svelteTestHelpers.cjs');
const resource = name => JSON.parse(readFileSync(path.join(__dirname,'..','..','resources',name),'utf8'));
const quality = loadTypeScript('qualityEditor.ts',{'./becEditor':loadTypeScript('becEditor.ts')});
const metadata = loadTypeScript('projectMetadataEditor.ts',{'./qualityEditor':quality,
  '../../resources/project-metadata-standard.json':resource('project-metadata-standard.json'),
  '../../resources/project-metadata-template.json':resource('project-metadata-template.json')});
const profile = loadTypeScript('projectPlotProfileReview.ts',{'./projectMetadataEditor':metadata});
const profileFile = loadTypeScript('profileFile.ts',{'./projectPlotProfileReview':profile});
const profileTable = loadTypeScript('profileTable.ts',{'./profileFile':profileFile,'./projectPlotProfileReview':profile});
const read = name => readFileSync(path.join(__dirname,name),'utf8');
const nullCell = () => ({storage:'null',text:null,integer:null,real:null,blobHex:null});
const rules = {columns:Array.from(profile.plotProfileFields,name=>({name,declaredType:'TEXT'})),
  rows:['1','2'].map(rowId=>({rowId,cells:Array.from({length:9},nullCell)}))};
const preview = {project:'Sample',table:'Sample_Profile',su:'None',totalPlots:4,plotNumbers:[" A' # ",'Z'],
  steps:[{rowId:'1',order:1,operation:'Add plots',plotCount:4,remaining:4},
    {rowId:'2',order:2,operation:'Subtract plots',plotCount:2,remaining:2}]};
const proposal = {input:{originalRules:rules,projectLump:null,subvarieties:false},preview};
const summary = plotNumber => ({plotNumber,fieldNumber:null,plotRepresenting:null,zone:null,subZone:null,siteSeries:null});
const navigation = () => ({contextId:'owned',result:structuredClone(preview),plots:preview.plotNumbers.map(summary)});

test('Scoped navigation preserves exact literal membership and paginates one immutable approved recordset',()=>{
  const source=profile.validateProfileNavigation(navigation(),proposal,'owned');
  const first=profile.profileNavigationPage(source,'owned',0,1);
  const second=profile.profileNavigationPage(source,'owned',1,1);
  assert.equal(first.total,2); assert.equal(first.plots[0].plotNumber," A' # ");
  assert.equal(second.total,2); assert.equal(second.plots[0].plotNumber,'Z');
  assert.equal(profile.profileNavigationPage(source,'owned',2,1).plots.length,0);
  for(const [id,offset,limit] of [['stale',0,1],['owned',-1,1],['owned',0,201],['owned',0,0],['owned',.5,1]]) {
    assert.throws(()=>profile.profileNavigationPage(source,id,offset,limit),/stale or out of range/);
  }
});

test('Navigation rejects foreign scopes, inconsistent counts, normalization, duplicate summaries and missing fields',()=>{
  for(const alter of [
    value=>value.contextId='stale',value=>value.result.su='Other',value=>value.result.totalPlots=5,
    value=>value.result.steps[0].plotCount=3,value=>value.result.plotNumbers[0]="A' #",
    value=>value.plots[0].plotNumber="A' #",value=>value.plots.reverse(),
    value=>value.plots[1]=value.plots[0],value=>value.plots=null,
    value=>delete value.plots[0].fieldNumber,value=>value.plots[0].zone=5
  ]) {
    const value=navigation(); alter(value);
    assert.throws(()=>profile.validateProfileNavigation(value,proposal,'owned'));
  }
});

test('Zero and over200 previews stay unavailable without clearing an existing filter; exact200 remains supported',()=>{
  for(const [count,unavailable] of [[0,true],[1,false],[200,false],[201,true]]) {
    const result={...preview,plotNumbers:Array.from({length:count},(_,index)=>String(index))};
    assert.equal(profile.profileFilterUnavailable(result)!==null,unavailable);
  }
});

test('Apply/Clear/Previous/Next reuse transition ownership and revalidation before publishing, with context retirement and a fresh plot mount',()=>{
  const app=read('App.svelte'),panel=read('ProjectPlotProfileReview.svelte'),form=read('FS882Form.svelte');
  assert.match(form,/VITE_PROJECT_PLOT_PROFILE_FILTERING === 'true'/);
  assert.match(panel,/allowFiltering = false/);
  assert.match(panel,/disabled=\{busy \|\| blocked \|\| profileFilterUnavailable\(result\) !== null\}/);
  assert.match(panel,/runInput = \$state\.snapshot\(input\)/);
  assert.match(panel,/structuredClone\(\$state\.snapshot\(\{ input: runInput, preview: result \}\)\)/);
  assert.match(app,/onProfileNavigation=\{applyProfileNavigation\}/);
  for(const name of ['applyProfileNavigation','clearProfileNavigation','navigateProfilePlot']) {
    const start=app.indexOf(`async function ${name}`);
    assert.ok(start>=0);
    assert.match(app.slice(start,app.indexOf('\n  async function ',start+1)),/await requestTransition/);
  }
  assert.match(app,/const next = await resolveNavigation\(proposal, contextId\);[\s\S]*profileNavigation = next; page = nextPage/);
  assert.match(app,/const next = await resolveNavigation\(source\.proposal, contextId\);[\s\S]*editorPlotNumber = target/);
  assert.match(app,/const next = source \? await resolveNavigation\(source\.proposal, source\.contextId\)/);
  assert.match(app,/if \(profileNavigation && profileNavigation\.contextId !== state\.contextId\) profileNavigation = null/);
  assert.match(app,/projectState\.set\(state\);\s*profileNavigation = null/);
  assert.match(app,/\{#key \$projectState\?\.contextId\}\s*\{#key editorPlotNumber\}/);
  assert.match(app,/<fieldset disabled=\{busy\} class="contents">/);
  assert.match(app,/await tick\(\);\s*const next = await resolveNavigation/);
  assert.doesNotMatch(app,/SetProfileFilter|CreateSU|UpdatePlotCount/);
});

test('Independent profile write authorization is default-off and shared-transition owned, not inferred from selection',()=>{
  const app=read('App.svelte');
  assert.match(app,/VITE_PLOT_PROFILE_WRITE_OWNERSHIP === 'true'/);
  assert.match(app,/aria-label="Reviewed profile write ownership"/);
  assert.match(app,/Context changes or restart clear it/);
  assert.match(app,/profileWriteContext !== \$projectState.contextId/);
  const start=app.indexOf('async function publishProfileWriteOwnership');
  const action=app.slice(start,app.indexOf('\n  async function ',start+1));
  assert.match(action,/\$state\.snapshot\(\{ review: profileWriteReview, enabled: !profileWriteReview.source.writable, confirmed: true \}\)/);
  assert.match(action,/await requestTransition/);
  assert.match(action,/contextId !== origin/);
  assert.match(action,/await tick\(\);\s*const state = await ContextService.SetPlotProfileEditing/);
  assert.match(action,/projectState\.set\(state\); profileNavigation = null/);
  assert.match(action,/view = 'plots'/);
  assert.match(action,/do not replay/);
  assert.doesNotMatch(action,/SwitchContext|SelectPlotProfile|SaveProjectPlotProfile/);
});

test('Write ownership review preserves exact physical source, typed rules and NULL/empty descriptions',()=>{
  const source={source:{name:" Other' # ",path:"C:\\profile O'Brien #.db"},table:" Other' # _Profile",available:true,writable:false,reason:''};
  const review={project:'Sample',table:source.table,rules:structuredClone(rules),
    descriptions:{columns:[{name:'table_name',declaredType:'TEXT'},{name:'description',declaredType:'TEXT'}],
      rows:[{rowId:'1',cells:[{storage:'text',text:source.table},nullCell()]},
        {rowId:'2',cells:[{storage:'text',text:source.table},{storage:'text',text:''}]}]},source};
  const result=profile.validateProjectPlotProfileReview(review);
  assert.equal(result.source.source.name,source.source.name);
  assert.equal(result.descriptions.rows[0].cells[1].storage,'null');
  assert.equal(result.descriptions.rows[1].cells[1].text,'');
  for(const alter of [value=>value.table='Sample_Profile',value=>value.source.writable='true',
    value=>value.rules.rows[1].rowId=value.rules.rows[0].rowId,value=>delete value.descriptions.columns]){
    const value=structuredClone(review);alter(value);
    assert.throws(()=>profile.validateProjectPlotProfileReview(value));
  }
});

test('Genuine absent description metadata differs from an empty physical metadata table or malformed response',()=>{
  const source={source:{name:'Bare',path:'C:\\bare.db'},table:'Bare_Profile',available:true,writable:false,reason:''};
  const review={project:'Sample',table:source.table,rules:structuredClone(rules),
    descriptions:{columns:[],rows:[]},source};
  assert.deepEqual(JSON.parse(JSON.stringify(profile.validateProjectPlotProfileReview(review).descriptions)),{columns:[],rows:[]});
  review.descriptions={columns:[{name:'table_name',declaredType:'TEXT'},{name:'description',declaredType:'TEXT'}],rows:[]};
  assert.equal(profile.validateProjectPlotProfileReview(review).descriptions.columns.length,2);
  for(const descriptions of [null,{columns:null,rows:[]},{columns:[],rows:null},
    {columns:[],rows:[{rowId:'1',cells:[]}]},{columns:[{name:'table_name',declaredType:'TEXT'}],rows:[]}]){
    assert.throws(()=>profile.validateProjectPlotProfileReview({...review,descriptions}));
  }
});

test('Blank profile-file creation preserves the exact original template and rejects partial or populated proposals',()=>{
  const review={metadataAbsent:true,template:{columns:profile.plotProfileFields.map((name,index)=>({
    name,declaredType:index===0?'SMALLINT':index===8?'INTEGER':'VARCHAR'})),rows:[]}};
  assert.equal(profileFile.validateProfileFileReview(review).template.columns.length,9);
  for(const alter of [value=>delete value.metadataAbsent,value=>value.metadataAbsent=false,
    value=>value.template.rows=[rules.rows[0]],value=>value.template.columns.reverse(),
    value=>value.template.columns[0].declaredType='INTEGER',value=>value.template.columns.pop()]){
    const invalid=structuredClone(review);alter(invalid);
    assert.throws(()=>profileFile.validateProfileFileReview(invalid));
  }
  const result={source:{name:'NewBlank',path:"C:\\new O'Brien #.db"},table:'NewBlank_Profile',ruleCount:0};
  assert.equal(profileFile.validateProfileFileCreated(result,'NewBlank'),result);
  for(const value of [{...result,ruleCount:1},{...result,table:'Other_Profile'},{...result,source:{name:'NewBlank',path:''}}])
    assert.throws(()=>profileFile.validateProfileFileCreated(value,'NewBlank'),/do not replay/);
});

test('New profile-file publication is separately default-off, shared-transition owned and never an implicit attach or grant',()=>{
  const app=read('App.svelte');
  assert.match(app,/VITE_PLOT_PROFILE_FILE_CREATION === 'true'/);
  assert.match(app,/aria-label="Reviewed blank profile file creation"/);
  assert.match(app,/<label for="profile-file-name"/);
  assert.match(app,/<label for="profile-file-path"/);
  const start=app.indexOf('async function createBlankProfileFile');
  const action=app.slice(start,app.indexOf('\n  async function ',start+1));
  assert.match(action,/\$state\.snapshot\(\{ review: profileFileReview, name: profileFileName, path: profileFilePath, confirmed: true \}\)/);
  assert.match(action,/await requestTransition/);
  assert.match(action,/await tick\(\);\s*const response = await ContextService.CreatePlotProfileFile/);
  assert.match(action,/profileFileCommitted = true/);
  assert.match(action,/do not replay creation/);
  assert.match(action,/view = 'plots'; editorPlotNumber = undefined/);
  assert.doesNotMatch(action,/SetPlotProfileEditing|SelectPlotProfile|SwitchContext/);
  assert.match(app,/profileFileCommitted \|\| !profileFileName \|\| !profileFilePath \|\| profileFileContext !== \$projectState.contextId/);
});

test('Existing-file profile creation requires exact original template and current selected-file writer ownership',()=>{
  const source={source:{name:'Owned',path:"C:\\owned O'Brien #.db"},table:'Owned_Profile',available:true,writable:true,reason:''};
  const review={template:{metadataAbsent:true,template:{columns:profile.plotProfileFields.map((name,index)=>({
    name,declaredType:index===0?'SMALLINT':index===8?'INTEGER':'VARCHAR'})),rows:[]}},
    profile:{project:'Sample',table:source.table,rules:structuredClone(rules),descriptions:{columns:[],rows:[]},source}};
  assert.equal(profileTable.validateProfileTableReview(review).profile.source.source.path,source.source.path);
  for(const alter of [value=>value.profile.source.writable=false,value=>value.profile.source.available=false,
    value=>value.profile.table='Foreign_Profile',value=>value.template.metadataAbsent=false,
    value=>value.profile.descriptions=null,value=>value.template.template.columns.pop()]){
    const invalid=structuredClone(review);alter(invalid);
    assert.throws(()=>profileTable.validateProfileTableReview(invalid));
  }
  const result={source:{name:'New',path:source.source.path},table:'New_Profile',ruleCount:0};
  assert.equal(profileTable.validateProfileTableCreated(result,'New',source.source.path),result);
  for(const response of [{...result,ruleCount:1},{...result,table:'Foreign_Profile'},
    {...result,source:{name:'New',path:'C:\\another.db'}}]){
    assert.throws(()=>profileTable.validateProfileTableCreated(response,'New',source.source.path),/do not replay/);
  }
});

test('Existing-file creation is separately gated inside profile selection, shares draft transitions and never switches or grants',()=>{
  const app=read('App.svelte');
  assert.match(app,/VITE_PLOT_PROFILE_TABLE_CREATION === 'true'/);
  assert.match(app,/\{#if profileTableCreationEnabled && \$projectState.plotProfile.writable\}/);
  const panel=app.indexOf('aria-label="Reviewed existing-file profile creation"');
  assert.ok(panel>app.indexOf('aria-label="Stored plot profile selection"'));
  assert.match(app,/<label for="profile-table-name"/);
  assert.match(app,/profileTableReview.profile.source.source.path/);
  const start=app.indexOf('async function createBlankProfileTable');
  const action=app.slice(start,app.indexOf('\n  async function ',start+1));
  assert.match(action,/\$state\.snapshot\(\{ review: profileTableReview, name: profileTableName, confirmed: true \}\)/);
  assert.match(action,/await requestTransition/);
  assert.match(action,/contextId !== origin/);
  assert.match(action,/await tick\(\);\s*const response = await ContextService.CreatePlotProfileTable/);
  assert.match(action,/profileTableCommitted = true/);
  assert.match(action,/do not replay creation/);
  assert.match(action,/view = 'plots'; editorPlotNumber = undefined/);
  assert.doesNotMatch(action,/SetPlotProfileEditing|SelectPlotProfile|SwitchContext/);
});
