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
}

export type CreateListenerForm = {
  name: string;
  type: string;
  host: string;
  port: string;
  protocol: string;
  tags: string;
  color: string;
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
});

export const emptyEditForm = (): EditListenerForm => ({
  ...emptyCreateForm(),
  notes: "",
  port: "",
});
