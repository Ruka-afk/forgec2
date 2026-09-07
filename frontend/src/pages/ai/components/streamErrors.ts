import { handleUnauthorized } from "@/lib/api";

const MAX_ERROR_CHARS = 800;

/** Read the structured error returned before an SSE stream starts. HTML and
 * oversized proxy bodies are deliberately ignored so login/proxy pages are
 * never dumped into the conversation. */
export async function readAIResponseError(response: Response): Promise<string | null> {
  handleUnauthorized(response);
  const contentType = response.headers.get("content-type") || "";
  if (!contentType.toLowerCase().includes("application/json")) return null;
  try {
    const body = await response.json() as { error?: unknown; message?: unknown };
    const candidate = typeof body.error === "string"
      ? body.error
      : typeof body.message === "string"
        ? body.message
        : "";
    const normalized = candidate.trim();
    if (!normalized) return null;
    return normalized.length <= MAX_ERROR_CHARS
      ? normalized
      : `${normalized.slice(0, MAX_ERROR_CHARS)}…`;
  } catch {
    return null;
  }
}
