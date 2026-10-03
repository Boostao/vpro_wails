const assert = require('node:assert/strict');
const {test} = require('node:test');
const {readFileSync} = require('node:fs');
const path = require('node:path');
const {compile} = require('svelte/compiler');
const {loadTypeScript} = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', {'./becEditor': loadTypeScript('becEditor.ts')});
const editor = loadTypeScript('projectMetadataEditor.ts', {
  './qualityEditor': quality,
  '../../resources/project-metadata-standard.json': JSON.parse(readFileSync(path.join(__dirname,'..','..','resources','project-metadata-standard.json'))),
  '../../resources/project-metadata-template.json': JSON.parse(readFileSync(path.join(__dirname,'..','..','resources','project-metadata-template.json')))
});
const restoration = loadTypeScript('projectMetadataRestore.ts', {'./qualityEditor': quality, './projectMetadataEditor': editor});
const transfer = loadTypeScript('environmentSUTransfer.ts', {'./projectMetadataRestore': restoration, './projectMetadataEditor': editor});
const read = name => readFileSync(path.join(__dirname,name),'utf8');
const cell = text => ({storage:text===null?'null':'text',text,integer:null,real:null,blobHex:null});
const table = (names,cells) => ({columns:names.map(name=>({name,declaredType:'VARCHAR'})),rows:cells.map((cells,i)=>({rowId:String(i+1),cells}))});
function review() {
  return {contextId:'owned',project:'Sample',su:'Selected',path:"C:\\owned O'Brien.db",historyHash:'',
    env:table(['PlotNumber'],[[cell('P1')],[cell('P2')]]),
    admin:table(['Plot','UserSiteUnit'],[[cell('P1'),cell('  Literal  ')],[cell('P2'),cell(null)]]),
    target:table(['PlotNumber','SiteUnit'],[[cell('P1'),cell(null)],[cell('P2'),cell('')]]),
    changes:[
      {plotNumber:'P1',envRowId:'1',adminRowId:'1',suRowId:'1',before:cell(null),after:cell('  Literal  ')},
      {plotNumber:'P2',envRowId:'2',adminRowId:'2',suRowId:'2',before:cell(''),after:cell(null)}
    ]};
}
const validate = r => transfer.validateEnvironmentSUReview(r,'owned','Sample','Selected',"C:\\owned O'Brien.db");
const request = r => transfer.environmentSURequest(r,'owned','Sample','Selected',"C:\\owned O'Brien.db");

test('SU transfer preserves original NULL/empty/text and independently snapshots reviewed physical links',()=>{
  const r=review(), payload=request(r);
  assert.equal(payload.confirmed,true);
  assert.equal(payload.review.changes[0].before.storage,'null');
  assert.equal(payload.review.changes[1].before.text,'');
  assert.equal(payload.review.changes[1].after.storage,'null');
  r.admin.rows[0].cells[1].text='Later mutation';
  assert.equal(payload.review.admin.rows[0].cells[1].text,'  Literal  ');
  assert.throws(()=>request({...review(),changes:[]}),/nonempty/);
});
test('SU transfer rejects foreign context, paths, repeated physical links and incomplete/repaired tagged cells',()=>{
  for(const mutate of [
    r=>r.contextId='stale',r=>r.su='None',r=>r.path='C:\\foreign.db',
    r=>r.env.rows[0].rowId='01',r=>r.admin.columns[1].name='Plot',
    r=>r.admin.rows.push({...r.admin.rows[0],rowId:'3'}),
    r=>r.changes.push({...r.changes[0]}),r=>r.changes[0].envRowId='2',
    r=>r.changes[0].plotNumber='p1',r=>r.changes[0].after.text='\ud800',
    r=>r.changes[0].before.integer='7',r=>r.target.rows[0].cells=null,
    r=>r.changes[0].after.text='x'.repeat(256)
  ]) {
    const r=review(); mutate(r); assert.throws(()=>validate(r));
  }
});
test('SU transfer committed results require exact count and positive signed64 history identity',()=>{
  const payload=request(review());
  assert.doesNotThrow(()=>transfer.validateEnvironmentSUResult({changedRows:2,historyId:'9007199254740993'},payload));
  for(const result of [null,{changedRows:1,historyId:'1'},{changedRows:2,historyId:'01'},{changedRows:2,historyId:'0'}]) {
    assert.throws(()=>transfer.validateEnvironmentSUResult(result,payload),/without replaying/);
  }
});
test('SU transfer remains independently gated and shared lifecycle guards prevent ordinary writes and replay',()=>{
  for(const file of ['EnvironmentSUTransfer.svelte','FS882Form.svelte']) {
    assert.equal(compile(read(file),{filename:file,generate:'client'}).warnings.length,0);
  }
  const panel=read('EnvironmentSUTransfer.svelte'), root=read('FS882Form.svelte');
  assert.match(root,/VITE_SOURCE_ENV_SU_TRANSFER === 'true'/);
  assert.match(root,/headerWorkflowBusy = \$derived\([^;]+environmentSUOpen/);
  assert.match(root,/if \(environmentSUOpen\) return environmentSUEditor\?\.getCloseState/);
  assert.match(root,/if \(environmentSUOpen\) \{ environmentSUEditor\?\.undo/);
  assert.match(root,/data-source-control="btnEnvIntoSu"/);
  assert.match(panel,/\$state\.snapshot\(review\)/);
  assert.match(panel,/committed = true; review = null;[\s\S]*validateEnvironmentSUResult/);
  assert.match(panel,/review kept for retry/);
  assert.match(panel,/not only the displayed plot or profile-navigation subset/);
  assert.doesNotMatch(panel,/window\.confirm/);
});
