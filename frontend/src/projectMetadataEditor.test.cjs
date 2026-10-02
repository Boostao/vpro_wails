const assert = require('node:assert/strict');
const {test} = require('node:test');
const {readFileSync} = require('node:fs');
const path = require('node:path');
const {compile} = require('svelte/compiler');
const {loadTypeScript} = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', {'./becEditor': loadTypeScript('becEditor.ts')});
const defaults = JSON.parse(readFileSync(path.join(__dirname,'..','..','resources','project-metadata-standard.json'),'utf8'));
const editor = loadTypeScript('projectMetadataEditor.ts', {'./qualityEditor': quality,'../../resources/project-metadata-standard.json':defaults});
const layout = JSON.parse(readFileSync(path.join(__dirname,'..','..','resources','project-metadata-layout.json'),'utf8'));
const presentation = loadTypeScript('projectMetadataPresentation.ts', {'../../resources/project-metadata-layout.json': layout});
const json = value => JSON.parse(JSON.stringify(value));
const cell = (storage, value) => ({storage,text:storage==='text'?value:null,integer:storage==='integer'?value:null,real:null,blobHex:null});
const columns = ['ID','ProjectID','ProjectTitle','StartDate','CollectedSite','DataQualityVeg','EcosysCollectionStandard','Notes'].map(name=>({name,declaredType:''}));
const row = {rowId:'-200',cells:[cell('integer','-200'),cell('text','Project'),cell('text','Historical'),cell('integer','2000'),cell('integer','-1'),cell('text','OLD'),cell('text','OLD'),cell('null')]};
const review = {project:'Sample',plotNumber:'P1',projectId:'Project',projectRecords:{columns,rows:[row,{...row,rowId:'-201',cells:[cell('integer','-201'),...row.cells.slice(1)]}]},masterTemplates:{columns:[],rows:[]}};
const field = (name,kind='text',maximum=255,extra={})=>({name,kind,maximum,collection:false,referenceList:'',referenceColumn:'',limitToList:false,options:[],...extra});

test('Metadata selection is explicit, literal and physical; duplicate candidates never imply first-row choice',()=>{
  const draft = editor.beginMetadataDraft(review,'-201');
  assert.equal(draft.id,-201);
  assert.equal(draft.original.rowId,'-201');
  assert.throws(()=>editor.beginMetadataDraft(review,''),/explicit physical/);
  assert.throws(()=>editor.beginMetadataDraft({...review,projectId:'project'},'-200'),/literal supported/);
  assert.throws(()=>editor.beginMetadataDraft({...review,projectRecords:{columns,rows:[row,row]}},'-200'),/explicit physical/);
  assert.throws(()=>editor.beginMetadataDraft({...review,projectId:null},'-200'),/literal parent/);
});
test('Blank metadata proposal captures empty schema and existing bounded parent identity without inventing a physical ID',()=>{
  const names=presentation.metadataGroups.flatMap(group=>group.names);
  const schema=['ID','ProjectID','AllSpecs','TableOfLists','DateLastEdited',...names].map(name=>({name,declaredType:'TEXT'}));
  const empty={...review,projectRecords:{columns:schema,rows:[]}};
  const request=editor.metadataBlankRequest(empty);
  assert.equal(request.projectId,'Project');
  assert.equal(request.plotNumber,'P1');
  assert.equal(request.original,empty.projectRecords);
  assert.equal('id' in request,false);
  for(const invalid of [review,{...empty,projectId:null},{...empty,projectId:''},{...empty,projectId:'x'.repeat(21)},
    {...empty,projectId:'\ud800'},{...empty,projectRecords:{columns:schema,rows:null}}]) {
    assert.throws(()=>editor.metadataBlankRequest(invalid));
  }
});
test('Metadata raw numeric errors survive serialization/remount, gate requests and clear on valid correction or Undo',()=>{
  const start = field('StartDate','integer',16);
  let draft = editor.beginMetadataDraft(review,'-200');
  for (const raw of ['1e2','32768','1.5','+1',' 1','-0','']) {
    draft = editor.stageMetadata(json(draft),start,raw,false);
    assert.equal(draft.cells.StartDate.raw,raw);
    assert.throws(()=>editor.metadataEditRequest(json(draft)));
  }
  draft = editor.stageMetadata(draft,start,'2024',false);
  assert.equal(editor.metadataErrors(draft).length,0);
  assert.deepEqual(json(editor.metadataEditRequest(draft).changes),[{column:'StartDate',value:cell('integer','2024')}]);
  assert.equal(editor.metadataDirty(editor.beginMetadataDraft(review,'-200')),false);
});
test('Metadata preserves NULL/empty/literal text and enforces exact UTF-16 bounds without truncation',()=>{
  const notes = field('Notes','text',0), title = field('ProjectTitle','text',255);
  let draft = editor.beginMetadataDraft(review,'-200');
  draft = editor.stageMetadata(draft,notes,'',false);
  assert.deepEqual(json(editor.metadataEditRequest(draft).changes[0].value),cell('text',''));
  draft = editor.stageMetadata(draft,notes,'',true);
  assert.equal(editor.metadataDirty(draft),false);
  for (const raw of ['\ud800','\udc00','🌲'.repeat(128)]) {
    draft = editor.stageMetadata(draft,title,raw,false);
    assert.equal(draft.cells.ProjectTitle.raw,raw);
    assert.throws(()=>editor.metadataEditRequest(json(draft)));
  }
  draft = editor.stageMetadata(draft,title,'🌲'.repeat(127)+'x',false);
  assert.equal(editor.metadataErrors(draft).length,0);
  draft = editor.stageMetadata(draft,title,'  Mixed CASE  ',false);
  assert.equal(editor.metadataEditRequest(draft).changes[0].value.text,'  Mixed CASE  ');
});
test('Metadata source collections are three-state options; invalid historical values are omitted unchanged',()=>{
  const collected = field('CollectedSite','integer',16,{collection:true});
  let draft = editor.beginMetadataDraft(review,'-200');
  draft = editor.stageMetadata(draft,collected,'-1',false);
  assert.equal(editor.metadataDirty(draft),false);
  draft = editor.stageMetadata(draft,collected,'0',false);
  assert.throws(()=>editor.metadataEditRequest(draft),/not a BOOLEAN/);
  draft = editor.stageMetadata(draft,collected,'3',false);
  assert.equal(editor.metadataEditRequest(draft).changes[0].value.integer,'3');
});
test('Metadata quality uses literal registered Notes; source-standard population requires an explicit separate decision',()=>{
  const qualityField=field('DataQualityVeg','text',255,{limitToList:true,referenceList:'PlotQualitySite',referenceColumn:'Note',options:[{rowId:'1',value:'Good Note',description:null}]});
  let draft=editor.stageMetadata(editor.beginMetadataDraft(review,'-200'),qualityField,'good note',false);
  assert.throws(()=>editor.metadataEditRequest(draft),/literal registered Note/);
  draft=editor.stageMetadata(draft,qualityField,'Good Note',false);
  draft=editor.stageMetadata(draft,field('EcosysCollectionStandard'),'DEIF 2024',false);
  assert.throws(()=>editor.metadataEditRequest(draft),/Explicitly keep/);
  draft.standardPopulation='keep';
  assert.equal(editor.metadataEditRequest(draft).standardPopulation,'keep');
  draft=editor.stageMetadata(draft,field('EcosysCollectionStandard'),'DTE changed',false);
  assert.equal(draft.standardPopulation,'');
});
test('Metadata source labels/grouping cover each ordinary field once and retain all74 binding relationships',()=>{
  const names=presentation.metadataGroups.flatMap(group=>group.names);
  assert.equal(names.length,70);
  assert.equal(new Set(names).size,70);
  for(const name of names) assert.ok(presentation.metadataLabel(name));
  assert.equal(layout.forms[0].fields.filter(field=>field.column).length,74);
  assert.equal(presentation.metadataLabel('CoverB2aDescription'),'B3 layer description');
  assert.equal(presentation.metadataLabel('GeoRefMethodOther'),'Georeference Method — other');
});
test('Confirmed source-standard population stages exactly24 literals, preserves other raw errors and permits deliberate later field edits',()=>{
  const names=presentation.metadataGroups.flatMap(group=>group.names);
  const complete={...review,projectRecords:{
    columns:[...columns.slice(0,2),...names.map(name=>({name,declaredType:'TEXT'}))],
    rows:[{rowId:'-200',cells:[cell('integer','-200'),cell('text','Project'),...names.map(()=>cell('null'))]}],
  }};
  const fields=names.map(name=>field(name));
  let draft=editor.stageMetadata(editor.beginMetadataDraft(complete,'-200'),field('EcosysCollectionStandard'),'DEIF 2024',false);
  draft=editor.stageMetadata(draft,field('StartDate','integer',16),'1e',false);
  const original=json(draft);
  assert.throws(()=>editor.populateMetadataStandard(draft,fields.filter(f=>f.name!=='Datum')),/unavailable/);
  assert.deepEqual(json(draft),original,'failed population cannot partially replace raw drafts');
  draft=editor.populateMetadataStandard(draft,fields);
  assert.equal(Object.keys(defaults).length,24);
  assert.equal(draft.standardPopulation,'populate');
  for(const [name,value] of Object.entries(defaults)) assert.equal(draft.cells[name].raw,value);
  assert.equal(draft.cells.StartDate.raw,'1e');
  assert.throws(()=>editor.metadataEditRequest(draft),/exact integer/);
  draft=editor.stageMetadata(draft,field('StartDate','integer',16),'2024',false);
  draft=editor.stageMetadata(draft,field('CoordinatingAgency'),'Deliberate override',false);
  assert.equal(editor.metadataEditRequest(draft).standardPopulation,'populate');
  assert.equal(editor.metadataEditRequest(draft).changes.length,26);
  assert.equal(draft.cells.CoordinatingAgency.value.text,'Deliberate override');
  draft=editor.stageMetadata(draft,field('EcosysCollectionStandard'),'DTE later',false);
  assert.equal(draft.standardPopulation,'');
  assert.throws(()=>editor.metadataEditRequest(draft),/confirm/);
  assert.throws(()=>editor.populateMetadataStandard(editor.beginMetadataDraft(complete,'-200'),fields),/changed recognized/);
});
test('Metadata component stays outside remounted tabs and gates ordinary Save/Lock/native-close/context while retaining failed writes',()=>{
  const form=readFileSync(path.join(__dirname,'FS882Form.svelte'),'utf8');
  const component=readFileSync(path.join(__dirname,'ProjectMetadataEditor.svelte'),'utf8');
  assert.match(form,/VITE_PROJECT_METADATA_EDITING === 'true'/);
  assert.match(form,/childUnsaved = \$derived\([^;]*metadataOpen/);
  assert.match(form,/childParentDisabled = \$derived\([^;]*metadataOpen/);
  assert.match(form,/if \(metadataOpen\) return metadataEditor\?\.getCloseState/);
  assert.match(form,/if \(metadataOpen\) \{ metadataEditor\?\.undo\(\); return; \}/);
  assert.match(form,/ordinary plot Save never applies it/);
  assert.match(form,/Finish or close metadata review before changing the plot lock/);
  assert.match(component,/await client\.SaveProjectMetadata\(request\)/);
  assert.match(component,/metadata edit committed, but/);
  assert.match(component,/completed writes must not be replayed/);
  assert.match(component,/Save failed; raw drafts retained/);
  assert.match(form,/VITE_PROJECT_METADATA_CREATION === 'true'/);
  assert.match(component,/blank = metadataBlankRequest\(review\)/);
  assert.match(component,/await client\.CreateBlankProjectMetadata\(blank\)/);
  assert.match(component,/blank = null; error = null; success = 'Blank creation proposal undone/);
  assert.match(component,/reviewed proposal retained for retry/);
  assert.match(component,/committed = true; blank = null/);
  assert.equal(compile(component,{filename:'ProjectMetadataEditor.svelte',generate:'client'}).warnings.length,0);
});
test('Metadata reload invalidates editing readiness until both current records and references load successfully',()=>{
  const component=readFileSync(path.join(__dirname,'ProjectMetadataEditor.svelte'),'utf8');
  assert.match(component,/reading = true; ready = false/);
  assert.match(component,/await Promise\.all\(\[\s*reads\.track\(client\.ReviewProjectMetadata\(plot\)\), reads\.track\(client\.ListProjectMetadataFields\(\)\)/);
  assert.match(component,/review = next; fields = definitions; draft = null; committedFailure = false; ready = true/);
  for(const guard of [
    '!ready || !review || busy || dirty || committedFailure',
    '!ready || !draft || busy || committedFailure',
    'disabled={!ready || busy || dirty || committedFailure}',
    'disabled={!ready || busy || committedFailure || cell.nullValue}',
    'disabled={!ready || busy || committedFailure}',
    'disabled={!ready || busy || !draft || !dirty',
  ]) assert.ok(component.includes(guard),guard);
  assert.match(component,/export function undo\(\) \{\s*if \(busy\)/);
});
