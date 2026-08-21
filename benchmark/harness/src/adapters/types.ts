/**
 * Provider adapter interface (harness DESIGN §7). Thin by design: the loop,
 * prompt, tools, and budgets are identical for every model; only this
 * translation layer differs.
 */

export interface ToolSpec {
  name: string;
  description: string;
  /** JSON Schema for the tool input. */
  inputSchema: Record<string, unknown>;
}

export interface ToolCall {
  id: string;
  name: string;
  input: unknown;
}

export type ChatMessage =
  | { role: 'system'; content: string }
  | { role: 'user'; content: string }
  | { role: 'assistant'; content: string; toolCalls: ToolCall[] }
  | { role: 'tool'; toolCallId: string; name: string; content: string; isError?: boolean };

export interface Usage {
  inputTokens: number;
  outputTokens: number;
}

export interface CompletionResult {
  text: string;
  toolCalls: ToolCall[];
  usage: Usage;
  stopReason?: string;
}

export interface ProviderAdapter {
  readonly provider: string;
  readonly model: string;
  complete(messages: ChatMessage[], tools: ToolSpec[]): Promise<CompletionResult>;
}

/** Shared: fetch with small retry on transient provider errors (429/5xx). */
export async function fetchWithRetry(url: string, init: RequestInit, attempts = 3): Promise<Response> {
  let lastError: Error | undefined;
  for (let attempt = 1; attempt <= attempts; attempt++) {
    try {
      const res = await fetch(url, init);
      if (res.status === 429 || res.status >= 500) {
        const body = await res.text().catch(() => '');
        lastError = new Error(`${res.status} ${res.statusText}: ${body.slice(0, 300)}`);
      } else {
        return res;
      }
    } catch (err) {
      lastError = err as Error;
    }
    if (attempt < attempts) {
      await new Promise((resolve) => setTimeout(resolve, 2000 * attempt));
    }
  }
  throw new Error(`request to ${url} failed after ${attempts} attempts: ${lastError?.message}`);
}
