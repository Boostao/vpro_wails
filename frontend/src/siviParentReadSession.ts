import { siviParentOriginalFromWire, type SIVIParentOwner } from './siviParentTransport';
import type { SIVIParentOriginal } from './siviParentEditor';

export interface SIVIParentReadView {
  original: SIVIParentOriginal | null;
  busy: boolean;
  error: string | null;
}
export class SIVIParentReadSession {
  private state: SIVIParentReadView = { original: null, busy: false, error: null };
  private generation = 0;
  private disposed = false;
  private readonly owner: SIVIParentOwner;

  constructor(owner: SIVIParentOwner, private readonly read: () => PromiseLike<unknown>,
    private readonly cancelRead: () => void, private readonly notify: () => void) {
    this.owner = structuredClone(owner);
  }

  view(): SIVIParentReadView { return structuredClone(this.state); }

  async load(): Promise<boolean> {
    if (this.disposed) throw new Error('SIVI parent reader ownership ended.');
    if (this.state.busy) throw new Error('Wait for or cancel the current SIVI parent read.');
    const generation = ++this.generation;
    this.state = { original: null, busy: true, error: null };
    this.notify();
    try {
      const wire = await this.read();
      if (this.disposed || generation !== this.generation) return false;
      this.state.original = siviParentOriginalFromWire(wire, this.owner);
      return true;
    } catch (cause) {
      if (this.disposed || generation !== this.generation) return false;
      this.state.error = `SIVI parent read failed: ${String(cause)}`;
      return false;
    } finally {
      if (!this.disposed && generation === this.generation) {
        this.state.busy = false;
        this.notify();
      }
    }
  }

  cancel() {
    if (this.disposed) throw new Error('SIVI parent reader ownership ended.');
    if (!this.state.busy) return;
    this.generation++;
    this.cancelRead();
    this.state = { original: null, busy: false, error: 'SIVI parent read cancelled. Reload explicitly.' };
    this.notify();
  }

  dispose() {
    this.disposed = true;
    this.generation++;
    this.cancelRead();
  }
}
