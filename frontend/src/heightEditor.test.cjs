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
  assert.match(source,/childUnsaved = \$derived\([^;]*deletionReview !== null\)/);
  assert.match(source,/childParentDisabled = \$derived\([^;]*deletionReview !== null\)/);
  assert.match(source,/deletionRequest\+\+; deletionReads\.cancelAll\(\)/);
  assert.match(source,/const species = rows\[0\]\.species/);
  assert.match(source,/const plot = draft\.plotNumber/);
  assert.match(source,/ordinary Save never deletes rows/);
  assert.match(source,/saveReason[\s\S]*deletionReview !== null \? 'Confirm reviewed vegetation deletion explicitly/);
  assert.match(source,/if \(deletionReview !== null\) \{ void cancelVegetationDeletion\(\); return; \}/);
  assert.match(source,/Vegetation deletion failed; review retained/);
  assert.match(source,/Vegetation deletion committed, but refresh failed/);
});
