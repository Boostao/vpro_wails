const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const ts = require('typescript');
const { compile } = require('svelte/compiler');
const output = {};
const numeric = {};
vm.runInNewContext(ts.transpileModule(readFileSync(path.join(__dirname,'numericEditor.ts'),'utf8'),
  {compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText,{exports:numeric});
vm.runInNewContext(ts.transpileModule(readFileSync(path.join(__dirname,'heightEditor.ts'),'utf8'),
  {compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText,
  {exports:output,require:name=>{if(name!=='./numericEditor')throw new Error(`Unexpected dependency ${name}`);return numeric;}});
const {heightField,heightValue,stageHeight,heightErrors,heightDirty,heightUpdates,vegetationNumberUpdates,singleMaximum}=output;
const json = value => JSON.parse(JSON.stringify(value));
test('Source NULL Cover6 notice stays nonblocking and distinguishes view removal from physical deletion',()=>{
  for(const form of ['SubVegCXL','SubVegChtXL']){
    let drafts=stageHeight({},-9,'cover6','',0,form);
    assert.equal(heightErrors(drafts).length,0);
    assert.equal(output.heightSourceNotices(drafts).length,1);
    assert.match(output.heightSourceNotices(drafts)[0],/does not delete the vegetation record/);
    assert.equal(json(vegetationNumberUpdates(drafts))[0].values.cover6,null);
    assert.equal(output.heightSourceNotices(json(drafts)).length,1);
    drafts=stageHeight(drafts,-9,'cover6','bad',0,form);
    assert.equal(output.heightSourceNotices(drafts).length,0);
    drafts=stageHeight(drafts,-9,'cover6','0',0,form);
    assert.equal(output.heightSourceNotices(drafts).length,0);
  }
  for(const [field,raw,expected,form] of [
    ['cover6','',null,'SubVegCXL'],['height6','',1,'SubVegChtXL'],
    ['cover1','',1,'SubVegAXL_BC'],['cover6','',1,'SubVegDXL'],
  ]){
    assert.equal(output.heightSourceNotices(stageHeight({},0,field,raw,expected,form)).length,0);
  }
});
test('Source warning feedback stays visible outside remounted grids and never joins invalid/Save gates',()=>{
  const form=readFileSync(path.join(__dirname,'FS882Form.svelte'),'utf8');
  assert.match(form,/heightNotices = \$derived\(heightSourceNotices\(heightDrafts\)\)/);
  assert.match(form,/each heightNotices as message[\s\S]{0,80}role="status"/);
  assert.doesNotMatch(form,/heightNotices\.length[^;\n]*(?:disabled|canSave)|invalid[^;\n]*heightNotices/);
  assert.match(form,/notices\.join\(' '\)/);
  const child=readFileSync(path.join(__dirname,'SourceChild.svelte'),'utf8');
  assert.match(child,/td:focus-within/);
  assert.match(child,/\.source-cell:focus-visible/);
});
test('Source grids stage lexical input while close, view and cancellation guards remain wired',()=>{
  for(const file of ['SourceChild.svelte','FS882Form.svelte','SourcePage.svelte']){
    const source=readFileSync(path.join(__dirname,file),'utf8');
    assert.equal(compile(source,{filename:file,generate:'client'}).warnings.length,0);
  }
  const child=readFileSync(path.join(__dirname,'SourceChild.svelte'),'utf8');
  assert.match(child,/oninput=\{event => onstage/);
  assert.match(child,/staged\?\.raw \?\? value/);
  const form=readFileSync(path.join(__dirname,'FS882Form.svelte'),'utf8');
  assert.match(form,/\{#if heightUnsaved\}[\s\S]*?Save height\/cover drafts/);
  assert.match(form,/await PlotService\.UpdateVegetationNumbers\(draft\.plotNumber, vegetationNumberUpdates\(heightDrafts\)\)/);
  assert.match(form,/VITE_HEIGHT_EDITING !== 'false'/);
  assert.match(form,/async function cancelHeightDrafts\(\)[\s\S]*?await loadChildData\(draft\.plotNumber\)/);
  assert.match(form,/if \(heightUnsaved\) \{\s*error = 'Save or Cancel height drafts before changing the plot lock.'/);
});
test('Height parsing preserves float64 and native negative/zero/NULL semantics without guessing units',()=>{
  for(const text of ['-1','0','1.234567890123','1e3'])assert.equal(heightValue('height1',text).value,Number(text));
  assert.equal(heightValue('height6','').value,null);
  assert.equal(heightValue('height1','1.234567890123').value,1.234567890123);
  assert.equal(heightValue('height1',String(singleMaximum)).error,null);
});
test('Nonfinite, partial and Single overflow input remains an explicit invalid draft',()=>{
  for(const text of ['NaN','Infinity','-Infinity','3.5e38','1e309','-','.','12x'])assert.ok(heightValue('height1',text).error,text);
  const draft=stageHeight({},1,'height1','NaN',1.25);
  assert.equal(heightDirty(draft),true);
  assert.equal(draft['1'].height1.raw,'NaN');
  assert.throws(()=>heightUpdates(draft),/finite/);
});
test('Cover semantics accept negative/fractional/NULL but never silently truncate 100',()=>{
  for(const field of ['cover1','cover6','totalA','totalB']){
    assert.equal(heightValue(field,'-1').value,-1);
    assert.equal(heightValue(field,'99.999').error,null);
    assert.equal(heightValue(field,'').value,null);
    assert.match(heightValue(field,'100').error,/not truncated to 10/);
  }
});
test('Staged multirow payload carries only edited fields and original NULL/value expectations',()=>{
  let drafts=stageHeight({},0,'height3','-7.25',null);
  drafts=stageHeight(drafts,42,'cover6','',20);
  drafts=stageHeight(drafts,0,'height3','-8.123456789',15);
  assert.deepEqual(json(heightUpdates(drafts)),[
    {id:0,values:{height3:-8.123456789},expected:{height3:null}},
    {id:42,values:{cover6:null},expected:{cover6:20}}
  ]);
  assert.equal(heightErrors(drafts).length,0);
});
test('Corrections and semantic no-ops clear invalidity without losing lexical input or expected baseline',()=>{
  let drafts=stageHeight({},1,'height1','invalid',1.25);
  drafts=stageHeight(drafts,1,'height1','1.2500',9);
  assert.equal(drafts['1'].height1.raw,'1.2500');
  assert.equal(heightDirty(drafts),false);
  assert.equal(heightErrors(drafts).length,0);
  assert.equal(heightUpdates(drafts).length,0);
});
test('Identity is signed32 including imported zero; columns do not accept unrelated or personal fields',()=>{
  assert.equal(heightField('Height6'),'height6');
  assert.equal(heightField('cover3'),'cover3');
  assert.equal(heightField('TotalA'),'totalA');
  assert.equal(heightField('Species'),undefined);
  assert.equal(heightField('HeightA'),undefined);
  assert.equal(heightField('Height5a'),undefined);
  for(const id of [null,NaN,1.5,2147483648])assert.throws(()=>stageHeight({},id,'height1','1',null),/identity/);
});
test('Draft transforms are immutable and preserve partner cell baseline during repeated edits',()=>{
  const first=stageHeight({},1,'height1','2',1);
  const second=stageHeight(first,1,'height2','3',null);
  assert.equal(first['1'].height2,undefined);
  assert.equal(second['1'].height1.expected,1);
  assert.equal(second['1'].height2.expected,null);
});
test('All eleven source covers/totals and six heights share strict numeric parsing and explicit original source forms',()=>{
  for(const field of ['cover1','cover2','cover3','totalA','cover4','cover5','totalB','cover6','cover7','cover8','cover9']){
    assert.equal(heightField(field),field);
    assert.equal(heightValue(field,'99.999').error,null);
    assert.ok(heightValue(field,'100').error);
    assert.equal(heightValue(field,'-2.25').value,-2.25);
    assert.equal(heightValue(field,'').value,null);
  }
  let drafts=stageHeight({},0,'cover1','3.5',0,'SubVegAXL_BC');
  drafts=stageHeight(drafts,0,'height1','-7.123456789',null,'SubVegAhtXL');
  drafts=stageHeight(drafts,0,'cover1','4.5',999,'SubVegAhtXL');
  drafts=stageHeight(drafts,-9,'cover9','',20,'SubVegDXL');
  assert.deepEqual(json(vegetationNumberUpdates(drafts)),[
    {id:0,values:{cover1:4.5,height1:-7.123456789},expected:{cover1:0,height1:null},
      forms:{cover1:'SubVegAXL_BC',height1:'SubVegAhtXL'}},
    {id:-9,values:{cover9:null},expected:{cover9:20},forms:{cover9:'SubVegDXL'}}
  ]);
  assert.throws(()=>vegetationNumberUpdates(stageHeight({},0,'cover7','1',0)),/original source form/);
  const invalid=stageHeight(drafts,0,'cover1','100',0,'SubVegAhtXL');
  assert.throws(()=>vegetationNumberUpdates(invalid),/less than 100/);
  const source=readFileSync(path.join(__dirname,'FS882Form.svelte'),'utf8');
  assert.match(source,/VITE_VEGETATION_NUMBER_EDITING !== 'false'/);
  assert.match(source,/onstage=\{numberEditingEnabled \?/);
  assert.match(source,/onedit=\{undefined\} ondelete=\{deletionEnabled \? reviewVegetationDeletion : undefined\}/);
  assert.match(source,/Vegetation record preview \(read-only\)/);
  assert.doesNotMatch(source,/updateVegCover/);
});
test('Deletion review compares independently returned source identity, code, full-row digest and physical columns',()=>{
  const deletion={};
  vm.runInNewContext(ts.transpileModule(readFileSync(path.join(__dirname,'vegetationDeletionEditor.ts'),'utf8'),
    {compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText,{exports:deletion});
  const review={id:0,form:'SubVegAXL_BC',species:'RAW',expected:'a'.repeat(64),columns:['ID','PlotNumber','Species','HiddenText']};
  assert.equal(deletion.validateDeletionReview(review,review.form,0,'RAW'),review);
  for(const change of [{id:-9},{form:'SubVegCXL'},{species:'changed'},{expected:''},{expected:'A'.repeat(64)},{columns:null},{columns:['ID']}])
    assert.throws(()=>deletion.validateDeletionReview({...review,...change},review.form,0,'RAW'),/planned source row/);
  assert.throws(()=>deletion.validateDeletionReview(review,review.form,2147483648,'RAW'));
});
test('Explicit deletion confirmation owns lifecycle without allowing generic Save, close or prototype bypass',()=>{
  const source=readFileSync(path.join(__dirname,'FS882Form.svelte'),'utf8');
  assert.match(source,/VITE_VEGETATION_DELETE_EDITING === 'true'/);
  assert.match(source,/childUnsaved = \$derived\([^;]*deletionReview !== null \|\| creationDraft !== null\)/);
  assert.match(source,/childParentDisabled = \$derived\([^;]*deletionReview !== null \|\| creationDraft !== null \|\| personalDraft !== null\)/);
  assert.match(source,/deletionRequest\+\+; deletionReads\.cancelAll\(\)/);
  assert.match(source,/const species = rows\[0\]\.species/);
  assert.match(source,/const plot = draft\.plotNumber/);
  assert.match(source,/ordinary Save never deletes rows/);
  assert.match(source,/saveReason[\s\S]*deletionReview !== null \? 'Confirm reviewed vegetation deletion explicitly/);
  assert.match(source,/if \(deletionReview !== null\) \{ void cancelVegetationDeletion\(\); return; \}/);
  assert.match(source,/Vegetation deletion failed; review retained/);
  assert.match(source,/Vegetation deletion committed, but refresh failed/);
});
const creation={};
const { loadTypeScript } = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', { './becEditor': loadTypeScript('becEditor.ts') });
const species = loadTypeScript('vegetationSpeciesEditor.ts', { './qualityEditor': quality });
vm.runInNewContext(ts.transpileModule(readFileSync(path.join(__dirname,'vegetationCreationEditor.ts'),'utf8'),
  {compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2022}}).outputText,
  {exports:creation,require:name=>{
    if(name==='./heightEditor')return output;
    if(name==='./qualityEditor')return quality;
    if(name==='./vegetationSpeciesEditor')return species;
    throw new Error(`Unexpected dependency ${name}`);
  }});
test('Creation preserves source numeric order, canonical literal codes, explicit zero/NULL and no guessed identity or Layer',()=>{
  let draft=creation.beginVegetationCreation('SubVegAXL_BC',['Species','Cover2','Cover1','Cover2','Collected']);
  assert.deepEqual(Object.keys(draft.cells),['cover2','cover1']);
  assert.equal(creation.vegetationCreationErrors(draft,[]).length,2);
  draft={...creation.stageVegetationCreation(draft,'Cover1','0'),species:'A'};
  const request=creation.vegetationCreationRequest(draft,[{code:'A'}]);
  assert.deepEqual(json(request),{form:'SubVegAXL_BC',species:'A',values:{cover2:null,cover1:0}});
  assert.throws(()=>creation.vegetationCreationRequest({...draft,species:'a'},[{code:'A'}]),/exact canonical/);
  assert.throws(()=>creation.stageVegetationCreation(draft,'height6','3'),/does not belong/);
});
test('Creation reuses raw Single/cover validation and keeps invalid input until valid correction',()=>{
  let draft={...creation.beginVegetationCreation('SubVegDXL',['Cover7']),species:'D'};
  draft=creation.stageVegetationCreation(draft,'cover7','100');
  assert.equal(draft.cells.cover7.raw,'100');
  assert.match(draft.cells.cover7.error,/less than 100/);
  assert.throws(()=>creation.vegetationCreationRequest(draft,[{code:'D'}]),/less than 100/);
  draft=creation.stageVegetationCreation(draft,'cover7','-3.125');
  assert.equal(draft.cells.cover7.error,null);
  assert.equal(creation.vegetationCreationRequest(draft,[{code:'D'}]).values.cover7,-3.125);
});
test('Height-only A creation stays explicit while C-height still requires source Cover6',()=>{
  let a={...creation.beginVegetationCreation('SubVegAhtXL',['Cover1','Height1']),species:'A'};
  a=creation.stageVegetationCreation(a,'height1','-7.123456789');
  assert.deepEqual(json(creation.vegetationCreationRequest(a,[{code:'A'}]).values),{cover1:null,height1:-7.123456789});
  let c={...creation.beginVegetationCreation('SubVegChtXL',['Cover6','Height6']),species:'C'};
  c=creation.stageVegetationCreation(c,'height6','1');
  assert.throws(()=>creation.vegetationCreationRequest(c,[{code:'C'}]),/no zero cover/);
  c=creation.stageVegetationCreation(c,'cover6','0');
  assert.equal(creation.vegetationCreationRequest(c,[{code:'C'}]).values.cover6,0);
});
test('Opt-in source creation owns independent persistent lifecycle and fails closed after committed refresh failure',()=>{
  const source=readFileSync(path.join(__dirname,'FS882Form.svelte'),'utf8');
  assert.match(source,/VITE_VEGETATION_CREATE_EDITING === 'true'/);
  assert.match(source,/childUnsaved = \$derived\([^;]*creationDraft !== null\)/);
  assert.match(source,/childParentDisabled = \$derived\([^;]*creationDraft !== null \|\| personalDraft !== null\)/);
  assert.match(source,/creationInvalid\.length > 0 \? 'Correct invalid vegetation creation/);
  assert.match(source,/if \(creationDraft !== null\) \{ await saveVegetationCreation\(\); return; \}/);
  assert.match(source,/if \(creationDraft !== null\) \{ void cancelVegetationCreation\(\); return; \}/);
  assert.match(source,/Vegetation creation failed; draft retained/);
  assert.match(source,/Vegetation creation committed, but refresh or identity verification failed/);
  assert.match(source,/observed\[0\]\.species !== request\.species/);
  assert.match(source,/creationChoices\.form === creationDraft\.form && creationChoices\.entered === creationDraft\.species/);
  assert.match(source,/Creation species choices unavailable; draft retained/);
  assert.match(source,/saving a new row never writes the user database/);
});
test('Creation decisions share source UCase, alias precedence and NULL-code rules without phantom row identity',()=>{
  for(const [kind,entered,selected,aliases,users,value] of [
    ['replace','olddup','new_a',[{code:'new_a'},{code:'NEW_B'}],[],'NEW_A'],
    ['keep','olddup',undefined,[{code:'NEW_A'}],[],'OLDDUP'],
    ['user','personal','Personal',[{code:null}],[{code:'Personal',codeType:null}],'PERSONAL'],
  ]){
    let draft=creation.stageVegetationCreation(creation.beginVegetationCreation('SubVegAXL_BC',['Cover1']),'Cover1','0');
    draft=creation.stageVegetationCreationSpecies(draft,entered);
    const choices={form:draft.form,entered,aliases,users};
    const resolved=creation.chooseVegetationCreationSpecies(draft,choices,kind,selected);
    const request=creation.vegetationCreationRequest(resolved,[]);
    assert.deepEqual(json(request),{form:draft.form,species:value,decision:kind,entered,
      ...(selected===undefined?{}:{selected}),values:{cover1:0}});
    assert.throws(()=>creation.chooseVegetationCreationSpecies(resolved,choices,kind,selected),/original creation entry/);
    assert.equal(creation.stageVegetationCreationSpecies(resolved,value).decision,undefined);
    assert.notEqual(creation.vegetationCreationSpeciesError(creation.stageVegetationCreationSpecies(resolved,value),[]),null);
    assert.deepEqual(json(resolved.cells),json(draft.cells));
  }
});
test('Creation decisions reject stale source identity, implicit choices, precedence bypass and unsupported Unicode events',()=>{
  let draft=creation.stageVegetationCreationSpecies(creation.beginVegetationCreation('SubVegCXL',['Cover6']),'old');
  const choices={form:draft.form,entered:'old',aliases:[{code:'NEW'}],users:[{code:'old'}]};
  for(const current of [{...choices,form:'SubVegDXL'},{...choices,entered:'other'}]){
    assert.throws(()=>creation.chooseVegetationCreationSpecies(draft,current,'keep'),/original creation entry/);
  }
  assert.throws(()=>creation.chooseVegetationCreationSpecies(draft,choices,'user','old'),/not available/);
  assert.throws(()=>creation.chooseVegetationCreationSpecies(draft,choices,'replace','new'),/not available/);
  assert.throws(()=>creation.chooseVegetationCreationSpecies(draft,choices,'keep','NEW'),/not available/);
  assert.throws(()=>creation.chooseVegetationCreationSpecies(draft,{...choices,aliases:[{code:null}]},'keep'),/not available/);
  assert.throws(()=>creation.chooseVegetationCreationSpecies(draft,{...choices,aliases:[{code:'É'}]},'replace','É'),/non-ASCII/);
  assert.match(creation.vegetationCreationSpeciesError({...draft,species:'\ud800'},[]),/incomplete Unicode/);
  assert.match(creation.vegetationCreationSpeciesError({...draft,species:'😀'.repeat(5)},[]),/8 UTF-16/);
  const chosen=creation.chooseVegetationCreationSpecies(draft,choices,'keep');
  assert.match(creation.vegetationCreationSpeciesError({...chosen,species:'old'},[]),/explicit creation decision/);
  assert.match(creation.vegetationCreationSpeciesError({...chosen,decision:{...chosen.decision,selected:'NEW'}},[]),/explicit creation decision/);
  assert.equal(creation.vegetationCreationSpeciesError({...draft,species:'É'},[{code:'É'}]),null);
});
