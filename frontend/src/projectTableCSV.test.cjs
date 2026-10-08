const assert = require('node:assert/strict');
const {test} = require('node:test');
const {createHash} = require('node:crypto');
const {readFileSync} = require('node:fs');
const path = require('node:path');
const {compile} = require('svelte/compiler');
const {loadTypeScript} = require('./svelteTestHelpers.cjs');
const quality = loadTypeScript('qualityEditor.ts', {'./becEditor':loadTypeScript('becEditor.ts')});
const cells = loadTypeScript('projectMetadataRestore.ts', {'./qualityEditor':quality,'./projectMetadataEditor':{}});
const csv = loadTypeScript('projectTableCSV.ts', {'./qualityEditor':quality,'./projectMetadataRestore':cells});
const checksum = async data => createHash('sha256').update(data,'utf8').digest('hex');
const copy = value => JSON.parse(JSON.stringify(value));
const cell = (storage,text=null) => ({storage,text,integer:null,real:null,blobHex:null});
const quote = value => /[",\n]/.test(value) ? `"${value.replaceAll('"','""')}"` : value;
function document(records=[['Value'],[''],[''],['""'],['9223372036854775807'],['-0'],['00ff']]) {
  const data=records.map((record,index)=>index && record.length===1 && record[0]==='' ? '""\n' :
    record.map(quote).join(',')+'\n').join('');
  return {contextId:'owner',project:'Sample',projectPath:'C:\\fixture\\Sample.db',descriptionMetadataPresent:true,csv:data,
    manifest:{version:1,table:'Sample_Other',columns:[{name:'Value',declaredType:''}],
      rowIds:['-9007199254740993','2','3','4','5','6'],storage:[['null'],['blob'],['text'],['integer'],['real'],['blob']],
      descriptions:[{rowId:'1',value:cell('null')},{rowId:'2',value:cell('text','')},{rowId:'3',value:cell('text','')}],
      sha256:createHash('sha256').update(data).digest('hex')}};
}
const validate = value => csv.validateProjectTableCSVReview(value,'owner','Sample','C:\\fixture\\Sample.db','Sample_Other',checksum);
function replaceData(value, records) {
  value.csv=records.map(record=>record.map(quote).join(',')+'\n').join('');
  value.manifest.sha256=createHash('sha256').update(value.csv).digest('hex');
}

test('CSV review preserves exact physical rows, single-column NULL/BLOB empties and duplicate nullable descriptions',async()=>{
  const value=document();
  assert.equal(JSON.stringify(await validate(value)),JSON.stringify(value));
  assert.equal(value.manifest.rowIds[0],'-9007199254740993');
  assert.equal(value.manifest.descriptions.length,3);
  assert.equal(value.manifest.descriptions[0].value.storage,'null');
  assert.equal(value.manifest.descriptions[1].value.text,'');
  const empty=document([['Value']]);empty.manifest.rowIds=[];empty.manifest.storage=[];
  assert.equal(JSON.stringify(await validate(empty)),JSON.stringify(empty));
  const absent=copy(empty);absent.descriptionMetadataPresent=false;absent.manifest.descriptions=[];
  assert.equal(JSON.stringify(await validate(absent)),JSON.stringify(absent));
});

test('CSV JSON-string text retains literal whitespace, comma, quotes, CRLF, NUL and astral Unicode',async()=>{
  const text='  literal,"quoted"\r\nline\n\0 \u{1f332}  ';
  const value=document();
  replaceData(value,[['Value'],[JSON.stringify(text)]]);
  value.manifest.rowIds=['1'];value.manifest.storage=[['text']];
  assert.equal(JSON.stringify(await validate(value)),JSON.stringify(value));
  const literal=document();
  replaceData(literal,[['Value'],['"NULL"'],['"."'],['"  "']]);
  literal.manifest.rowIds=['1','2','3'];literal.manifest.storage=[['text'],['text'],['text']];
  assert.equal(JSON.stringify(await validate(literal)),JSON.stringify(literal));
});

test('CSV validation detaches the reviewed snapshot before asynchronous checksum work',async()=>{
  const value=document(), expected=JSON.stringify(value);
  const result=await csv.validateProjectTableCSVReview(value,'owner','Sample','C:\\fixture\\Sample.db','Sample_Other',async data=>{
    value.contextId='changed during checksum';
    value.manifest.columns[0].name='caller changed';
    value.manifest.descriptions[1].value.text='caller changed';
    return checksum(data);
  });
  assert.equal(JSON.stringify(result),expected);
  result.manifest.rowIds[0]='7';
  assert.equal(value.manifest.rowIds[0],'-9007199254740993');
});

test('CSV transport rejects foreign, incomplete, normalized or corrupted reviews before presentation',async()=>{
  const mutations=[
    v=>v.contextId='stale',v=>v.project='Other',v=>v.projectPath='C:\\other\\Sample.db',
    v=>v.descriptionMetadataPresent=undefined,v=>v.descriptionMetadataPresent=false,
    v=>v.manifest.version=2,v=>v.manifest.table='Sample_Env',v=>v.manifest.columns=null,
    v=>v.manifest.columns[0].name='Wrong',v=>v.manifest.columns[0].declaredType=null,
    v=>v.manifest.rowIds[0]='+1',v=>v.manifest.rowIds[0]='9223372036854775808',
    v=>v.manifest.rowIds[1]=v.manifest.rowIds[0],v=>v.manifest.rowIds.pop(),
    v=>v.manifest.storage[0]=null,v=>v.manifest.storage[0]=[],
    v=>v.manifest.storage[0][0]='boolean',v=>v.manifest.descriptions=null,
    v=>v.manifest.descriptions[1].rowId='1',v=>v.manifest.descriptions[1].value.text='\ud800',
    v=>v.csv+='\n',v=>v.csv=v.csv.replaceAll('\n','\r\n'),v=>v.csv+='\ud800',
    v=>v.manifest.sha256='0'.repeat(64),v=>v.manifest.sha256='F'.repeat(64)
  ];
  for(const mutate of mutations) {
    const value=document();mutate(value);
    await assert.rejects(validate(value));
  }
  for(const [tag,raw] of [['text','"\\ud800"'],['text','null'],['integer','01'],['integer','+1'],
    ['integer','9223372036854775808'],['real','NaN'],['real','1e999'],['real',''],
    ['blob','FF'],['blob','0'],['null','NULL']]) {
    const value=document();
    replaceData(value,[['Value'],[raw]]);
    value.manifest.rowIds=['1'];value.manifest.storage=[[tag]];
    await assert.rejects(validate(value));
  }
  for(const data of ['Value\n\n','Value\n"unterminated\n','Value\n"closed"junk\n','Value\n7']) {
    const value=document();value.csv=data;value.manifest.rowIds=['1'];value.manifest.storage=[['integer']];
    value.manifest.sha256=createHash('sha256').update(data).digest('hex');
    await assert.rejects(validate(value));
  }
});

test('CSV UI is independently default-off and shares existing draft, context and busy-close navigation',()=>{
  const component=readFileSync(path.join(__dirname,'ProjectTableCSVReview.svelte'),'utf8');
  const app=readFileSync(path.join(__dirname,'App.svelte'),'utf8');
  const navigation=readFileSync(path.join(__dirname,'Navigation.svelte'),'utf8');
  assert.equal(compile(component,{filename:'ProjectTableCSVReview.svelte',generate:'client'}).warnings.length,0);
  assert.match(navigation,/VITE_TABLE_CSV_REVIEW === 'true' \? 'table-csv' : undefined/);
  assert.match(app,/view === 'table-csv' && import\.meta\.env\.VITE_TABLE_CSV_REVIEW === 'true'/);
  assert.match(app,/function navigate[\s\S]*?requestTransition/);
  assert.match(app,/busy \|\| editorBusy \|\| transitionWorking/);
  assert.match(component,/reads\.track\(ContextService\.GetProjectTableCSVReview\(contextId, table\)\)/);
  assert.match(component,/const validated = await validateProjectTableCSVReview[\s\S]*?if \(request !== generation\) return;[\s\S]*?review = validated/);
  assert.match(component,/onDestroy\(cancel\)/);
  assert.match(component,/reads\.cancelAll\(\)/);
  assert.match(component,/<textarea id="csv-value-preview" readonly/);
  assert.match(component,/File publication, import, RDS and TurboVeg remain unavailable/);
  assert.doesNotMatch(component,/Save|download=|CreateBlank|SaveProject/);
});
