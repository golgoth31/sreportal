export type ComponentStatus =
  | "operational"
  | "degraded"
  | "partial_outage"
  | "major_outage"
  | "unknown"
  | "maintenance";

export type MaintenancePhase = "upcoming" | "in_progress" | "completed";

export type IncidentPhase =
  | "investigating"
  | "identified"
  | "monitoring"
  | "resolved";

export type IncidentSeverity = "critical" | "major" | "minor";

export interface DailyStatus {
  date: string;
  worstStatus: ComponentStatus;
}

export interface PlatformComponent {
  name: string;
  displayName: string;
  description: string;
  group: string;
  link: string;
  portalRef: string;
  declaredStatus: ComponentStatus;
  computedStatus: ComponentStatus;
  activeIncidents: number;
  lastStatusChange: string;
  dailyWorstStatus: DailyStatus[];
}

export interface Maintenance {
  name: string;
  title: string;
  description: string;
  portalRef: string;
  components: string[];
  scheduledStart: string;
  scheduledEnd: string;
  affectedStatus: string;
  phase: MaintenancePhase;
}

export interface IncidentUpdate {
  timestamp: string;
  phase: IncidentPhase;
  message: string;
}

export interface Incident {
  name: string;
  title: string;
  portalRef: string;
  components: string[];
  severity: IncidentSeverity;
  currentPhase: IncidentPhase;
  updates: IncidentUpdate[];
  startedAt: string;
  resolvedAt: string;
  durationMinutes: number;
}
