export interface EvasionTechnique {
  value: string;
  labelKey: string;
  group: string;
}

export const EVASION_TECHNIQUES: EvasionTechnique[] = [
  { value: "amsi", labelKey: "agents.evasion.amsi", group: "AMSI" },
  { value: "amsi_session", labelKey: "agents.evasion.amsi_session", group: "AMSI" },
  { value: "amsi_hardware_bp", labelKey: "agents.evasion.amsi_hardware_bp", group: "AMSI" },
  { value: "etw", labelKey: "agents.evasion.etw", group: "ETW" },
  { value: "etw_ntrace", labelKey: "agents.evasion.etw_ntrace", group: "ETW" },
  { value: "etw_hardware_bp", labelKey: "agents.evasion.etw_hardware_bp", group: "ETW" },
  { value: "etwti", labelKey: "agents.evasion.etwti", group: "ETW" },
  { value: "blockdlls", labelKey: "agents.evasion.blockdlls", group: "Process" },
  { value: "protect_process", labelKey: "agents.evasion.protect_process", group: "Process" },
  { value: "unhook_ntdll", labelKey: "agents.evasion.unhook_ntdll", group: "Module" },
  { value: "kernel_callback", labelKey: "agents.evasion.kernel_callback", group: "Module" },
  { value: "enum_callbacks", labelKey: "agents.evasion.enum_callbacks", group: "Module" },
  { value: "objcb", labelKey: "agents.evasion.objcb", group: "Module" },
  { value: "imgload", labelKey: "agents.evasion.imgload", group: "Module" },
];

export const EVASION_GROUPS = ["AMSI", "ETW", "Process", "Module"];
