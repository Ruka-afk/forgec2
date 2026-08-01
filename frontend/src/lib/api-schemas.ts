import { z } from "zod";

// ── Agent (Implant) ──

export const AgentSchema = z.object({
  id: z.string().uuid(),
  hostname: z.string(),
  username: z.string(),
  os: z.string(),
  os_arch: z.string().optional(),
  pid: z.number().int().optional(),
  process_name: z.string().optional(),
  internal_ip: z.string().optional(),
  external_ip: z.string().optional(),
  listener_id: z.string().optional(),
  transport: z.string().optional(),
  status: z.enum(["online", "stale", "offline", "dead"]).optional(),
  last_seen: z.string().datetime().optional(),
  first_seen: z.string().datetime().optional(),
  notes: z.string().optional(),
  tags: z.string().optional(),
  parent_id: z.string().optional(),
  integrity: z.number().int().min(0).max(3).optional(),
  threat_level: z.number().int().min(0).max(3).optional(),
  sleep: z.number().int().optional(),
  jitter: z.number().optional(),
});
export type Agent = z.infer<typeof AgentSchema>;

export const AgentListResponseSchema = z.object({
  success: z.boolean(),
  data: z.array(AgentSchema),
  total: z.number().int(),
});
export type AgentListResponse = z.infer<typeof AgentListResponseSchema>;

export const AgentDetailResponseSchema = z.object({
  success: z.boolean(),
  data: AgentSchema,
});
export type AgentDetailResponse = z.infer<typeof AgentDetailResponseSchema>;

// ── Task ──

export const TaskStatusEnum = z.enum(["pending", "running", "completed", "failed", "cancelled", "aborted"]);
export type TaskStatus = z.infer<typeof TaskStatusEnum>;

export const TaskSchema = z.object({
  id: z.number().int(),
  agent_id: z.string().uuid(),
  type: z.string(),
  command: z.string().optional(),
  status: TaskStatusEnum,
  result: z.string().optional(),
  error: z.string().optional(),
  created_at: z.string().datetime().optional(),
  started_at: z.string().datetime().nullable().optional(),
  completed_at: z.string().datetime().nullable().optional(),
  priority: z.number().int().min(0).max(3).optional(),
});
export type Task = z.infer<typeof TaskSchema>;

export const TaskListResponseSchema = z.object({
  success: z.boolean(),
  data: z.array(TaskSchema),
  total: z.number().int(),
});
export type TaskListResponse = z.infer<typeof TaskListResponseSchema>;

// ── Dashboard Stats ──

export const DashboardStatsSchema = z.object({
  total_agents: z.number().int(),
  online_agents: z.number().int(),
  stale_agents: z.number().int(),
  offline_agents: z.number().int(),
  pending_tasks: z.number().int(),
  total_listeners: z.number().int().optional(),
});
export type DashboardStats = z.infer<typeof DashboardStatsSchema>;

export const DashboardStatsResponseSchema = z.object({
  success: z.boolean(),
  data: DashboardStatsSchema.optional(),
});

// ── Listener ──

export const ListenerSchema = z.object({
  id: z.number().int(),
  name: z.string(),
  type: z.string(),
  bind_address: z.string(),
  bind_port: z.number().int(),
  enabled: z.boolean(),
  transport: z.string().optional(),
  created_at: z.string().datetime().optional(),
});
export type Listener = z.infer<typeof ListenerSchema>;

export const ListenerListResponseSchema = z.object({
  success: z.boolean(),
  data: z.array(ListenerSchema),
});

// ── Generic API Response ──

export const ApiErrorResponseSchema = z.object({
  success: z.literal(false),
  error: z.string(),
});

export const ApiSuccessResponseSchema = z.object({
  success: z.literal(true),
});

export function parseResponse<T>(schema: z.ZodType<T>, data: unknown): T {
  const result = schema.safeParse(data);
  if (!result.success) {
    console.error("API response validation failed:", result.error.issues);
    throw new Error("Invalid API response format");
  }
  return result.data;
}
