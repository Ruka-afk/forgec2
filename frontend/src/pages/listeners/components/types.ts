export interface Listener {
  id?: string;
  ID?: string;
  name?: string;
  Name?: string;
  type?: string;
  Type?: string;
  Scheme?: string;
  scheme?: string;
  Protocol?: string;
  protocol?: string;
  host?: string;
  Host?: string;
  port?: number | string;
  Port?: number | string;
  enabled?: boolean;
  Enabled?: boolean;
  notes?: string;
  Notes?: string;
  tags?: string;
  Tags?: string;
  color?: string;
  Color?: string;
  status?: string;
  Status?: string;
  // Per-listener TLS posture for https/tls listeners. Empty/false means
  // "inherit the server-wide setting"; the server only ever lets these
  // strengthen that posture.
  client_ca_file?: string;
  require_client_cert?: boolean;
  min_tls_version?: string;
}

export type CreateListenerForm = {
  name: string;
  type: string;
  host: string;
  port: string;
  protocol: string;
  tags: string;
  color: string;
  client_ca_file: string;
  require_client_cert: boolean;
  min_tls_version: string;
};

export type EditListenerForm = CreateListenerForm & {
  notes: string;
};

export const emptyCreateForm = (): CreateListenerForm => ({
  name: "",
  type: "http",
  host: "0.0.0.0",
  port: "8443",
  protocol: "http",
  tags: "",
  color: "",
  client_ca_file: "",
  require_client_cert: false,
  min_tls_version: "",
});

export const emptyEditForm = (): EditListenerForm => ({
  ...emptyCreateForm(),
  notes: "",
  port: "",
});

/** Per-listener TLS knobs only apply to these schemes (mirrors the server rule). */
export function isTLSScheme(scheme: string | undefined | null): boolean {
  const s = (scheme || "").trim().toLowerCase();
  return s === "https" || s === "tls";
}
