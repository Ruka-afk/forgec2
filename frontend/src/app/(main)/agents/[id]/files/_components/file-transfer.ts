import { API_BASE } from "@/lib/constants";
import { api, getCsrfToken, pollTask } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { downloadBlob } from "@/lib/download";
import { exfilBasename, fileTaskId, pullPlan, transferProgressAt, type TransferProgress } from "./file-task";

type TFn = (key: string, params?: Record<string, string | number>) => string;

export async function pullRemoteFile(opts: {
  agentId: string;
  remotePath: string;
  filename: string;
  fileSize?: number;
  t: TFn;
  signal?: AbortSignal;
  onProgress?: (p: TransferProgress) => void;
}): Promise<"pulled" | "partial"> {
  const plan = pullPlan(opts.fileSize ?? 0);
  let offset = 0;
  opts.onProgress?.(transferProgressAt(0, plan));
  while (offset < plan.total) {
    if (opts.signal?.aborted) throw new DOMException("aborted", "AbortError");
    const size = Math.min(plan.chunk, plan.total - offset);
    const data = await api.post(paths.agents.filesExfil(opts.agentId), {
      path: opts.remotePath,
      offset: String(offset),
      size: String(size),
    });
    const taskId = fileTaskId(data);
    if (!taskId) throw new Error(opts.t("agents.files_pull_failed"));
    const st = await pollTask(opts.agentId, taskId, { timeoutMs: 180_000, signal: opts.signal });
    if (st.status === "failed") throw new Error(st.error || opts.t("agents.files_pull_failed"));
    offset += size;
    opts.onProgress?.(transferProgressAt(offset, plan));
  }
  const { blob, filename: saved } = await api.downloadGet(
    paths.agents.filesExfilGet(opts.agentId, exfilBasename(opts.remotePath)),
  );
  downloadBlob(blob, saved || opts.filename);
  return plan.partial ? "partial" : "pulled";
}

export function pushLocalFile(opts: {
  agentId: string;
  destPath: string;
  file: File;
  t: TFn;
  onProgress?: (pct: number) => void;
  signal?: AbortSignal;
}): Promise<void> {
  return new Promise((resolve, reject) => {
    const formData = new FormData();
    formData.append("file", opts.file);
    formData.append("target_path", opts.destPath);
    formData.append("path", opts.destPath);
    const xhr = new XMLHttpRequest();
    xhr.open("POST", `${API_BASE}${paths.agents.filesPush(opts.agentId)}`);
    xhr.setRequestHeader("X-CSRF-Token", getCsrfToken());
    xhr.withCredentials = true;
    const onAbort = () => {
      xhr.abort();
      reject(new DOMException("aborted", "AbortError"));
    };
    opts.signal?.addEventListener("abort", onAbort, { once: true });
    xhr.upload.addEventListener("progress", (evt) => {
      if (evt.lengthComputable) opts.onProgress?.(Math.round((evt.loaded / evt.total) * 100));
    });
    xhr.addEventListener("load", () => {
      opts.signal?.removeEventListener("abort", onAbort);
      void (async () => {
        try {
          if (xhr.status < 200 || xhr.status >= 300) {
            throw new Error(opts.t("agents.files_upload_failed_status", { status: xhr.status }));
          }
          let ack: unknown = {};
          try { ack = JSON.parse(xhr.responseText || "{}"); } catch { /* non-json */ }
          const taskId = fileTaskId(ack);
          if (taskId) {
            const st = await pollTask(opts.agentId, taskId, { timeoutMs: 180_000, signal: opts.signal });
            if (st.status === "failed") throw new Error(st.error || opts.t("agents.files_push_failed"));
          }
          resolve();
        } catch (err) {
          reject(err);
        }
      })();
    });
    xhr.addEventListener("error", () => {
      opts.signal?.removeEventListener("abort", onAbort);
      reject(new Error(opts.t("agents.files_upload_failed_network")));
    });
    xhr.send(formData);
  });
}
