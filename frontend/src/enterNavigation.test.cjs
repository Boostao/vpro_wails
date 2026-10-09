const assert = require('node:assert/strict');
const {readFileSync} = require('node:fs');
const path = require('node:path');
const {test} = require('node:test');
const {loadTypeScript} = require('./svelteTestHelpers.cjs');
const navigation = loadTypeScript('enterNavigation.ts');
const position = (extra={}) => ({form:null,record:null,column:'FieldNo',enabled:true,visible:true,invalid:false,...extra});

test('Enter field navigation skips disabled and actually hidden controls without wrapping or allocating a record',()=>{
  const fields=[position(),position({enabled:false}),position({visible:false}),position({column:'StartDate'})];
  assert.equal(navigation.nextEnterPosition(fields,0,'field'),3);
  assert.equal(navigation.nextEnterPosition(fields,3,'field'),null);
  for(const key of ['invalid','enabled','visible']) {
    const blocked=[position({[key]:key==='invalid'}),position()];
    assert.equal(navigation.nextEnterPosition(blocked,0,'field'),null);
  }
  for(const index of [-1,fields.length,NaN,0.5]) assert.throws(()=>navigation.nextEnterPosition(fields,index,'field'),/currently observed/);
});

test('Source Enter record navigation retains the literal column and physical form while skipping unavailable records',()=>{
  const source=position({form:'SubVegCXL',record:'-12',column:'Cover6'});
  const fields=[source,position({...source,column:'Species'}),position({...source,record:'-11',column:'Species'}),
    position({...source,record:'-11',enabled:false}),position({...source,record:'-10',visible:false}),
    position({...source,form:'SubVegAXL_BC',record:'-9'}),position({...source,record:'-8'})];
  assert.equal(navigation.nextEnterPosition(fields,0,'record'),6);
  assert.equal(navigation.nextEnterPosition(fields,6,'record'),null);
  assert.equal(navigation.nextEnterPosition([source,position({...source,record:null})],0,'record'),null);
  assert.equal(navigation.nextEnterPosition([source,position({...source,column:'cover6',record:'-8'})],0,'record'),null);
  assert.throws(()=>navigation.nextEnterPosition([position()],0,'record'),/existing physical record/);
  assert.throws(()=>navigation.nextEnterPosition([position({column:''})],0,'field'),/literal column/);
});

test('Five linked next-record paths and the shared extended A presentation are enabled; stale handlers are not inferred',()=>{
  const fixture=readFileSync(path.join(__dirname,'..','..','testdata','navigation','enter-mode.vba'),'utf8').replace(/\r\n/g,'\n');
  const source=JSON.parse(readFileSync(path.join(__dirname,'..','..','resources','fs882-xl-layout.json'),'utf8')).forms[0];
  assert.match(fixture,/RunEnterKeyActionNF\(\)[\s\S]*?Application\.SetOption "Move After Enter", 1/);
  assert.match(fixture,/RunEnterKeyActionNR\(\)[\s\S]*?Application\.SetOption "Move After Enter", 2/);
  const linked=[];
  for(const control of source.controls) {
    const entered=control.events?.find(event=>event.property==='OnEnter');
    if(entered && fixture.includes(`Private Sub ${entered.procedure}()\n    RunEnterKeyActionNR`)) {
      linked.push(control.properties.SourceObject.value.replace(/^Form\./,''));
    }
  }
  assert.deepEqual([...linked, 'SubVegAXL'].sort(),[...navigation.sourceNextRecordForms].sort());
  assert.equal(navigation.enterNavigationMode(null),'field');
  for(const form of ['SubVegAXL_BC','SubVegAXL','SubVegCXL','SubVegDXL','SoilHumusXL','SoilMineralXL']) {
    assert.equal(navigation.enterNavigationMode(form),'record');
  }
  for(const form of ['SubVegAhtXL','SubVegChtXL','USysVegOtherXL','SubOtherXL','Child162','Child164','Child165']) {
    assert.equal(navigation.enterNavigationMode(form),null);
  }
});

test('Enter adapter preserves native list/select/multiline/IME shortcuts and uses real visibility and fieldset disabling',()=>{
  const source=readFileSync(path.join(__dirname,'enterNavigation.ts'),'utf8');
  const form=readFileSync(path.join(__dirname,'FS882Form.svelte'),'utf8');
  for(const guard of ['event.isComposing','event.repeat','event.altKey','event.ctrlKey','event.metaKey','event.shiftKey',
    'source.list',"source.closest('dialog,[role=dialog]')","control.matches(':disabled')",'control.checkVisibility()',
    'control.readOnly',"control.getAttribute('aria-invalid') === 'true'"]) assert.ok(source.includes(guard),guard);
  assert.doesNotMatch(source,/\.click\(|\.submit\(|Save|Create|ByID|fetch\(/);
  assert.match(form,/VITE_SOURCE_ENTER_NAVIGATION === 'true'/);
  assert.match(form,/<svelte:window onkeydown=\{sourceEnter\}/);
  assert.match(form,/error = `Source Enter navigation failed/);
  assert.match(source,/element instanceof HTMLTextAreaElement/);
  assert.match(source,/textarea\[data-column\]/);
});
