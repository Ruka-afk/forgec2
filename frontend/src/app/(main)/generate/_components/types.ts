export type { Listener } from "@/types/listener";
export type {
  ProfilePreset, SharedState, EXEForm, PS1Form, LinuxForm, MacOSForm,
  StagerForm, ShellcodeForm, DonutForm, OneLinerForm,
  BusyState, Results, OneLinerType, OneLinerData,
} from "@/types/generate";

export interface ToastMessage {
  id: number;
  text: string;
  type: "success" | "error" | "info";
}
