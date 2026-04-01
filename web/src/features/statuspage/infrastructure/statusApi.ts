import { create } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import { createGrpcWebTransport } from "@connectrpc/connect-web";

import {
  StatusService,
  ListComponentsRequestSchema,
  ListMaintenancesRequestSchema,
  ListIncidentsRequestSchema,
  type ComponentResource as ProtoComponent,
  type MaintenanceResource as ProtoMaintenance,
  type IncidentResource as ProtoIncident,
  type IncidentUpdate as ProtoIncidentUpdate,
  ComponentStatus as ProtoComponentStatus,
  MaintenancePhase as ProtoMaintenancePhase,
  IncidentPhase as ProtoIncidentPhase,
  IncidentSeverity as ProtoIncidentSeverity,
} from "@/gen/sreportal/v1/status_pb";
import type {
  PlatformComponent,
  DailyStatus,
  Maintenance,
  Incident,
  IncidentUpdate,
  ComponentStatus,
  MaintenancePhase,
  IncidentPhase,
  IncidentSeverity,
} from "../domain/types";

const transport = createGrpcWebTransport({ baseUrl: window.location.origin });
const client = createClient(StatusService, transport);

function timestampToIso(
  ts: { seconds?: bigint; nanos?: number } | undefined
): string {
  if (ts == null || ts.seconds == null) return "";
  const ms = Number(ts.seconds) * 1000 + (ts.nanos ?? 0) / 1e6;
  return new Date(ms).toISOString();
}

function toComponentStatus(s: ProtoComponentStatus): ComponentStatus {
  switch (s) {
    case ProtoComponentStatus.OPERATIONAL:
      return "operational";
    case ProtoComponentStatus.DEGRADED:
      return "degraded";
    case ProtoComponentStatus.PARTIAL_OUTAGE:
      return "partial_outage";
    case ProtoComponentStatus.MAJOR_OUTAGE:
      return "major_outage";
    case ProtoComponentStatus.MAINTENANCE:
      return "maintenance";
    default:
      return "unknown";
  }
}

function toMaintenancePhase(p: ProtoMaintenancePhase): MaintenancePhase {
  switch (p) {
    case ProtoMaintenancePhase.UPCOMING:
      return "upcoming";
    case ProtoMaintenancePhase.IN_PROGRESS:
      return "in_progress";
    case ProtoMaintenancePhase.COMPLETED:
      return "completed";
    default:
      return "upcoming";
  }
}

function toIncidentPhase(p: ProtoIncidentPhase): IncidentPhase {
  switch (p) {
    case ProtoIncidentPhase.INVESTIGATING:
      return "investigating";
    case ProtoIncidentPhase.IDENTIFIED:
      return "identified";
    case ProtoIncidentPhase.MONITORING:
      return "monitoring";
    case ProtoIncidentPhase.RESOLVED:
      return "resolved";
    default:
      return "investigating";
  }
}

function toIncidentSeverity(s: ProtoIncidentSeverity): IncidentSeverity {
  switch (s) {
    case ProtoIncidentSeverity.CRITICAL:
      return "critical";
    case ProtoIncidentSeverity.MAJOR:
      return "major";
    case ProtoIncidentSeverity.MINOR:
      return "minor";
    default:
      return "minor";
  }
}

function toDomainComponent(p: ProtoComponent): PlatformComponent {
  return {
    name: p.name,
    displayName: p.displayName,
    description: p.description,
    group: p.group,
    link: p.link,
    portalRef: p.portalRef,
    declaredStatus: toComponentStatus(p.declaredStatus),
    computedStatus: toComponentStatus(p.computedStatus),
    activeIncidents: p.activeIncidents,
    lastStatusChange: timestampToIso(p.lastStatusChange),
    dailyWorstStatus: p.dailyWorstStatus.map(
      (d): DailyStatus => ({
        date: d.date,
        worstStatus: toComponentStatus(d.worstStatus),
      })
    ),
  };
}

function toDomainMaintenance(p: ProtoMaintenance): Maintenance {
  return {
    name: p.name,
    title: p.title,
    description: p.description,
    portalRef: p.portalRef,
    components: p.components,
    scheduledStart: timestampToIso(p.scheduledStart),
    scheduledEnd: timestampToIso(p.scheduledEnd),
    affectedStatus: p.affectedStatus,
    phase: toMaintenancePhase(p.phase),
  };
}

function toDomainIncidentUpdate(p: ProtoIncidentUpdate): IncidentUpdate {
  return {
    timestamp: timestampToIso(p.timestamp),
    phase: toIncidentPhase(p.phase),
    message: p.message,
  };
}

function toDomainIncident(p: ProtoIncident): Incident {
  return {
    name: p.name,
    title: p.title,
    portalRef: p.portalRef,
    components: p.components,
    severity: toIncidentSeverity(p.severity),
    currentPhase: toIncidentPhase(p.currentPhase),
    updates: p.updates.map(toDomainIncidentUpdate),
    startedAt: timestampToIso(p.startedAt),
    resolvedAt: timestampToIso(p.resolvedAt),
    durationMinutes: p.durationMinutes,
  };
}

export async function listComponents(
  portalRef?: string,
  group?: string
): Promise<PlatformComponent[]> {
  const request = create(ListComponentsRequestSchema, {
    portalRef: portalRef ?? "",
    group: group ?? "",
  });
  const response = await client.listComponents(request);
  return response.components.map(toDomainComponent);
}

export async function listMaintenances(
  portalRef?: string
): Promise<Maintenance[]> {
  const request = create(ListMaintenancesRequestSchema, {
    portalRef: portalRef ?? "",
  });
  const response = await client.listMaintenances(request);
  return response.maintenances.map(toDomainMaintenance);
}

export async function listIncidents(
  portalRef?: string
): Promise<Incident[]> {
  const request = create(ListIncidentsRequestSchema, {
    portalRef: portalRef ?? "",
  });
  const response = await client.listIncidents(request);
  return response.incidents.map(toDomainIncident);
}
