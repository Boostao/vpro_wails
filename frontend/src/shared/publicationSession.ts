export interface PublicationView<Outcome> {
  busy: boolean;
  blocked: boolean;
  outcome: Outcome | null;
  error: string;
  requestedDestination: string;
}

export class PublicationSession<Outcome extends { errorMessage: string }> {
  private state: PublicationView<Outcome> = { busy: false, blocked: false, outcome: null, error: '', requestedDestination: '' };
  private listeners = new Set<() => void>();
  constructor(private readonly label: string, private readonly acknowledgementLabel = label) {}
  view(): PublicationView<Outcome> { return structuredClone(this.state); }
  subscribe(listener: () => void): () => void {
    this.listeners.add(listener);
    listener();
    return () => this.listeners.delete(listener);
  }
  private notify() { for (const listener of this.listeners) listener(); }
  async publish<Request>(request: Request, destination: string,
    port: (request: Request) => PromiseLike<Outcome | null>,
    validate: (value: Outcome | null) => Outcome): Promise<void> {
    if (this.state.busy || this.state.blocked) throw new Error(`Acknowledge the prior ${this.label} outcome before a new publication.`);
    const requested = structuredClone(request);
    this.state = { busy: true, blocked: true, outcome: null, error: '', requestedDestination: destination };
    this.notify();
    try {
      this.state.outcome = validate(await port(requested));
      this.state.error = this.state.outcome.errorMessage;
    } catch (cause) {
      this.state.error = `${this.label} outcome unknown. Do not repeat this destination; retain and inspect it before proceeding: ${String(cause)}`;
    } finally {
      this.state.busy = false;
      this.notify();
    }
  }
  acknowledge() {
    if (this.state.busy) throw new Error(`Wait for ${this.acknowledgementLabel} publication to return; publication cannot be cancelled or replayed.`);
    this.state.blocked = false;
    this.notify();
  }
}
