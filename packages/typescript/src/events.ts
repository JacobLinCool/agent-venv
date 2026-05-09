export type EventKind =
  | "env.created"
  | "env.attached"
  | "profile.materialized"
  | "credentials.copied"
  | "credentials.refreshed"
  | "env.destroyed"
  | "registry.read"
  | "registry.written"
  | "error";

export interface Event {
  tsMs: number;
  kind: EventKind;
  data: Record<string, unknown>;
}

export type EventSink = (event: Event) => void;

export class EventLog {
  private origin = process.hrtime.bigint();
  private events: Event[] = [];
  constructor(private readonly sink?: EventSink) {}

  emit(kind: EventKind, data: Record<string, unknown> = {}): Event {
    const elapsedNs = process.hrtime.bigint() - this.origin;
    const tsMs = Number(elapsedNs / 1_000_000n);
    const event: Event = { tsMs, kind, data };
    this.events.push(event);
    if (this.sink) {
      try {
        this.sink(event);
      } catch {
        // Subscriber errors must not break the lifecycle.
      }
    }
    return event;
  }

  all(): Event[] {
    return [...this.events];
  }
}

export function eventToWire(event: Event): Record<string, unknown> {
  return { ts_ms: event.tsMs, kind: event.kind, ...event.data };
}
