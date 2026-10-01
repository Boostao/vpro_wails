export interface CancellableRead<T> extends PromiseLike<T> {
  cancel(): void;
}

export class ReadRequests {
  private pending = new Set<CancellableRead<unknown>>();

  track<T>(request: CancellableRead<T>): Promise<T> {
    this.pending.add(request);
    return Promise.resolve(request).finally(() => this.pending.delete(request));
  }

  cancelAll(): void {
    const requests = [...this.pending];
    this.pending.clear();
    for (const request of requests) {
      cancelledReads.add(request);
      request.cancel();
    }
  }
}
import { Call, CancelledRejectionError } from '@wailsio/runtime';

const cancelledReads = new WeakSet<object>();

export function expectedReadCancellation(reason: unknown): boolean {
  return reason instanceof CancelledRejectionError && cancelledReads.has(reason.promise) &&
    reason.cause instanceof Call.RuntimeError && reason.cause.message === 'context canceled';
}

// beta.26 reports the backend acknowledgement even after its promise was cancelled.
if (typeof window !== 'undefined') {
  window.addEventListener('unhandledrejection', event => {
    if (expectedReadCancellation(event.reason)) event.preventDefault();
  });
}
