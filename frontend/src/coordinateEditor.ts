import type { CoordinateParts } from '../bindings/github.com/boostao/vpro-wails';

import type { CoordinateDisplayMode } from './paperLayout';
export type { CoordinateDisplayMode } from './paperLayout';
export type CoordinateAxis = 'latitude' | 'longitude';
export type CoordinatePart = 'degrees' | 'minutes' | 'seconds';
export type CoordinateBuffers = Record<CoordinatePart, string> & { negative: boolean };
export interface CoordinateAxisState {
  value: number | null;
  buffers: CoordinateBuffers;
  error: string | null;
  ready: boolean;
  touched: boolean;
  pending: boolean;
}
export interface CoordinateState {
  mode: CoordinateDisplayMode;
  ready: boolean;
  busy: boolean;
  axes: Record<CoordinateAxis, CoordinateAxisState>;
}
export interface CoordinateAPI {
  getMode(): Promise<string>;
  setMode(mode: CoordinateDisplayMode): Promise<void>;
  decompose(value: number | null, mode: CoordinateDisplayMode): Promise<CoordinateParts>;
  convert(axis: CoordinateAxis, mode: CoordinateDisplayMode, parts: CoordinateParts): Promise<number | null>;
}
export interface CoordinateCallbacks {
  state(state: CoordinateState): void;
  busy(busy: boolean): void;
  dirty(): void;
  value(axis: CoordinateAxis, value: number | null): void;
  validation(axis: CoordinateAxis, message: string | null): void;
  error(message: string): void;
}
const axes: CoordinateAxis[] = ['latitude', 'longitude'];
const empty = (): CoordinateBuffers => ({ degrees: '', minutes: '', seconds: '', negative: false });
const axisState = (): CoordinateAxisState => ({ value: null, buffers: empty(), error: null, ready: false, touched: false, pending: false });
const sessions = new WeakMap<object, CoordinateState>();
const pendingSessions = new WeakMap<object, Set<object>>();
export function coordinateSession(identity: object) { return sessions.get(identity); }
export function rememberCoordinateSession(identity: object, state: CoordinateState) { sessions.set(identity, state); }
export function coordinateBusy(identity: object, owner: object, busy: boolean): boolean {
  let owners = pendingSessions.get(identity);
  if (!owners) { owners = new Set(); pendingSessions.set(identity, owners); }
  if (busy) owners.add(owner); else owners.delete(owner);
  return owners.size > 0;
}

export const coordinateControls: Record<string, { axis: CoordinateAxis; part: CoordinatePart; mode: CoordinateDisplayMode }> = {
  Latitude: { axis: 'latitude', part: 'degrees', mode: 'dd' },
  Longitude: { axis: 'longitude', part: 'degrees', mode: 'dd' },
  LatD2: { axis: 'latitude', part: 'degrees', mode: 'dm' },
  LatMD: { axis: 'latitude', part: 'minutes', mode: 'dm' },
  LonD2: { axis: 'longitude', part: 'degrees', mode: 'dm' },
  LonMD: { axis: 'longitude', part: 'minutes', mode: 'dm' },
  LatD: { axis: 'latitude', part: 'degrees', mode: 'dms' },
  LatM: { axis: 'latitude', part: 'minutes', mode: 'dms' },
  LatS: { axis: 'latitude', part: 'seconds', mode: 'dms' },
  LonD: { axis: 'longitude', part: 'degrees', mode: 'dms' },
  LonM: { axis: 'longitude', part: 'minutes', mode: 'dms' },
  LonS: { axis: 'longitude', part: 'seconds', mode: 'dms' }
};
export const coordinateOptions: Record<string, CoordinateDisplayMode> = { Check369: 'dd', Check371: 'dm', Check373: 'dms' };
export function isCoordinateControl(name: string | undefined) {
  return name !== undefined && (Object.hasOwn(coordinateControls, name) || Object.hasOwn(coordinateOptions, name) || name === 'optCoordMethod2');
}
export function isCoordinateMode(mode: string): mode is CoordinateDisplayMode { return ['dd', 'dm', 'dms'].includes(mode); }

function coordinatePartValue(axis: CoordinateAxis, part: CoordinatePart, text: string): number | null {
  if (text === '') return null;
  if (!/^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?$/.test(text) || !Number.isFinite(Number(text))) {
    throw new Error(`Enter a finite ${axis} ${part}.`);
  }
  return Number(text);
}

export function coordinateDecimalValue(axis: CoordinateAxis, raw: string, unchanged: number | null):
  { raw: string; value: number | null; error: string | null } {
  try {
    const value = coordinatePartValue(axis, 'degrees', raw);
    if (value !== unchanged) {
      coordinateParts(axis, 'dd', { degrees: raw, minutes: '', seconds: '', negative: false });
    }
    return { raw, value, error: null };
  } catch (cause) {
    return { raw, value: null, error: cause instanceof Error ? cause.message : String(cause) };
  }
}

export function coordinateParts(axis: CoordinateAxis, mode: CoordinateDisplayMode, buffers: CoordinateBuffers): CoordinateParts {
  const parts: CoordinateParts = { degrees: null, minutes: null, seconds: null, negative: mode !== 'dd' && buffers.negative };
  const required: CoordinatePart[] = mode === 'dd' ? ['degrees'] : mode === 'dm' ? ['degrees', 'minutes'] : ['degrees', 'minutes', 'seconds'];
  for (const part of required) {
    parts[part] = coordinatePartValue(axis, part, buffers[part]);
  }
  if (required.every(part => parts[part] === null)) return parts;
  if (required.some(part => parts[part] === null)) throw new Error(`Complete all ${axis} ${mode.toUpperCase()} parts, or clear all parts.`);
  const degrees = parts.degrees!;
  if (mode !== 'dd' && (!Number.isInteger(degrees) || degrees < 0 || buffers.degrees.startsWith('-'))) throw new Error(`${axis} degrees must be an unsigned integer; use the sign selector.`);
  if (mode !== 'dd' && (parts.minutes! < 0 || parts.minutes! >= 60 || (mode === 'dms' && !Number.isInteger(parts.minutes)))) throw new Error(`${axis} minutes must be ${mode === 'dms' ? 'an integer ' : ''}from 0 to less than 60.`);
  if (mode === 'dms' && (parts.seconds! < 0 || parts.seconds! >= 60)) throw new Error(`${axis} seconds must be from 0 to less than 60.`);
  const magnitude = mode === 'dd' ? Math.abs(degrees) : degrees + parts.minutes! / 60 + (parts.seconds ?? 0) / 3600;
  if (magnitude > (axis === 'latitude' ? 90 : 180)) throw new Error(`${axis} must be within ${axis === 'latitude' ? '±90' : '±180'} degrees.`);
  return parts;
}
function buffersFrom(parts: CoordinateParts): CoordinateBuffers {
  for (const part of ['degrees', 'minutes', 'seconds'] as const) {
    if (parts[part] !== null && (typeof parts[part] !== 'number' || !Number.isFinite(parts[part]))) throw new Error('Coordinate service returned invalid display parts.');
  }
  if (typeof parts.negative !== 'boolean') throw new Error('Coordinate service returned an invalid sign.');
  const text = (value: number | null) => value === null ? '' : Object.is(value, -0) ? '-0' : String(value);
  return { degrees: text(parts.degrees), minutes: text(parts.minutes), seconds: text(parts.seconds), negative: parts.negative };
}

export class CoordinateEditor {
  private data: CoordinateState;
  private revisions = { latitude: 0, longitude: 0 };
  private epoch = 0;
  private pending = 0;
  private alive = true;
  private preferencePending = false;
  constructor(private api: CoordinateAPI, private callbacks: CoordinateCallbacks, session?: CoordinateState) {
    this.data = session ? structuredClone(session) : { mode: 'dd', ready: false, busy: false, axes: { latitude: axisState(), longitude: axisState() } };
    this.data.ready = false;
    this.data.busy = false;
  }
  snapshot(): CoordinateState { return structuredClone(this.data); }
  private publish() { if (this.alive) this.callbacks.state(this.snapshot()); }
  private begin() {
    if (++this.pending === 1) { this.data.busy = true; this.callbacks.busy(true); }
    this.publish();
  }
  private end() {
    if (--this.pending === 0) { this.data.busy = false; this.callbacks.busy(false); }
    this.publish();
  }
  private valid(axis: CoordinateAxis, revision: number, epoch: number) { return this.alive && this.revisions[axis] === revision && this.epoch === epoch; }
  private fail(axis: CoordinateAxis, cause: unknown) {
    const message = String(cause instanceof Error ? cause.message : cause);
    this.data.axes[axis].error = message;
    this.callbacks.validation(axis, message);
    this.callbacks.error(message);
    this.publish();
  }
  syncDraft(latitude: number | null, longitude: number | null) {
    for (const axis of axes) {
      const value = axis === 'latitude' ? latitude : longitude;
      if (Object.is(value, this.data.axes[axis].value)) continue;
      const hadValidation = this.data.axes[axis].error !== null || this.data.axes[axis].pending;
      this.revisions[axis]++;
      this.data.axes[axis] = { ...axisState(), value };
      if (hadValidation) this.callbacks.validation(axis, null);
      if (this.data.ready && !this.preferencePending) void this.decompose(axis);
    }
  }
  private async decompose(axis: CoordinateAxis) {
    const revision = ++this.revisions[axis], epoch = this.epoch, state = this.data.axes[axis];
    state.ready = false;
    this.begin();
    try {
      const parts = await this.api.decompose(state.value, this.data.mode);
      if (!this.valid(axis, revision, epoch)) return;
      const hadError = state.error !== null;
      state.buffers = buffersFrom(parts);
      state.ready = true;
      state.error = null;
      state.pending = false;
      if (hadError) this.callbacks.validation(axis, null);
      this.publish();
    } catch (cause) { if (this.valid(axis, revision, epoch)) this.fail(axis, cause); }
    finally { this.end(); }
  }
  async initialize() {
    if (!this.alive || this.data.busy) return;
    const epoch = ++this.epoch;
    this.preferencePending = true;
    this.begin();
    try {
      const mode = await this.api.getMode();
      if (!this.alive || epoch !== this.epoch) return;
      if (!isCoordinateMode(mode)) throw new Error('Coordinate service returned an unsupported display mode.');
      const invalid = axes.some(axis => this.data.axes[axis].touched && (this.data.axes[axis].error || this.data.axes[axis].pending));
      if (mode !== this.data.mode && invalid) this.callbacks.error('Coordinate preference changed while input is invalid; finish the input or Undo before changing display mode.');
      else if (mode !== this.data.mode) {
        this.data.mode = mode;
        for (const axis of axes) this.data.axes[axis] = { ...axisState(), value: this.data.axes[axis].value };
      }
      this.data.ready = true;
      for (const axis of axes) {
        const state = this.data.axes[axis];
        if (state.touched) {
          state.ready = true;
          if (state.pending) void this.convert(axis);
          else if (state.error) this.callbacks.validation(axis, state.error);
        } else await this.decompose(axis);
      }
    } catch (cause) {
      if (this.alive && epoch === this.epoch) { this.data.ready = false; this.callbacks.error(`Coordinate preference could not be loaded: ${String(cause)}`); }
    } finally { if (epoch === this.epoch) this.preferencePending = false; this.end(); }
  }
  async changeMode(mode: CoordinateDisplayMode) {
    if (!this.alive || !this.data.ready || this.data.busy || axes.some(axis => this.data.axes[axis].error)) {
      if (this.alive) this.callbacks.error('Finish valid coordinate input or Undo before changing display mode.');
      return;
    }
    if (mode === this.data.mode) return;
    const epoch = ++this.epoch;
    this.preferencePending = true;
    this.begin();
    try {
      await this.api.setMode(mode);
      if (!this.alive || epoch !== this.epoch) return;
      this.data.mode = mode;
      for (const axis of axes) this.data.axes[axis] = { ...axisState(), value: this.data.axes[axis].value };
      await Promise.all(axes.map(axis => this.decompose(axis)));
    } catch (cause) { if (this.alive && epoch === this.epoch) this.callbacks.error(`Coordinate preference could not be saved: ${String(cause)}`); }
    finally { if (epoch === this.epoch) this.preferencePending = false; this.end(); }
  }
  edit(axis: CoordinateAxis, part: CoordinatePart | 'negative', value: string | boolean) {
    if (!this.alive || !this.data.ready || !this.data.axes[axis].ready || this.preferencePending) return;
    const state = this.data.axes[axis];
    if (part === 'negative') state.buffers.negative = value === true;
    else state.buffers[part] = String(value);
    state.touched = true;
    this.callbacks.dirty();
    void this.convert(axis);
  }
  private async convert(axis: CoordinateAxis) {
    const revision = ++this.revisions[axis], epoch = this.epoch, state = this.data.axes[axis];
    let parts: CoordinateParts;
    try { parts = coordinateParts(axis, this.data.mode, state.buffers); }
    catch (cause) { state.pending = false; this.fail(axis, cause); return; }
    state.error = null;
    state.pending = true;
    this.callbacks.validation(axis, `Validating ${axis} coordinate…`);
    this.begin();
    try {
      const value = await this.api.convert(axis, this.data.mode, parts);
      if (!this.valid(axis, revision, epoch)) return;
      if (value !== null && (typeof value !== 'number' || !Number.isFinite(value) || Math.abs(value) > (axis === 'latitude' ? 90 : 180))) throw new Error('Coordinate service returned an invalid coordinate.');
      state.value = value;
      state.pending = false;
      this.callbacks.value(axis, value);
      this.callbacks.validation(axis, null);
      this.publish();
    } catch (cause) { if (this.valid(axis, revision, epoch)) { state.pending = false; this.fail(axis, cause); } }
    finally { this.end(); }
  }
  dispose() {
    this.alive = false;
    this.epoch++;
    if (this.pending === 0) this.callbacks.busy(false);
  }
}
