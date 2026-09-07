export type { Listener } from "@/types/listener";
export type {
  ProfilePreset, SharedState, BinaryForm, UnixForm, PS1Form, StagerForm,
  ShellcodeForm, DonutForm, OneLinerForm, BinaryVariant, UnixVariant, StagerVariant,
  GenerateState, GenerateResult, PS1Result, OneLinerResult,
  PayloadForms, PayloadStates, PayloadExtras, PayloadKey,
  DEFAULT_BINARY_FORM, DEFAULT_UNIX_FORM, DEFAULT_PS1_FORM, DEFAULT_STAGER_FORM,
  DEFAULT_SHELLCODE_FORM, DEFAULT_DONUT_FORM, DEFAULT_ONELINER_FORM,
  createDefaultForms, createDefaultStates,
  OneLinerType, OneLinerData, BuildHistoryEntry,
} from "@/types/generate";

import { z } from "zod";

// Schema-driven clamps for shared beacon settings. Values entered in the
// Connection panel are coerced + clamped through these instead of ad-hoc
// Math.min/Math.max chains so the valid range lives in one place.
const intervalSchema = z.coerce.number().int().transform((v) => Math.min(86400, Math.max(1, v)));
const jitterSchema = z.coerce.number().int().transform((v) => Math.min(100, Math.max(0, v)));

export function clampInterval(raw: string): string {
  const r = intervalSchema.safeParse(raw);
  return r.success ? String(r.data) : "1";
}

export function clampJitter(raw: string): string {
  const r = jitterSchema.safeParse(raw);
  return r.success ? String(r.data) : "0";
}
