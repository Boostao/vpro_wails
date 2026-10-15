import { SummaryVegetationFieldListService } from '../bindings/github.com/boostao/vpro-wails';
import { fieldListJSON, fieldListOwner, validateFieldListReview, type FieldListOwner, type FieldListRead,
  type FieldListReview, type SummaryVegetationFieldListSession } from './summaryVegetationFieldList';
import { fieldListRegistry, type FieldListEnvironment } from './summaryVegetationFieldListSessions';

export type FieldListContext = Omit<FieldListOwner, 'sourcePath'>;
export interface FieldListDesktopPort {
  read(contextId: string): FieldListRead<unknown>;
  save(contextId: string, requestJSON: string): FieldListRead<unknown>;
  validationYield?(): PromiseLike<void>;
}
export interface FieldListDesktopState {
  context: FieldListContext;
  scope: FieldListOwner | null;
  session: SummaryVegetationFieldListSession | null;
  readBusy: boolean;
  readError: string;
  bootstrapPending: boolean;
  canDismissInitialReadError: boolean;
}
export interface FieldListDesktopActivity { busy: boolean; blocked: boolean }

function literalContext(value: FieldListContext): FieldListContext {
  const names = ['contextId', 'project', 'projectPath', 'su', 'suPath'];
  if (!value || typeof value !== 'object' || Reflect.ownKeys(value).length !== names.length ||
      Reflect.ownKeys(value).some(name => typeof name !== 'string' || !names.includes(name)) ||
      Object.values(value).some(item => typeof item !== 'string' || item.includes('\0')) ||
      !value.contextId || !value.project || !value.projectPath || !value.su ||
      value.su !== 'None' && !value.suPath)
    throw new Error('Complete literal five-field context required.');
  fieldListJSON(value);
  return Object.freeze({ ...value });
}

export function fieldListDesktop(port: FieldListDesktopPort,
  environment: FieldListEnvironment = import.meta.env, uuid?: () => string) {
  const contexts = new Map<string, {
    state: FieldListDesktopState; generation: number; pending: FieldListRead<unknown> | null;
    seed: FieldListReview | null; bootstrapDispatches: number; established: boolean;
  }>();
  const listeners = new Set<() => void>();
  const changed = () => { for (const listener of listeners) listener(); };
  const registry = fieldListRegistry(owner => ({
    read: () => {
      const entry = contexts.get(key(owner));
      if (entry?.seed) {
        const review = entry.seed; entry.seed = null;
        return Promise.resolve(review);
      }
      return port.read(owner.contextId);
    },
    save: json => port.save(owner.contextId, json),
    validationYield: port.validationYield,
  }), environment, uuid);
  registry.subscribe(changed);
  function key(context: FieldListContext) { return JSON.stringify(literalContext({
    contextId: context.contextId, project: context.project, projectPath: context.projectPath,
    su: context.su, suPath: context.suPath,
  })); }
  function entry(context: FieldListContext) {
    const id = JSON.stringify(literalContext(context)), existing = contexts.get(id);
    if (existing) return existing;
    if (!registry.enabled) throw new Error('Private Summary environmental field-list disabled.');
    if (contexts.size >= 16) throw new Error('Retained field-list bootstrap capacity reached; no state is evicted.');
    const created = { state: { context: literalContext(context), scope: null, session: null,
      readBusy: false, readError: '', bootstrapPending: false, canDismissInitialReadError: false } as FieldListDesktopState, generation: 0,
      pending: null as FieldListRead<unknown> | null, seed: null as FieldListReview | null,
      bootstrapDispatches: 0, established: false };
    contexts.set(id, created);
    return created;
  }
  function state(context: FieldListContext): FieldListDesktopState {
    const current = entry(context);
    return Object.freeze({ ...current.state, bootstrapPending: current.bootstrapDispatches !== 0,
      canDismissInitialReadError: dismissReason(current) === '' });
  }
  function dismissReason(current: ReturnType<typeof entry>): string {
    if (current.established || current.state.session || current.state.scope || current.seed)
      return 'Initial read acknowledgement cannot dismiss established field-list authority, drafts or receipts.';
    if (current.state.readBusy || current.bootstrapDispatches !== 0 || current.pending)
      return 'Wait for every initial field-list read dispatch and validation to settle before acknowledging its error.';
    if (!current.state.readError)
      return 'No failed initial field-list read diagnostic is available to acknowledge.';
    return '';
  }
  function dismissInitialReadError(context: FieldListContext) {
    const current = entry(context), reason = dismissReason(current);
    if (reason) throw new Error(reason);
    current.state.readError = '';
    changed();
  }
  function activity(): FieldListDesktopActivity {
    const retained = registry.activity();
    return { busy: retained.busy || [...contexts.values()].some(e => e.state.readBusy || e.bootstrapDispatches !== 0),
      blocked: retained.blocked || [...contexts.values()].some(e => !!e.state.readError) };
  }
  function cancel(context: FieldListContext) {
    const current = entry(context);
    current.generation++;
    current.seed = null;
    current.state.session?.cancel();
    if (current.state.readBusy) {
      current.state.readBusy = false;
      current.state.readError = 'Owned bootstrap Load cancelled; explicit Load required.';
      const pending = current.pending; current.pending = null;
      try { pending?.cancel?.(); } catch (cause) { current.state.readError += ` ${String(cause)}`; }
      changed();
    }
  }
  async function load(context: FieldListContext): Promise<boolean> {
    const current = entry(context);
    if (current.state.readBusy || current.bootstrapDispatches !== 0) return false;
    if (current.state.session) return current.state.session.load();
    const generation = ++current.generation;
    current.bootstrapDispatches++;
    current.state.readBusy = true; current.state.readError = ''; changed();
    try {
      if (generation !== current.generation) return false;
      const request = port.read(current.state.context.contextId); current.pending = request;
      const value = await request;
      if (generation !== current.generation) return false;
      if (!value || typeof value !== 'object' || !('sourcePath' in value))
        throw new Error('Owned field-list review has no sourcePath.');
      const scope = fieldListOwner({ ...current.state.context, sourcePath: value.sourcePath });
      const reviewed = validateFieldListReview(value, scope);
      await port.validationYield?.();
      if (generation !== current.generation) return false;
      current.seed = reviewed;
      current.established = true;
      const session = registry.session(scope);
      if (generation !== current.generation) return false;
      current.state.scope = scope; current.state.session = session;
      current.state.readBusy = false; current.pending = null;
      if (generation !== current.generation) return false;
      changed();
      if (generation !== current.generation) return false;
      const loaded = await session.load();
      if (generation !== current.generation) return false;
      return loaded;
    } catch (cause) {
      if (generation === current.generation)
        current.state.readError = `Owned field-list bootstrap failed; no default authority: ${String(cause)}`;
      return false;
    } finally {
      current.bootstrapDispatches--;
      if (generation === current.generation) {
        current.pending = null; current.seed = null; current.state.readBusy = false;
      }
      changed();
    }
  }
  return {
    enabled: registry.enabled, state, activity, load, cancel, dismissInitialReadError,
    contextAvailable(context: FieldListContext) {
      try {
        const id = JSON.stringify(literalContext(context));
        return registry.enabled && (contexts.has(id) || contexts.size < 16);
      } catch { return false; }
    },
    subscribe(listener: () => void) { listeners.add(listener); listener(); return () => { listeners.delete(listener); }; },
    detach(context: FieldListContext) { cancel(context); entry(context).state.session?.detach(); },
  };
}

export const summaryVegetationFieldListDesktop = fieldListDesktop({
  read: id => SummaryVegetationFieldListService.GetSummaryVegetationFieldList(id),
  save: (id, json) => SummaryVegetationFieldListService.SaveSummaryVegetationFieldList(id, json),
  validationYield: () => new Promise(resolve => setTimeout(resolve, 0)),
});
