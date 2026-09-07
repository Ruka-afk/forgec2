import type { NormalizedAgent as Agent } from "@/types/agent";

export interface Campaign {
  id: string;
  name: string;
  description: string;
  status: string;
  created_at: string;
  updated_at: string;
  agents: Agent[];
}

export interface PhaseEvent {
  phase: string;
  first_seen: string;
  task_count: number;
}

export interface AgentStat {
  agent_id: string;
  hostname: string;
  username: string;
  ip: string;
  task_count: number;
  phases: Record<string, number>;
}

export interface CampaignStats {
  total_agents: number;
  total_tasks: number;
  completed_tasks: number;
  failed_tasks: number;
  kill_chain_summary: Record<string, number>;
  phase_timeline: PhaseEvent[];
  agent_breakdown: AgentStat[];
}

interface KillChainPhaseStatus {
  phase: string;
  status: "pending" | "completed";
  task_count: number;
}

export interface CampaignMITRE {
  campaign_id: string;
  campaign_name: string;
  phases: KillChainPhaseStatus[];
  timeline: PhaseEvent[];
}

export interface KillChainTemplate {
  name: string;
  description: string;
  steps: KillChainStepDef[];
}

interface KillChainStepDef {
  phase: string;
  task_type: string;
  params: Record<string, string>;
  wait_time: number;
}

export interface PhaseTask {
  phase: string;
  task_type: string;
  status: string;
  hostname?: string;
  agent_id?: string;
}

export const PHASE_ORDER = [
  "Reconnaissance",
  "Resource Development",
  "Initial Access",
  "Execution",
  "Persistence",
  "Privilege Escalation",
  "Defense Evasion",
  "Credential Access",
  "Discovery",
  "Lateral Movement",
  "Collection",
  "Command and Control",
  "Exfiltration",
  "Impact",
];

