const assert=require('node:assert/strict');
const {test}=require('node:test');
const {readFileSync}=require('node:fs');
const path=require('node:path');
const {render}=require('svelte/server');
const {compile}=require('svelte/compiler');
const {loadTypeScript,serverComponent}=require('./svelteTestHelpers.cjs');
const quality=loadTypeScript('qualityEditor.ts',{'./becEditor':loadTypeScript('becEditor.ts')});
const editor=loadTypeScript('speciesCodeCheckEditor.ts',{'./qualityEditor':quality});
const json=value=>JSON.parse(JSON.stringify(value));
const review={project:'Sample',su:'Scoped unit',rows:[
  {id:0,plotNumber:'P1',code:'OLD',status:'unlisted'},
  {id:-9,plotNumber:'P2',code:'OLD',status:'unlisted'},
  {id:3,plotNumber:'P3',code:null,status:'missing'},
  {id:4,plotNumber:'P4',code:'',status:'unlisted'},
]};
const options=[{code:'NEW',source:'master',scientificName:null,englishName:'',lifeform:null,codeType:null}];
test('Species check names its scope and keeps original NULL/empty/literal identities through remount',()=>{
  let draft=editor.stageCodeCheck(editor.beginCodeCheck(review),0,'NEW');
  assert.throws(()=>editor.codeCheckUpdates(draft),/Review this literal replacement/);
  draft=editor.acceptCodeCheckTarget(json(draft),0,'NEW',options,true);
  assert.deepEqual(json(editor.codeCheckUpdates(json(draft))),[
    {id:0,plotNumber:'P1',expected:'OLD',value:'NEW'},
    {id:-9,plotNumber:'P2',expected:'OLD',value:'NEW'},
  ]);
  assert.equal(draft.review.rows[2].code,null);
  assert.equal(draft.review.rows[3].code,'');
  draft=editor.stageCodeCheck(draft,0,'new');
  assert.throws(()=>editor.acceptCodeCheckTarget(draft,0,'new',options,false),/no longer matches/);
});
test('Ignore and Ignore All are runtime-only and never manufacture user definitions or assignments',()=>{
  let draft=editor.stageCodeCheck(editor.beginCodeCheck(review),3,'bad');
  draft=editor.ignoreCodeCheck(json(draft),3,true);
  assert.deepEqual(json(draft.ignored),[3]);
  assert.deepEqual(json(editor.codeCheckUpdates(draft)),[]);
  draft=editor.ignoreCodeCheck(draft,0,true);
  assert.deepEqual(json(draft.ignored),[3,0,-9]);
  assert.equal(draft.review.rows.length,4);
  draft=editor.stageCodeCheck(draft,0,'NEW');
  assert.equal(draft.ignored.includes(0),false);
  assert.equal(draft.ignored.includes(-9),true);
});
test('Raw errors and stale replacement reviews block transport without trimming, case repair or truncation',()=>{
  let draft=editor.beginCodeCheck(review);
  for(const raw of ['', 'OVERLONG1', '\ud800']){
    draft=editor.stageCodeCheck(draft,0,raw);
    assert.equal(draft.cells['0'].raw,raw);
    assert.throws(()=>editor.codeCheckUpdates(json(draft)));
  }
  draft=editor.stageCodeCheck(draft,0,'NEW');
  assert.throws(()=>editor.acceptCodeCheckTarget(draft,0,'OTHER',options,false));
  draft=editor.acceptCodeCheckTarget(draft,0,'NEW',options,false);
  draft.cells['0'].reviewed='STALE';
  assert.throws(()=>editor.codeCheckUpdates(draft),/original explicit replacement/);
  assert.throws(()=>editor.beginCodeCheck({...review,rows:[review.rows[0],review.rows[0]]}),/ambiguous/);
  assert.throws(()=>editor.beginCodeCheck({...review,su:''}),/explicit.*scope/);
});
test('Code-check personal metadata retains independent physical source and original literal names',()=>{
  const species=loadTypeScript('vegetationSpeciesEditor.ts',{'./qualityEditor':quality});
  const personal=loadTypeScript('personalSpeciesEditor.ts',{'./qualityEditor':quality,'./vegetationSpeciesEditor':species});
  let metadata=personal.beginCodeCheckPersonalSpecies(review.rows[0],{form:'USysCodeCheck',entered:'OLD',aliases:[],users:[]});
  assert.deepEqual(json(metadata.source),{kind:'codecheck',id:0,plotNumber:'P1',expected:'OLD'});
  assert.equal(personal.matchesCodeCheckPersonalSpeciesSource(metadata,review.rows[0]),true);
  assert.equal(personal.matchesCodeCheckPersonalSpeciesSource(metadata,{...review.rows[0],plotNumber:'FOREIGN'}),false);
  assert.equal(personal.matchesCodeCheckPersonalSpeciesSource(metadata,{...review.rows[0],code:'CHANGED'}),false);
  metadata=personal.stagePersonalText(metadata,'scientificName','  Literal é  ',false);
  metadata=personal.stagePersonalText(metadata,'englishName','',false);
  assert.deepEqual(json(personal.personalSpeciesRequest(metadata)),{entered:'OLD',scientificName:'  Literal é  ',englishName:'',lifeform:null});
  assert.throws(()=>personal.beginCodeCheckPersonalSpecies(review.rows[2],{form:'USysCodeCheck',entered:'',aliases:[],users:[]}));
});
test('Shared personal fields keep visible source labels, original IDs and distinct NULL/empty controls',()=>{
  const species=loadTypeScript('vegetationSpeciesEditor.ts',{'./qualityEditor':quality});
  const personal=loadTypeScript('personalSpeciesEditor.ts',{'./qualityEditor':quality,'./vegetationSpeciesEditor':species});
  const fields=serverComponent(readFileSync(path.join(__dirname,'PersonalSpeciesFields.svelte'),'utf8'),'PersonalSpeciesFields.svelte',{'./personalSpeciesEditor':personal});
  const draft=personal.beginCodeCheckPersonalSpecies(review.rows[0],{form:'USysCodeCheck',entered:'OLD',aliases:[],users:[]});
  const html=render(fields,{props:{draft,disabled:false,onchange(){}}}).body;
  for(const id of ['personal-code','personal-lifeform','personal-scientificName','personal-englishName','personal-null-scientificName','personal-null-englishName']){
    assert.match(html,new RegExp(`for="${id}"`));
    assert.equal((html.match(new RegExp(`id="${id}"`,'g'))||[]).length,1);
  }
  assert.match(html,/store NULL, not an empty string/);
});
test('Code-check scope and raw errors remain outside remounted tabs and explicitly gate Save/Lock/close/Undo',()=>{
  const form=readFileSync(path.join(__dirname,'FS882Form.svelte'),'utf8');
  const component=readFileSync(path.join(__dirname,'SpeciesCodeCheck.svelte'),'utf8');
  const page=readFileSync(path.join(__dirname,'SourcePage.svelte'),'utf8');
  assert.match(form,/VITE_SPECIES_CODE_CHECK_EDITING === 'true'/);
  assert.match(form,/nonParentChildUnsaved = \$derived\([^;]*codeCheckOpen/);
  assert.match(form,/childParentDisabled = \$derived\([^;]*codeCheckOpen/);
  assert.match(form,/if \(codeCheckOpen\) return codeCheckEditor\?\.getCloseState/);
  assert.match(form,/if \(codeCheckOpen\) \{ codeCheckEditor\?\.undo\(\); return; \}/);
  assert.match(form,/ordinary plot Save never applies them/);
  assert.match(form,/Finish or close species-code review before changing the plot lock/);
  assert.match(page,/btnCheckSppCodes' && onSpeciesCheck/);
  assert.match(component,/await client\.SaveSpeciesCodeCheck\(updates\)/);
  assert.match(component,/await client\.CreatePersonalSpeciesDefinition\(request\)/);
  assert.match(component,/matchesCodeCheckPersonalSpeciesSource\(proposed, rows\[0\]\)/);
  assert.match(component,/matchesCodeCheckPersonalSpeciesSource\(proposed, sources\[0\]\)/);
  assert.match(component,/Ignore never creates or deletes temporary LifeForm999/);
  assert.equal(compile(component,{filename:'SpeciesCodeCheck.svelte',generate:'client'}).warnings.length,0);
});
