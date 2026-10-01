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
const {heightField,heightValue,stageHeight,heightErrors,heightDirty,heightUpdates,singleMaximum}=output;
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
  assert.match(form,/\{#if heightUnsaved\}[\s\S]*?Save height drafts/);
  assert.match(form,/await PlotService\.UpdateHeightRecords\(draft\.plotNumber, updates\)/);
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
