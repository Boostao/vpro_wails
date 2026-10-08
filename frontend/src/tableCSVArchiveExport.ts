import type { TableCSVArchiveReview, TableCSVArchiveOutcome } from '../bindings/github.com/boostao/vpro-wails';
import { coreTableSuffixes, validateProjectTableCSVReview, type ValidatedProjectTableCSVReview } from './projectTableCSV';
import { wellFormedUTF16 } from './qualityEditor';
import { Dialogs } from '@wailsio/runtime';

export function chooseTableCSVArchiveFile(): Promise<string> {
  return Dialogs.SaveFile({ Title: 'Select a new migration archive destination',
    Filename: 'migration.zip', Filters: [{ DisplayName: 'ZIP migration archive', Pattern: '*.zip' }] });
}

export interface TableCSVArchiveOwner { contextId: string; project: string; projectPath: string }
export type ValidatedTableCSVArchiveReview = Omit<TableCSVArchiveReview, 'review'> & { review: ValidatedProjectTableCSVReview };
const hash = (value: unknown): value is string => typeof value === 'string' && /^[0-9a-f]{64}$/.test(value);
const text = (value: unknown): value is string => typeof value === 'string' && wellFormedUTF16(value) && !value.includes('\0');
export const validTableCSVArchiveDestination = (value: unknown): value is string => text(value) && value.length > 0;
const sameOwner = (left: TableCSVArchiveOwner, right: TableCSVArchiveOwner) =>
  left.contextId === right.contextId && left.project === right.project && left.projectPath === right.projectPath;
const ownedTable = (owner: TableCSVArchiveOwner, table: string) =>
  coreTableSuffixes.some(suffix => table === `${owner.project}_${suffix}`);

export async function validateTableCSVArchiveReview(value: TableCSVArchiveReview | null,
  owner: TableCSVArchiveOwner, table: string, checksum?: (csv: string) => Promise<string>): Promise<ValidatedTableCSVArchiveReview> {
  const detached = value ? structuredClone(value) : null;
  const scope = structuredClone(owner);
  if (!detached || !hash(detached.approvalHash) || !hash(detached.archiveSHA256) ||
      detached.format !== 'vpro-owned-table-csv' || detached.version !== 1 ||
      !Number.isSafeInteger(detached.byteCount) || detached.byteCount <= 0) {
    throw new Error('Migration archive review has an incomplete format, version, source approval, archive hash or byte count.');
  }
  const review = await validateProjectTableCSVReview(detached.review, scope.contextId, scope.project, scope.projectPath, table, checksum);
  // Only the CSV checksum is independently checked here; approval and ZIP hash are supplied by the backend.
  return { ...detached, review };
}

export function validateTableCSVArchiveOutcome(value: TableCSVArchiveOutcome | null,
  destination: string, archiveSHA256: string): TableCSVArchiveOutcome {
  const detached = value ? structuredClone(value) : null;
  if (!validTableCSVArchiveDestination(destination) || !hash(archiveSHA256) || !detached ||
      detached.requestedDestination !== destination || !text(detached.path) || !text(detached.errorMessage) ||
      !['published', 'published-with-errors', 'not-published'].includes(detached.status) ||
      (detached.sha256 !== '' && !hash(detached.sha256)) ||
      (detached.status !== 'not-published' && (!detached.path || detached.sha256 !== archiveSHA256)) ||
      (detached.status === 'not-published' && (detached.path !== '' || detached.sha256 !== '')) ||
      (detached.status === 'published' ? detached.errorMessage !== '' : detached.errorMessage === '')) {
    throw new Error('Migration archive publication receipt contradicts the literal destination, expected bytes or outcome.');
  }
  return detached;
}

export interface TableCSVArchiveReceipt {
  table: string;
  requestedDestination: string;
  archiveSHA256: string;
  approvalHash: string;
  phase: 'pending' | 'known' | 'unknown';
  outcome: TableCSVArchiveOutcome | null;
  error: string;
  acknowledged: boolean;
}
export interface TableCSVArchivePublicationView {
  busy: boolean;
  blocked: boolean;
  storageError: string;
  receipts: TableCSVArchiveReceipt[];
}
export interface ArchiveReceiptStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}
type PublicationPort = (request: { table: string; approvalHash: string; destination: string }) =>
  PromiseLike<TableCSVArchiveOutcome | null>;

export class TableCSVArchivePublicationSession {
  private readonly owner: TableCSVArchiveOwner;
  private readonly key: string;
  private readonly storage: ArchiveReceiptStorage;
  private state: TableCSVArchivePublicationView = { busy: false, blocked: false, storageError: '', receipts: [] };
  private listeners = new Set<() => void>();

  constructor(owner: TableCSVArchiveOwner, storage: ArchiveReceiptStorage = {
    getItem: key => localStorage.getItem(key), setItem: (key, value) => localStorage.setItem(key, value),
  }) {
    this.owner = structuredClone(owner); this.storage = storage;
    this.key = `vpro-owned-table-csv-receipts-v1:${owner.contextId}`;
    try {
      const raw = storage.getItem(this.key);
      if (raw !== null) {
        const saved = JSON.parse(raw);
        if (saved.version !== 1 || !saved.owner || !sameOwner(saved.owner, this.owner) || !Array.isArray(saved.receipts)) {
          throw new Error('Stored archive receipt ownership or format differs.');
        }
        for (const receipt of saved.receipts) {
          if (!ownedTable(this.owner, receipt.table) || !validTableCSVArchiveDestination(receipt.requestedDestination) ||
              !hash(receipt.archiveSHA256) || !hash(receipt.approvalHash) || !text(receipt.error) ||
              typeof receipt.acknowledged !== 'boolean' || !['pending', 'known', 'unknown'].includes(receipt.phase) ||
              (receipt.phase !== 'known' && receipt.outcome !== null) ||
              (receipt.phase === 'pending' && receipt.acknowledged)) throw new Error('Stored archive attempt is incomplete.');
          if (receipt.phase === 'known') {
            validateTableCSVArchiveOutcome(receipt.outcome, receipt.requestedDestination, receipt.archiveSHA256);
          }
        }
        this.state.receipts = saved.receipts;
        for (const receipt of this.state.receipts) {
          if (receipt.phase === 'pending') {
            receipt.phase = 'unknown';
            receipt.error = 'Archive outcome unknown after reload during publication. Do not replay; inspect the destination.';
          }
        }
        this.state.blocked = this.state.receipts.some(receipt => !receipt.acknowledged);
        this.persist();
      }
    } catch (cause) {
      this.state.storageError = `Archive receipt history unavailable; export is blocked to preserve evidence: ${String(cause)}`;
      this.state.blocked = true;
    }
  }
  view(): TableCSVArchivePublicationView { return structuredClone(this.state); }
  subscribe(listener: () => void): () => void {
    this.listeners.add(listener); listener();
    return () => this.listeners.delete(listener);
  }
  private notify() { for (const listener of this.listeners) listener(); }
  private persist() {
    this.storage.setItem(this.key, JSON.stringify({ version: 1, owner: this.owner, receipts: this.state.receipts }));
  }
  private available() {
    if (this.state.busy || this.state.blocked) throw new Error('Wait for and acknowledge the prior archive attempt before proceeding.');
  }
  async chooseDestination(current: string, chooser: () => PromiseLike<string>): Promise<string> {
    this.available(); this.state.busy = true; this.notify();
    try {
      const selected = await chooser();
      if (selected === '') return current;
      if (!validTableCSVArchiveDestination(selected)) throw new Error('Picker returned a malformed literal destination.');
      return selected;
    } finally { this.state.busy = false; this.notify(); }
  }
  async publish(value: TableCSVArchiveReview, table: string, destination: string, port: PublicationPort): Promise<void> {
    this.available();
    if (!validTableCSVArchiveDestination(destination)) throw new Error('An explicit nonempty literal destination is required.');
    const detached = structuredClone(value);
    this.state.busy = true; this.notify();
    let review: ValidatedTableCSVArchiveReview;
    try {
      review = await validateTableCSVArchiveReview(detached, this.owner, table);
      const receipt: TableCSVArchiveReceipt = { table, requestedDestination: destination, archiveSHA256: review.archiveSHA256,
        approvalHash: review.approvalHash, phase: 'pending', outcome: null, error: '', acknowledged: false };
      this.state.receipts.push(receipt);
      try { this.persist(); } catch (cause) {
        this.state.receipts.pop();
        this.state.storageError = `Cannot retain archive attempt history; no publication invoked: ${String(cause)}`;
        this.state.blocked = true;
        throw cause;
      }
      this.state.blocked = true; this.notify();
      try {
        receipt.outcome = validateTableCSVArchiveOutcome(await port({ table, approvalHash: review.approvalHash, destination }),
          destination, review.archiveSHA256);
        receipt.phase = 'known'; receipt.error = receipt.outcome.errorMessage;
      } catch (cause) {
        receipt.phase = 'unknown';
        receipt.error = `Archive outcome unknown. Do not replay this destination; inspect it before proceeding: ${String(cause)}`;
      }
      try { this.persist(); } catch (cause) {
        this.state.storageError = `Archive receipt persistence failed; retain this visible evidence, do not replay: ${String(cause)}`;
      }
    } finally { this.state.busy = false; this.notify(); }
  }
  acknowledge() {
    if (this.state.busy || this.state.storageError) throw new Error('Wait for publication/picker and resolve receipt storage; never cancel or replay publication.');
    for (const receipt of this.state.receipts) receipt.acknowledged = true;
    try { this.persist(); this.state.blocked = false; } catch (cause) {
      this.state.storageError = `Cannot persist archive acknowledgement: ${String(cause)}`;
    }
    this.notify();
  }
}

const sessions = new Map<string, { owner: TableCSVArchiveOwner; session: TableCSVArchivePublicationSession }>();
export function tableCSVArchivePublicationSession(owner: TableCSVArchiveOwner): TableCSVArchivePublicationSession {
  const existing = sessions.get(owner.contextId);
  if (existing) {
    if (!sameOwner(existing.owner, owner)) throw new Error('Archive receipt belongs to different owned project paths.');
    return existing.session;
  }
  const session = new TableCSVArchivePublicationSession(owner);
  sessions.set(owner.contextId, { owner: structuredClone(owner), session });
  return session;
}
