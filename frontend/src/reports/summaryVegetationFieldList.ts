export const fieldListSource = 'owned imported SQLite VPro64.USysShortVegSummaryFieldList; complete physical rows; Label read-only';
export const fieldListLimits = Object.freeze({ rows: 10000, actions: 4096, receipts: 128, jsonUnits: 16 * 1024 * 1024 });
const columnNames = ['Order', 'Source', 'Label', 'Item', 'Report', 'FunctionName', 'FieldNumber'];
const ownerKeys = ['contextId', 'project', 'projectPath', 'su', 'suPath', 'sourcePath'];
export interface FieldListOwner {
  contextId: string; project: string; projectPath: string; su: string; suPath: string; sourcePath: string;
}
export interface FieldListCell {
  storage: 'null' | 'text' | 'integer' | 'real' | 'blob';
  text: string | null; integer: string | null; real: number | null; blobHex: string | null;
}
export interface FieldListRow { rowId: string; cells: FieldListCell[] }
export interface FieldListTable { columns: { name: string; declaredType: string }[]; rows: FieldListRow[] }
export interface FieldListReview extends FieldListOwner { source: string; physical: FieldListTable }
export interface FieldListAction { kind: 'row' | 'all' | 'none'; rowId: string; selected: boolean }
export interface FieldListPlan {
  original: FieldListTable; proposed: FieldListTable;
  assignments: { before: FieldListRow; after: FieldListRow }[];
}
export interface FieldListRequest { requestId: string; review: FieldListReview; actions: FieldListAction[] }
export interface FieldListOutcome {
  contextId: string; requestId: string; changed: boolean; committed: boolean; replayed: boolean;
  verified: boolean; plan: FieldListPlan | null; errorMessage: string;
}
export interface FieldListIndex { visible: string[]; selected: string[]; error: string }
export interface FieldListReceipt {
  kind: 'known' | 'unknown'; request: FieldListRequest; outcome: FieldListOutcome | null;
  error: string; reconciliation: FieldListReview | null; acknowledged: boolean;
}
export interface FieldListRead<T> extends PromiseLike<T> { cancel?(): void }
export interface FieldListPort {
  read(): FieldListRead<unknown>;
  save(requestJSON: string): FieldListRead<unknown>;
  validationYield?(): PromiseLike<void>;
}
export interface FieldListView {
  review: FieldListReview | null; proposed: FieldListTable | null; actions: FieldListAction[];
  reconciliationReview: FieldListReview | null;
  sourceIndex: FieldListIndex; proposedIndex: FieldListIndex;
  busy: boolean; dispatchSettled: boolean; readError: string; draftError: string; operationError: string;
  receipts: FieldListReceipt[]; revision: number;
}
function fail(message: string): never { throw new Error(message); }
function unicode(value: string): boolean {
  for (let i = 0; i < value.length; i++) {
    const c = value.charCodeAt(i);
    if (c >= 0xd800 && c <= 0xdbff) {
      const next = value.charCodeAt(++i);
      if (!(next >= 0xdc00 && next <= 0xdfff)) return false;
    } else if (c >= 0xdc00 && c <= 0xdfff) return false;
  }
  return true;
}
function text(value: unknown, nonempty = false, noNul = false): string {
  if (typeof value !== 'string' || value.length > fieldListLimits.jsonUnits || !unicode(value) ||
      nonempty && value === '' || noNul && value.includes('\0'))
    fail('Malformed literal Unicode string.');
  return value;
}
function object(value: unknown, keys: string[]): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) fail('Expected exact object.');
  const own = Reflect.ownKeys(value);
  if (own.length !== keys.length || own.some(key => typeof key !== 'string' || !keys.includes(key)))
    fail('Missing, duplicate or foreign object property.');
  return value as Record<string, unknown>;
}
function array(value: unknown, limit: number): unknown[] {
  if (!Array.isArray(value) || value.length > limit) fail('Missing or oversized explicit array.');
  if (Reflect.ownKeys(value).length !== value.length + 1 ||
      !Array.from({ length: value.length }, (_, i) => Object.hasOwn(value, i)).every(Boolean))
    fail('Sparse or decorated arrays are unavailable.');
  return value;
}
function signed64(value: unknown): string {
  const s = text(value);
  if (s.length > 20 || !/^(?:0|-?[1-9][0-9]*)$/.test(s)) fail('Noncanonical signed64 identity/value.');
  const n = BigInt(s);
  if (n < -(1n << 63n) || n >= 1n << 63n) fail('Out-of-range signed64 identity/value.');
  return s;
}
function boolean(value: unknown): boolean {
  if (typeof value !== 'boolean') fail('Expected explicit Boolean.');
  return value;
}
// Copies only already-validated primitive trees: no reactive proxy enters structuredClone.
function copy<T>(value: T): T {
  if (Array.isArray(value)) return value.map(item => copy(item)) as T;
  if (value !== null && typeof value === 'object') {
    const result: Record<string, unknown> = {};
    for (const [key, item] of Object.entries(value)) result[key] = copy(item);
    return result as T;
  }
  return value;
}
function freeze<T>(value: T): T {
  if (value !== null && typeof value === 'object') {
    for (const item of Object.values(value)) freeze(item);
    Object.freeze(value);
  }
  return value;
}
function same(a: unknown, b: unknown): boolean {
  if (Object.is(a, b)) return true;
  if (!a || !b || typeof a !== 'object' || typeof b !== 'object' || Array.isArray(a) !== Array.isArray(b)) return false;
  const left = Object.keys(a), right = Object.keys(b);
  return left.length === right.length && left.every(key => Object.hasOwn(b, key) &&
    same((a as Record<string, unknown>)[key], (b as Record<string, unknown>)[key]));
}
export function fieldListOwner(value: unknown): FieldListOwner {
  const v = object(value, ownerKeys);
  const owner = { contextId: text(v.contextId, true, true), project: text(v.project, true, true),
    projectPath: text(v.projectPath, true, true), su: text(v.su, true, true), suPath: text(v.suPath, false, true),
    sourcePath: text(v.sourcePath, true, true) };
  if (owner.su !== 'None' && owner.suPath === '') fail('Selected unit requires complete literal path provenance.');
  return freeze(owner);
}
export function fieldListCell(value: unknown): FieldListCell {
  const v = object(value, ['storage', 'text', 'integer', 'real', 'blobHex']);
  const storage = text(v.storage);
  if (!['null', 'text', 'integer', 'real', 'blob'].includes(storage)) fail('Unknown physical storage tag.');
  for (const key of ['text', 'integer', 'real', 'blobHex']) {
    const active = storage === key || storage === 'blob' && key === 'blobHex';
    if (!active && v[key] !== null) fail('Contradictory physical storage payload.');
  }
  let cell: FieldListCell = { storage: storage as FieldListCell['storage'], text: null, integer: null, real: null, blobHex: null };
  if (storage === 'text') cell.text = text(v.text);
  if (storage === 'integer') cell.integer = signed64(v.integer);
  if (storage === 'real') {
    if (typeof v.real !== 'number' || !Number.isFinite(v.real)) fail('Nonfinite physical REAL.');
    cell.real = v.real;
  }
  if (storage === 'blob') {
    const hex = text(v.blobHex);
    if (!/^(?:[0-9a-f]{2})*$/.test(hex)) fail('Noncanonical physical BLOB.');
    cell.blobHex = hex;
  }
  return cell;
}
function row(value: unknown): FieldListRow {
  const v = object(value, ['rowId', 'cells']);
  const cells = array(v.cells, 7);
  if (cells.length !== 7) fail('All seven physical cells are required.');
  return { rowId: signed64(v.rowId), cells: cells.map(fieldListCell) };
}
export function fieldListTable(value: unknown): FieldListTable {
  const v = object(value, ['columns', 'rows']);
  const columns = array(v.columns, 7).map((item, i) => {
    const c = object(item, ['name', 'declaredType']);
    if (c.name !== columnNames[i]) fail('Exact source column names/order required.');
    return { name: text(c.name), declaredType: text(c.declaredType, false, true) };
  });
  if (columns.length !== 7) fail('Exact seven physical columns required.');
  const rows = array(v.rows, fieldListLimits.rows).map(row);
  for (let i = 1; i < rows.length; i++)
    if (BigInt(rows[i - 1].rowId) >= BigInt(rows[i].rowId)) fail('Duplicate or unordered physical rowids.');
  return { columns, rows };
}
export function validateFieldListReview(value: unknown, owner: FieldListOwner): FieldListReview {
  const v = object(fromWire(value), [...ownerKeys, 'source', 'physical']);
  const scope: Record<string, unknown> = {};
  for (const key of ownerKeys) scope[key] = v[key];
  const observed = fieldListOwner(scope);
  if (!same(observed, fieldListOwner(owner)) || v.source !== fieldListSource) fail('Foreign field-list owner or source provenance.');
  const reviewed = { ...observed, source: fieldListSource, physical: fieldListTable(v.physical) };
  fieldListJSON(reviewed);
  return freeze(reviewed);
}
export function fieldListIndex(table: FieldListTable): FieldListIndex {
  try {
    const entries = table.rows.map(r => {
      const order = r.cells[0], report = r.cells[4];
      let n: number | null = null;
      if (order.storage !== 'null') {
        if (order.storage !== 'integer' || order.integer === null ||
            BigInt(order.integer) < -32768n || BigInt(order.integer) > 32767n) fail(`Row ${r.rowId} Order unavailable without source signed16 coercion.`);
        n = Number(order.integer);
      }
      if (report.storage !== 'null' && (report.storage !== 'integer' || !['-1', '0'].includes(report.integer ?? '')))
        fail(`Row ${r.rowId} Report unavailable without Boolean normalization.`);
      return { id: r.rowId, order: n, selected: report.storage === 'integer' && report.integer === '-1' };
    });
    entries.sort((a, b) => a.order === null ? b.order === null ? 0 : -1 : b.order === null ? 1 : a.order - b.order);
    return { visible: entries.filter(e => e.order !== null && e.order > 0).map(e => e.id),
      selected: entries.filter(e => e.selected).map(e => e.id), error: '' };
  } catch (cause) { return { visible: [], selected: [], error: String(cause) }; }
}
function action(value: unknown): FieldListAction {
  const v = object(value, ['kind', 'rowId', 'selected']);
  if (!['row', 'all', 'none'].includes(text(v.kind))) fail('Unavailable source action.');
  const kind = v.kind as FieldListAction['kind'], selected = boolean(v.selected);
  const rowId = kind === 'row' ? signed64(v.rowId) : text(v.rowId);
  if (kind !== 'row' && (rowId !== '' || selected !== (kind === 'all'))) fail('Bulk action must match exact source semantics.');
  return { kind, rowId, selected };
}
export function planFieldList(tableValue: FieldListTable, actionsValue: unknown): FieldListPlan {
  const original = fieldListTable(tableValue), proposed = copy(original);
  const actions = array(actionsValue, fieldListLimits.actions).map(action);
  for (const a of actions) {
    let matched = a.kind !== 'row';
    for (let i = 0; i < original.rows.length; i++) {
      const r = original.rows[i];
      if (a.kind === 'row' && r.rowId !== a.rowId) continue;
      if (a.kind === 'row') {
        const order = r.cells[0], report = r.cells[4];
        if (order.storage !== 'integer' || order.integer === null || BigInt(order.integer) <= 0n || BigInt(order.integer) > 32767n ||
            report.storage !== 'integer' || !['-1', '0'].includes(report.integer ?? ''))
          fail('Row checkbox requires original positive signed16 Order and nonNULL calibrated Report.');
        matched = true;
      }
      proposed.rows[i].cells[4] = { storage: 'integer', text: null, integer: a.selected ? '-1' : '0', real: null, blobHex: null };
    }
    if (!matched) fail('Row action is absent from complete physical review.');
  }
  return freeze({ original, proposed, assignments: proposed.rows.flatMap((r, i) =>
    same(r.cells[4], original.rows[i].cells[4]) ? [] : [{ before: copy(original.rows[i]), after: copy(r) }]) });
}
const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
export function fieldListRequest(value: unknown, owner: FieldListOwner): FieldListRequest {
  const v = object(fromWire(value), ['requestId', 'review', 'actions']);
  const requestId = text(v.requestId);
  if (!uuidPattern.test(requestId)) fail('Save requires canonical lowercase UUIDv4.');
  const review = validateFieldListReview(v.review, owner), actions = array(v.actions, fieldListLimits.actions).map(action);
  planFieldList(review.physical, actions);
  return freeze({ requestId, review, actions });
}
export function validateFieldListOutcome(value: unknown, request: FieldListRequest): FieldListOutcome {
  const v = object(fromWire(value), ['contextId', 'requestId', 'changed', 'committed', 'replayed', 'verified', 'plan', 'errorMessage']);
  if (v.contextId !== request.review.contextId || v.requestId !== request.requestId) fail('Foreign Save acknowledgement.');
  const changed = boolean(v.changed), committed = boolean(v.committed), replayed = boolean(v.replayed), verified = boolean(v.verified);
  const errorMessage = text(v.errorMessage, false, true);
  const expected = planFieldList(request.review.physical, request.actions);
  let plan: FieldListPlan | null = null;
  if (v.plan !== null) {
    const p = object(v.plan, ['original', 'proposed', 'assignments']);
    plan = { original: fieldListTable(p.original), proposed: fieldListTable(p.proposed),
      assignments: array(p.assignments, fieldListLimits.rows).map(item => {
        const a = object(item, ['before', 'after']);
        return { before: row(a.before), after: row(a.after) };
      }) };
    if (!same(plan, expected)) fail('Save plan differs from independent complete source action plan.');
  }
  if (committed) {
    if (!changed || !plan || expected.assignments.length === 0) fail('Contradictory committed physical plan.');
  } else if (changed || replayed || (plan !== null && (!verified || expected.assignments.length !== 0)) ||
      (verified && (!plan || errorMessage === '' && expected.assignments.length !== 0)) || (!verified && !errorMessage))
    fail('Contradictory precommit/noop acknowledgement.');
  const outcome = { contextId: request.review.contextId, requestId: request.requestId, changed, committed, replayed,
    verified, plan, errorMessage };
  fieldListJSON(outcome);
  return freeze(outcome);
}
// Tokenize before JSON.parse: decoded duplicate keys and Unicode repair cannot become authority.
export function fromWire(value: unknown): unknown {
  if (typeof value !== 'string') return value;
  if (value.length > fieldListLimits.jsonUnits || !unicode(value)) fail('Malformed or oversized raw JSON.');
  let pos = 0;
  const ws = () => { while (/\s/.test(value[pos] ?? '') && pos < value.length) pos++; };
  const string = (): string => {
    const start = pos++;
    while (pos < value.length) {
      if (value[pos] === '\\') { pos += 2; continue; }
      if (value[pos++] === '"') return text(JSON.parse(value.slice(start, pos)));
    }
    return fail('Unterminated JSON string.');
  };
  const visit = (depth: number): void => {
    if (depth > 32) fail('JSON nesting exceeds bounded physical contract.');
    ws();
    if (value[pos] === '"') { string(); return; }
    if (value[pos] === '{' || value[pos] === '[') {
      const isObject = value[pos++] === '{', end = isObject ? '}' : ']', seen = new Set<string>();
      ws(); if (value[pos] === end) { pos++; return; }
      for (;;) {
        if (isObject) {
          ws(); if (value[pos] !== '"') fail('Expected JSON property.');
          const key = string();
          if (seen.has(key)) fail('Duplicate decoded JSON property.');
          seen.add(key); ws(); if (value[pos++] !== ':') fail('Expected JSON colon.');
        }
        visit(depth + 1); ws();
        const separator = value[pos++];
        if (separator === end) return;
        if (separator !== ',') fail('Expected JSON separator.');
      }
    }
    const start = pos;
    while (pos < value.length && !/[\s,\]}]/.test(value[pos])) pos++;
    if (start === pos) fail('Missing JSON scalar.');
    JSON.parse(value.slice(start, pos));
  };
  visit(0); ws(); if (pos !== value.length) fail('Trailing JSON input.');
  return JSON.parse(value);
}
export function fieldListJSON(value: unknown): string {
  const encode = (v: unknown): string => {
    if (v === null) return 'null';
    if (typeof v === 'string') return JSON.stringify(text(v));
    if (typeof v === 'boolean') return String(v);
    if (typeof v === 'number' && Number.isFinite(v)) return Object.is(v, -0) ? '-0' : String(v);
    if (Array.isArray(v)) return `[${v.map(encode).join(',')}]`;
    if (typeof v === 'object' && v) return `{${Object.entries(v).map(([key, item]) => `${JSON.stringify(key)}:${encode(item)}`).join(',')}}`;
    return fail('Non-JSON physical value.');
  };
  const encoded = encode(value);
  if (encoded.length > fieldListLimits.jsonUnits) fail('Request exceeds bounded JSON capacity.');
  return encoded;
}
export class SummaryVegetationFieldListSession {
  private readonly owner: FieldListOwner;
  private review: FieldListReview | null = null;
  private actions: FieldListAction[] = [];
  private receipts: FieldListReceipt[] = [];
  private readError = '';
  private draftError = '';
  private operationError = '';
  private busy = false;
  private dispatchSettled = true;
  private generation = 0;
  private revision = 0;
  private pending: FieldListRead<unknown> | null = null;
  private activeSave: FieldListRequest | null = null;
  private correctionReview: FieldListReview | null = null;
  private disposed = false;
  private readonly ids = new Set<string>();
  private readonly listeners = new Set<() => void>();
  constructor(owner: FieldListOwner, private readonly port: FieldListPort, private readonly writingEnabled = false,
    private readonly newRequestId: () => string = () => globalThis.crypto.randomUUID()) {
    this.owner = fieldListOwner(owner);
  }
  view(): FieldListView {
    const plan = this.review ? planFieldList(this.review.physical, this.actions) : null;
    return freeze({ review: copy(this.review), proposed: plan?.proposed ?? null, actions: copy(this.actions),
      reconciliationReview: copy(this.correctionReview),
      sourceIndex: this.review ? fieldListIndex(this.review.physical) : { visible: [], selected: [], error: '' },
      proposedIndex: plan ? fieldListIndex(plan.proposed) : { visible: [], selected: [], error: '' },
      busy: this.busy, dispatchSettled: this.dispatchSettled, readError: this.readError, draftError: this.draftError,
      operationError: this.operationError, receipts: copy(this.receipts), revision: this.revision });
  }
  subscribe(listener: () => void): () => void {
    this.alive(); this.listeners.add(listener); listener();
    return () => { this.listeners.delete(listener); };
  }
  private changed() { this.revision++; for (const listener of this.listeners) listener(); }
  private alive() { if (this.disposed) fail('Field-list session disposed.'); }
  private idle() { this.alive(); if (this.busy || !this.dispatchSettled) fail('Wait for field-list operation/delivery settlement.'); }
  private outstanding() { return this.receipts.some(r => !r.acknowledged); }
  closeState() {
    const dirty = this.actions.length !== 0;
    const blocked = !!this.readError || !!this.draftError || this.outstanding() || !this.dispatchSettled;
    const saveReason = this.busy || !this.dispatchSettled ? 'Wait for the field-list operation/delivery.'
      : this.outstanding() ? 'Reconcile/correct and acknowledge retained Save receipt; do not replay.'
      : this.readError || this.draftError || (!this.review ? 'Explicitly Load complete field-list authority.'
        : !this.writingEnabled ? 'Private field-list writing disabled.' : this.receipts.length >= fieldListLimits.receipts
          ? 'Retained receipt capacity reached; no evidence is dropped.' : !dirty ? 'No staged source actions.' : '');
    return { busy: this.busy || !this.dispatchSettled, unsaved: dirty || this.outstanding(), blocked,
      canSave: saveReason === '', saveReason, error: this.readError || this.draftError || this.operationError };
  }
  setActions(value: unknown): boolean {
    this.idle();
    if (this.outstanding()) fail('Correct retained receipt before replacing its action draft.');
    if (!this.writingEnabled || !this.review) fail('Enabled owned Load required before staging.');
    try {
      const actions = array(value, fieldListLimits.actions).map(action);
      planFieldList(this.review.physical, actions);
      this.actions = freeze(actions); this.draftError = ''; this.operationError = '';
      this.changed(); return true;
    } catch (cause) { this.draftError = String(cause); this.changed(); return false; }
  }
  stageRow(rowId: string, selected: boolean) { return this.setActions([...this.actions, { kind: 'row', rowId, selected }]); }
  stageAll() { return this.setActions([...this.actions, { kind: 'all', rowId: '', selected: true }]); }
  stageNone() { return this.setActions([...this.actions, { kind: 'none', rowId: '', selected: false }]); }
  undo() {
    this.idle();
    this.actions = []; this.draftError = ''; this.operationError = ''; this.changed();
  }
  private begin() { this.busy = true; this.operationError = ''; const generation = ++this.generation; this.changed(); return generation; }
  private finish(generation: number) { if (generation === this.generation) { this.pending = null; this.busy = false; this.changed(); } }
  async load(): Promise<boolean> {
    this.idle();
    if (this.actions.length && !this.outstanding()) fail('Undo retained actions before explicit Load.');
    const generation = this.begin();
    this.correctionReview = null;
    try {
      if (generation !== this.generation) return false;
      const request = this.port.read(); this.pending = request;
      const value = await request;
      if (generation !== this.generation) return false;
      const reviewed = validateFieldListReview(value, this.owner);
      await this.port.validationYield?.();
      if (generation !== this.generation) return false;
      if (this.outstanding()) this.correctionReview = reviewed;
      else this.review = reviewed;
      this.readError = '';
      return true;
    } catch (cause) {
      if (generation === this.generation) this.readError = `Owned field-list read failed; no repaired/default authority: ${String(cause)}`;
      return false;
    } finally { this.finish(generation); }
  }
  private unknown(request: FieldListRequest, cause: unknown) {
    if (!this.receipts.some(r => r.request.requestId === request.requestId)) this.receipts.push({
      kind: 'unknown', request, outcome: null, error: String(cause), reconciliation: null, acknowledged: false });
    this.correctionReview = null;
    this.operationError = 'Save delivery/acknowledgement unknown; never replay. Wait, explicitly Load and correct the retained receipt.';
  }
  cancel() {
    this.alive(); if (!this.busy) return;
    this.generation++; this.busy = false;
    if (this.activeSave) this.unknown(this.activeSave, 'Save cancelled after dispatch; outcome unknown.');
    else { this.readError = 'Owned Load cancelled; prior authority retained, explicit Load required.'; this.correctionReview = null; }
    const pending = this.pending; this.pending = null;
    try { pending?.cancel?.(); } catch (cause) { this.operationError += ` Cancellation failed: ${String(cause)}`; }
    this.changed();
  }
  detach() { this.cancel(); this.generation++; }
  async save(): Promise<boolean> {
    this.idle();
    const close = this.closeState();
    if (!close.canSave || !this.review) fail(close.saveReason);
    const id = this.newRequestId();
    const request = fieldListRequest({ requestId: id, review: this.review, actions: this.actions }, this.owner);
    if (this.ids.has(id)) fail('Save UUID was already used; never repeat an attempt.');
    const json = fieldListJSON(request);
    const generation = this.begin();
    if (generation !== this.generation) return false;
    this.ids.add(id);
    this.activeSave = request; this.dispatchSettled = false; this.correctionReview = null;
    try {
      const call = this.port.save(json); this.pending = call;
      const value = await call;
      if (generation !== this.generation) return false;
      const outcome = validateFieldListOutcome(value, request);
      await this.port.validationYield?.();
      if (generation !== this.generation) return false;
      this.receipts.push({ kind: 'known', request, outcome, error: outcome.errorMessage, reconciliation: null, acknowledged: false });
      this.operationError = outcome.errorMessage;
      if (outcome.plan && outcome.verified && outcome.errorMessage === '') {
        this.review = freeze({ ...copy(request.review), physical: copy(outcome.plan.proposed) }); this.actions = []; this.draftError = '';
      }
      return true;
    } catch (cause) {
      if (generation === this.generation) this.unknown(request, cause);
      return false;
    } finally {
      this.pending = null; this.dispatchSettled = true; this.activeSave = null; this.finish(generation);
      // A cancelled dispatch may settle after its UI generation has been retired.
      this.changed();
    }
  }
  correctReceipt() {
    this.idle();
    if (!this.outstanding() || !this.correctionReview || this.readError)
      fail('Correction requires a successful explicit owned Load after the retained receipt.');
    for (const receipt of this.receipts) if (!receipt.acknowledged) receipt.reconciliation = copy(this.correctionReview);
    this.review = this.correctionReview;
    this.actions = []; this.draftError = ''; this.operationError = '';
    this.correctionReview = null; this.changed();
  }
  acknowledge() {
    this.idle();
    if (this.readError || this.draftError) fail('Correct retained read/draft errors before acknowledgement.');
    if (this.correctionReview) fail('Explicit correction of loaded reconciliation is required before acknowledgement.');
    for (const r of this.receipts) if (!r.acknowledged &&
        (r.kind === 'unknown' || r.outcome?.verified !== true || r.outcome.errorMessage !== '') && !r.reconciliation)
      fail('Unknown/unverified/error outcome requires explicit Load and correction; committed evidence is retained.');
    for (const r of this.receipts) r.acknowledged = true;
    this.changed();
  }
  dispose() {
    this.idle();
    const close = this.closeState();
    if (close.blocked || close.unsaved || this.receipts.length) fail('Retained authority/errors/receipts cannot be discarded.');
    this.disposed = true; this.generation++; this.listeners.clear();
  }
}
