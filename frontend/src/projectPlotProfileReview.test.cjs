const assert = require('node:assert/strict');
const {test} = require('node:test');
const {readFileSync} = require('node:fs');
const path = require('node:path');
const {compile} = require('svelte/compiler');
const {loadTypeScript, serverComponent} = require('./svelteTestHelpers.cjs');
const {render} = require('svelte/server');
const read = name => readFileSync(path.join(__dirname,name),'utf8');
const quality = loadTypeScript('qualityEditor.ts', {'./becEditor':loadTypeScript('becEditor.ts')});
const resource = name => JSON.parse(readFileSync(path.join(__dirname,'..','..','resources',name),'utf8'));
const metadata = loadTypeScript('projectMetadataEditor.ts', {
  './qualityEditor':quality,
  '../../resources/project-metadata-standard.json':resource('project-metadata-standard.json'),
  '../../resources/project-metadata-template.json':resource('project-metadata-template.json')
});
const profile = loadTypeScript('projectPlotProfileReview.ts', {'./projectMetadataEditor':metadata});
const ruleEditor = loadTypeScript('projectProfileRuleEditor.ts', {'./projectMetadataEditor':metadata,'./projectPlotProfileReview':profile});
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

test('Project-local profile review and editing are separately gated, resident across tabs and preserve independent draft ownership',()=>{
  const component=read('ProjectPlotProfileReview.svelte');
  const form=read('FS882Form.svelte');
  assert.equal(compile(component,{filename:'ProjectPlotProfileReview.svelte',generate:'client'}).warnings.length,0);
  assert.match(form,/VITE_PROJECT_PLOT_PROFILE_REVIEW === 'true'/);
  assert.match(form,/headerWorkflowBusy = \$derived\([^;]*profileReviewBusy/);
  assert.match(form,/\{#if profileReviewOpen\}\s*<ProjectPlotProfileReview/);
  assert.match(component,/reads\.track\(client\.ReviewProjectPlotProfile\(\)\)/);
  assert.match(component,/allowEditing \? reads\.track\(client\.ListProjectPlotProfileChoices\(\)\)/);
  assert.match(component,/generation\+\+; reads\.cancelAll\(\)/);
  assert.match(component,/review = null; error = null; lump = null; result = null; reading = true/);
  assert.match(component,/request === generation/);
  assert.match(component,/no rules, counts or filters changed/);
  assert.match(component,/disabled title="Ordered profile execution is not implemented."/);
  assert.match(component,/PlotCount is historical storage/);
  assert.match(component,/aria-label=\{`\$\{item\.table\.columns\[index\]\.name\}, record \$\{row\.rowId\}`\}/);
  assert.doesNotMatch(component,/\.Update|\.Set|\.Restore/);
  assert.match(form,/VITE_PROJECT_PLOT_PROFILE_EDITING === 'true'/);
  assert.match(component,/\{#if allowEditing && draft\}/);
  assert.match(component,/client\.SaveProjectPlotProfile\(profileRuleEditRequest\(draft\)\)/);
  assert.match(component,/if \(busy \|\| dirty \|\| creation \|\| deletion !== null\)/);
  assert.match(component,/if \(busy \|\| blocked\)/);
  assert.match(component,/committedFailure = true; draft = null/);
  assert.match(component,/completed writes must not be replayed/);
  assert.match(form,/if \(profileReviewBlocked\) return profileEditor\?\.getCloseState/);
  assert.match(form,/if \(profileReviewBlocked\) \{ profileEditor\?\.undo\(\); return; \}/);
  assert.match(form,/ordinary plot Save never writes profile rules/);
  assert.match(form,/VITE_PROJECT_PLOT_PROFILE_RUN === 'true'/);
  assert.match(component,/if \(!allowRun \|\| busy \|\| blocked \|\| !review\) return/);
  assert.match(component,/\{#if allowRun\}[\s\S]*Run stored profile preview[\s\S]*\{:else\}[\s\S]*Run Profile \(unavailable\)/);
  assert.match(component,/originalRules: source\.rules, projectLump: lump, subvarieties/);
  assert.match(component,/disabled=\{busy \|\| blocked \|\| !lump\}/);
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

test('Stored-rule drafts reuse exact metadata scalars, preserve NULL/empty and apply source Table-to-Field proposals explicitly',()=>{
  const source=review();
  const record=source.rules.rows[0];
  record.cells[0]=cell('integer','1'); record.cells[1]=cell('text','Env');
  record.cells[2]=cell('text','NutrientRegime'); record.cells[3]=cell('text','=');
  record.cells[6]=cell('text','c');
  let draft={review:profile.validateProjectPlotProfileReview(source),cells:{}};
  assert.equal(ruleEditor.profileRulesDirty(draft),false);
  draft=ruleEditor.stageProfileRule(draft,'-1','Table','Veg',false);
  assert.equal(ruleEditor.profileRuleValue(draft,'-1','Field').raw,'Species');
  const request=ruleEditor.profileRuleEditRequest(draft);
  assert.deepEqual(JSON.parse(JSON.stringify(request.drafts[0])),{
    rowId:'-1',changes:[{column:'Table',value:cell('text','Veg')},{column:'Field',value:cell('text','Species')}]
  });
  for(const raw of ['32768','-32769','1.0','1e2','-0',' 1']) {
    const invalid=ruleEditor.stageProfileRule(draft,'-1','Order',raw,false);
    assert.equal(ruleEditor.profileRuleValue(invalid,'-1','Order').raw,raw);
    assert.equal(ruleEditor.profileRulesDirty(invalid),true);
    assert.throws(()=>ruleEditor.profileRuleEditRequest(invalid),/every raw profile error/);
  }
  for(const raw of ['a'.repeat(256),'\uD800','😀'.repeat(128)]) {
    const invalid=ruleEditor.stageProfileRule(draft,'-1','Criteria',raw,false);
    assert.ok(ruleEditor.profileRuleErrors(invalid).length);
  }
  for(const raw of ['a'.repeat(255),'😀'.repeat(127)+'a',"  O'Brien  "]) {
    const valid=ruleEditor.stageProfileRule(draft,'-1','Criteria',raw,false);
    assert.equal(ruleEditor.profileRuleValue(valid,'-1','Criteria').value.text,raw);
    assert.equal(ruleEditor.profileRuleErrors(valid).length,0);
  }
  const empty=ruleEditor.stageProfileRule(draft,'-1','Criteria','',false);
  const nullDraft=ruleEditor.stageProfileRule(draft,'-1','Criteria','',true);
  assert.equal(ruleEditor.profileRuleValue(empty,'-1','Criteria').value.storage,'text');
  assert.equal(ruleEditor.profileRuleValue(nullDraft,'-1','Criteria').value.storage,'null');
  const badLayer=ruleEditor.stageProfileRule(draft,'-1','Layer','SumD',false);
  assert.ok(ruleEditor.profileRuleErrors(badLayer).length);
  const corrected=ruleEditor.stageProfileRule(badLayer,'-1','Layer','SumB',false);
  assert.equal(ruleEditor.profileRuleErrors(corrected).length,0);
  assert.equal(ruleEditor.profileRuleValue(corrected,'-1','Layer').value.text,'SumB');
  const envAgain=ruleEditor.stageProfileRule(corrected,'-1','Table','Env',false);
  assert.ok(ruleEditor.profileRuleValue(envAgain,'-1','Layer').error);
  assert.throws(()=>ruleEditor.profileRuleEditRequest(envAgain),/every raw profile error/);
  assert.equal(ruleEditor.profileRuleValue(envAgain,'-1','Criteria').raw,'c');
});

test('Profile suggestions preserve project/reference ownership and historical omission without restricting free literal inputs',()=>{
  const source=profile.validateProjectPlotProfileReview(review());
  source.rules.rows[0].cells[1]=cell('text','env');
  source.rules.rows[0].cells[6]=cell('text','a'.repeat(400));
  let draft={review:source,cells:{}};
  const suggestions=ruleEditor.validateProfileChoices({envFields:['PlotNumber','Quoted field'],species:[cell('text','ABC'),cell('text',''),cell('null')]});
  assert.equal(JSON.stringify(ruleEditor.profileRuleOptions(draft,'-1','Field',suggestions)),JSON.stringify(suggestions.envFields));
  assert.throws(()=>ruleEditor.validateProfileChoices({...suggestions,envFields:['P','P']}),/Complete selected-project/);
  assert.throws(()=>ruleEditor.validateProfileChoices({...suggestions,species:[{storage:'null',text:'invalid'}]}),/inconsistent typed/);
  const historical=ruleEditor.stageProfileRule(draft,'-1','Criteria','a'.repeat(400),false);
  assert.equal(ruleEditor.profileRulesDirty(historical),false);
  assert.equal(ruleEditor.profileRuleErrors(historical).length,0);
  draft=ruleEditor.stageProfileRule(historical,'-1','Table','vEg',false);
  assert.equal(ruleEditor.profileRuleValue(draft,'-1','Table').value.text,'vEg');
  assert.equal(ruleEditor.profileRuleValue(draft,'-1','Field').value.text,'Species');
  assert.equal(JSON.stringify(ruleEditor.profileRuleOptions(draft,'-1','Species',suggestions)),JSON.stringify(['ABC','']));
  const request=ruleEditor.profileRuleEditRequest(draft);
  assert.equal(request.drafts[0].changes.some(change=>change.column==='Criteria'),false);
  const retained=JSON.parse(JSON.stringify(ruleEditor.stageProfileRule(draft,'-1','Order','oops',false)));
  assert.equal(ruleEditor.profileRuleValue(retained,'-1','Order').raw,'oops');
  assert.ok(ruleEditor.profileRuleErrors(retained).length);
  assert.equal(ruleEditor.profileRulesDirty({review:retained.review,cells:{}}),false);
});

test('Rule lifecycle proposals never allocate fake IDs/defaults and share raw/dependent validation with existing rules',()=>{
  const source=profile.validateProjectPlotProfileReview(review());
  let creation=ruleEditor.beginProfileRuleCreation(source);
  const blank=ruleEditor.profileRuleCreationRequest(creation);
  assert.equal(blank.values.length,8);
  assert.equal(blank.values.every(change=>change.value.storage==='null'),true);
  assert.equal(Object.hasOwn(blank,'rowId'),false);
  assert.equal(blank.values.some(change=>change.column==='PlotCount'),false);
  creation=ruleEditor.stageProfileRuleCreation(creation,'Table','Veg',false);
  assert.equal(ruleEditor.profileCreationValue(creation,'Field').value.text,'Species');
  creation=ruleEditor.stageProfileRuleCreation(creation,'Layer','SumB',false);
  const bad=ruleEditor.stageProfileRuleCreation(creation,'Order','32768',false);
  assert.throws(()=>ruleEditor.profileRuleCreationRequest(bad),/every raw creation error/);
  const retained=JSON.parse(JSON.stringify(bad));
  assert.equal(ruleEditor.profileCreationValue(retained,'Order').raw,'32768');
  const changedTable=ruleEditor.stageProfileRuleCreation(creation,'Table','Env',false);
  assert.ok(ruleEditor.profileCreationErrors(changedTable).length);
  const cells=source.rules.columns.map(column=>{
    const input=ruleEditor.profileEditableFields.find(name=>name===column.name);
    return input ? ruleEditor.profileCreationValue(creation,input).value : cell('null');
  });
  ruleEditor.validateCreatedProfileRule({rowId:'3',cells},creation);
  for(const rowId of ['-1','2','03','9223372036854775808']) assert.throws(()=>ruleEditor.validateCreatedProfileRule({rowId,cells},creation),/identity\/schema/);
  assert.throws(()=>ruleEditor.validateCreatedProfileRule({rowId:'3',cells:[...cells.slice(0,8),cell('integer','0')]},creation),/differs/);
  assert.throws(()=>ruleEditor.profileRuleDeletionRequest(source,'-1',false),/Explicitly confirm/);
  assert.throws(()=>ruleEditor.profileRuleDeletionRequest(source,'unreviewed',true),/Explicitly confirm/);
  assert.equal(JSON.stringify(ruleEditor.profileRuleDeletionRequest(source,'-1',true)),
    JSON.stringify({originalRules:source.rules,rowId:'-1',confirmed:true}));
  assert.throws(()=>ruleEditor.beginProfileRuleCreation({...source,rules:{...source.rules,columns:[...source.rules.columns,{name:'Extra'}]}}),/unmapped/);
});

test('Lifecycle refresh independently preserves ownership, exact surviving typed values/counts and the one intended identity change',()=>{
    const source=profile.validateProjectPlotProfileReview(review());
    const created=row('3');
    const next={...source,rules:{...source.rules,rows:[created,...source.rules.rows]}};
    ruleEditor.validateProfileLifecycleReview(source,next,{created});
    const removed={...source,rules:{...source.rules,rows:source.rules.rows.filter(row=>row.rowId!=='-1')}};
    ruleEditor.validateProfileLifecycleReview(source,removed,{deleted:'-1'});
    const drift=structuredClone(next);
    drift.rules.rows[1].cells[8]=cell('integer','123');
    for(const changed of [drift,{...next,project:'Other'},{...next,table:'Other_Profile'},
      {...next,rules:{...next.rules,rows:source.rules.rows}},
      {...next,rules:{...next.rules,columns:[...next.rules.columns].reverse()}}]) {
      assert.throws(()=>ruleEditor.validateProfileLifecycleReview(source,changed,{created}));
    }
    assert.throws(()=>ruleEditor.validateProfileLifecycleReview(source,source,{deleted:'-1'}));
  });

  test('Shared responsive proposal/existing inputs retain visible associated labels, exact raw errors and explicit NULL controls',()=>{
    const component=serverComponent(read('ProfileRuleInputs.svelte'),'ProfileRuleInputs.svelte',{});
    const proposal=ruleEditor.stageProfileRuleCreation(ruleEditor.beginProfileRuleCreation(
      profile.validateProjectPlotProfileReview(review())),'Order','oops',false);
    const fields=ruleEditor.profileEditableFields.map(name=>({name,cell:ruleEditor.profileCreationValue(proposal,name),options:[]}));
    const html=render(component,{props:{prefix:'new-profile-rule',identityLabel:'new proposal',fields,onstage(){}}}).body;
    assert.equal((html.match(/type="text"/g)||[]).length,8);
    assert.equal((html.match(/type="checkbox"/g)||[]).length,8);
    for(const name of ruleEditor.profileEditableFields) {
      assert.ok(html.includes(`for="new-profile-rule-${name}"`));
      assert.ok(html.includes(`id="new-profile-rule-${name}"`));
      assert.ok(html.includes(`NULL ${name}, new proposal`));
    }
    assert.match(html,/value="oops"/);
    assert.match(html,/aria-invalid="true"/);
    assert.match(html,/role="alert"/);
    assert.match(html,/sm:grid-cols-2 xl:grid-cols-4/);
  });

  test('Creation/deletion controls are separately default-off, resident and own parent gates without replaying committed writes',()=>{
    const component=read('ProjectPlotProfileReview.svelte'),form=read('FS882Form.svelte');
    assert.match(component,/allowCreation = false, allowDeletion = false/);
    assert.match(form,/VITE_PROJECT_PLOT_PROFILE_CREATION === 'true'/);
    assert.match(form,/VITE_PROJECT_PLOT_PROFILE_DELETION === 'true'/);
    assert.match(form,/allowCreation=\{profileCreationEnabled\} allowDeletion=\{profileDeletionEnabled\}/);
    assert.match(form,/profileEditingEnabled && \(metadataOpen \|\| codeCheckOpen \|\| personalDraft !== null/);
    assert.match(component,/blocked = \$derived\(dirty \|\| creation !== null \|\| deletion !== null \|\| committedFailure\)/);
    assert.match(component,/creation = null; deletion = null/);
    assert.match(component,/if \(!allowEditing \|\| !allowCreation/);
    assert.match(component,/if \(!allowEditing \|\| !allowDeletion/);
    assert.match(component,/client\.CreateProjectPlotProfileRule\(profileRuleCreationRequest\(source\)\)/);
    assert.match(component,/client\.DeleteProjectPlotProfileRule\(profileRuleDeletionRequest\(source, rowId, true\)\)/);
    assert.match(component,/committed = true; creation = null/);
    assert.match(component,/committed = true; deletion = null/);
    assert.match(component,/validateProfileLifecycleReview\(source\.review, next, \{ created \}\)/);
    assert.match(component,/validateProfileLifecycleReview\(source, next, \{ deleted: rowId \}\)/);
    assert.match(component,/nullable\/raw proposal retained for retry/);
    assert.match(component,/original confirmation retained for retry/);
    assert.match(component,/no physical identity allocated/);
    assert.match(component,/Profile rule deletion confirmation/);
    assert.match(component,/profileCellLabel\(cell\)/);
    assert.match(component,/Cancel profile deletion/);
  });
