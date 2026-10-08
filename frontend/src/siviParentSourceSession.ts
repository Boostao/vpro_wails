import type { SIVIParentJoinReview, SIVIProjectChoices } from '../bindings/github.com/boostao/vpro-wails';
import { exactSigned64 } from './projectMetadataRestore';
import { siviMetadataTableFromWire, type SIVIParentOwner } from './siviParentTransport';

const record = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);
function rowIDs(value: unknown): value is string[] {
  return Array.isArray(value) && Array.from(value).every(id => typeof id === 'string' && exactSigned64(id)) &&
    new Set(value).size === value.length;
}
function joinFromWire(wire: unknown, owner: SIVIParentOwner): SIVIParentJoinReview {
  if (!record(wire) || wire.ContextID !== owner.contextId || wire.Project !== owner.project || wire.Plot !== owner.plot ||
    wire.Scope !== 'DAO-General1033-ASCII-alphanumeric-TEXT7' || typeof wire.Diagnostic !== 'string' ||
    typeof wire.Verified !== 'boolean' || !rowIDs(wire.EnvRowIDs) || !rowIDs(wire.AdminRowIDs) ||
    wire.Verified && (wire.EnvRowIDs.length !== 1 || wire.AdminRowIDs.length !== 1)) {
    throw new Error('SIVI join review is incomplete or belongs to another owner.');
  }
  return { ContextID: owner.contextId, Project: owner.project, Plot: owner.plot, Scope: wire.Scope,
    Diagnostic: wire.Diagnostic, Verified: wire.Verified, EnvRowIDs: [...wire.EnvRowIDs], AdminRowIDs: [...wire.AdminRowIDs] };
}
export type SIVIProjectPhysicalChoices = Omit<SIVIProjectChoices, 'Choices'> & {
  Choices: ReturnType<typeof siviMetadataTableFromWire>;
};
export function siviProjectChoicesFromWire(wire: unknown, owner: SIVIParentOwner): SIVIProjectPhysicalChoices {
  if (!record(wire) || wire.ContextID !== owner.contextId || wire.Project !== owner.project ||
    (wire.SourceOption !== 1 && wire.SourceOption !== 2) || typeof wire.Table !== 'string' ||
    (wire.SourceOption === 1 ? wire.Source !== 'Env' || wire.Alias !== 'project' || wire.Table !== `${owner.project}_Metadata`
      : wire.Source !== 'Master' || wire.Alias !== 'VMetaData' || wire.Table.toLowerCase() !== 'projectmetadata')) {
    throw new Error('SIVI ProjectID choices are incomplete or belong to another source/owner.');
  }
  const choices = siviMetadataTableFromWire(wire.Choices);
  if (choices.columns.length !== 2 || choices.columns[0].name.toLowerCase() !== 'projectid' ||
    choices.columns[1].name.toLowerCase() !== 'projecttitle') throw new Error('SIVI ProjectID columns are incomplete.');
  return { ContextID: owner.contextId, Project: owner.project, SourceOption: wire.SourceOption,
    Source: wire.SourceOption === 1 ? 'Env' : 'Master', Alias: wire.SourceOption === 1 ? 'project' : 'VMetaData',
    Table: wire.Table, Choices: choices };
}

export interface SIVIParentSourceView {
  join: SIVIParentJoinReview | null;
  choices: SIVIProjectChoices | null;
  busy: boolean;
  saving: boolean;
  authorityUnknown: boolean;
  error: string | null;
}
export interface SIVIParentSourcePort {
  join(): PromiseLike<unknown>;
  choices(): PromiseLike<unknown>;
  setSource(expected: number, source: number): PromiseLike<unknown>;
  cancel(): void;
}
export class SIVIParentSourceSession {
  private state: SIVIParentSourceView = { join: null, choices: null, busy: false, saving: false, authorityUnknown: false, error: null };
  private generation = 0;
  private disposed = false;
  private readonly owner: SIVIParentOwner;
  constructor(owner: SIVIParentOwner, private readonly port: SIVIParentSourcePort, private readonly notify: () => void) {
    this.owner = structuredClone(owner);
  }
  view(): SIVIParentSourceView { return structuredClone(this.state); }
  private ready() {
    if (this.disposed) throw new Error('SIVI source ownership ended.');
    if (this.state.busy) throw new Error('Wait for the current SIVI source operation.');
  }
  async load(): Promise<boolean> {
    this.ready();
    const generation = ++this.generation;
    const authorityUnknown = this.state.authorityUnknown;
    this.state = { join: null, choices: null, busy: true, saving: false, authorityUnknown, error: null };
    this.notify();
    try {
      const [join, choices] = await Promise.all([this.port.join(), this.port.choices()]);
      if (this.disposed || generation !== this.generation) return false;
      this.state.join = joinFromWire(join, this.owner);
      this.state.choices = siviProjectChoicesFromWire(choices, this.owner);
      this.state.authorityUnknown = false;
      return true;
    } catch (cause) {
      if (this.disposed || generation !== this.generation) return false;
      this.port.cancel();
      this.state.error = `SIVI source read failed: ${String(cause)}`;
      return false;
    } finally {
      if (!this.disposed && generation === this.generation) { this.state.busy = false; this.notify(); }
    }
  }
  async setSource(source: number): Promise<boolean> {
    this.ready();
    if (source !== 1 && source !== 2) throw new Error('ProjectID source must be Env or Master.');
    if (!this.state.choices) throw new Error('Reload owned ProjectID choices before switching source.');
    const expected = this.state.choices.SourceOption;
    this.state.busy = this.state.saving = true;
    this.state.error = null;
    this.notify();
    try {
      const choices = siviProjectChoicesFromWire(await this.port.setSource(expected, source), this.owner);
      if (choices.SourceOption !== source) throw new Error('Committed source differs from the requested option; reload.');
      this.state.choices = choices;
      return true;
    } catch (cause) {
      this.state.choices = null;
      this.state.authorityUnknown = true;
      this.state.error = `SIVI source preference failed: ${String(cause)} Reload to observe the current setting.`;
      return false;
    } finally {
      this.state.busy = this.state.saving = false;
      this.notify();
    }
  }
  cancel() {
    if (this.state.saving) throw new Error('Wait for the source preference commit; it cannot be cancelled or discarded.');
    if (!this.state.busy) return;
    this.generation++;
    this.port.cancel();
    this.state = { join: null, choices: null, busy: false, saving: false,
      authorityUnknown: this.state.authorityUnknown, error: 'SIVI source read cancelled. Reload explicitly.' };
    this.notify();
  }
  dispose() {
    if (this.state.saving) throw new Error('Wait for the source preference commit before ending ownership.');
    if (this.state.authorityUnknown) throw new Error('Reload owned ProjectID choices to observe the source preference outcome before ending ownership.');
    this.disposed = true;
    this.generation++;
    this.port.cancel();
  }
}
