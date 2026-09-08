/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_FORGEC2_API_BASE?: string;
  readonly VITE_FORGEC2_WS_URL?: string;
  readonly VITE_FORGEC2_BACKEND_PORT?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
