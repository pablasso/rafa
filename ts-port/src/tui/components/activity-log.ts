/**
 * Tool use timeline component
 */

export interface ActivityEvent {
  icon: string;
  label: string;
  detail?: string;
  status: "running" | "done" | "error";
  duration?: number;
}

export class ActivityLogComponent {
  private events: ActivityEvent[] = [];

  addEvent(event: ActivityEvent): void {
    this.events.push(event);
  }

  clear(): void {
    this.events = [];
  }

  getEvents(): ActivityEvent[] {
    return this.events;
  }

  render(_width: number): string[] {
    return ["Activity Log - placeholder"];
  }
}
