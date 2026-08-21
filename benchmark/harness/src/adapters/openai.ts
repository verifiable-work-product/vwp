/**
 * OpenAI-compatible /v1/chat/completions adapter — also serves vLLM/Ollama
 * and other OpenAI-compatible servers (open-weight models run on the same
 * rails). Env: OPENAI_API_KEY, OPENAI_BASE_URL (default https://api.openai.com/v1).
 */
import type { ChatMessage, CompletionResult, ProviderAdapter, ToolCall, ToolSpec } from './types.js';
import { fetchWithRetry } from './types.js';

/* eslint-disable @typescript-eslint/no-explicit-any */

interface OpenAIOptions {
  apiKey?: string;
  baseUrl?: string;
}

export class OpenAIAdapter implements ProviderAdapter {
  readonly provider = 'openai';
  private readonly apiKey: string;
  private readonly baseUrl: string;

  constructor(readonly model: string, opts: OpenAIOptions = {}) {
    const apiKey = opts.apiKey ?? process.env.OPENAI_API_KEY;
    if (!apiKey) throw new Error('OPENAI_API_KEY is not set');
    this.apiKey = apiKey;
    this.baseUrl = (opts.baseUrl ?? process.env.OPENAI_BASE_URL ?? 'https://api.openai.com/v1').replace(/\/$/, '');
  }

  async complete(messages: ChatMessage[], tools: ToolSpec[]): Promise<CompletionResult> {
    const converted: any[] = [];
    for (const msg of messages) {
      if (msg.role === 'system') {
        converted.push({ role: 'system', content: msg.content });
      } else if (msg.role === 'user') {
        converted.push({ role: 'user', content: msg.content });
      } else if (msg.role === 'assistant') {
        converted.push({
          role: 'assistant',
          content: msg.content.length > 0 ? msg.content : null,
          ...(msg.toolCalls.length > 0
            ? {
                tool_calls: msg.toolCalls.map((c) => ({
                  id: c.id,
                  type: 'function',
                  function: { name: c.name, arguments: JSON.stringify(c.input ?? {}) },
                })),
              }
            : {}),
        });
      } else {
        converted.push({ role: 'tool', tool_call_id: msg.toolCallId, content: msg.content });
      }
    }

    const body = {
      model: this.model,
      temperature: 0,
      messages: converted,
      tools: tools.map((t) => ({
        type: 'function',
        function: { name: t.name, description: t.description, parameters: t.inputSchema },
      })),
    };

    const res = await fetchWithRetry(`${this.baseUrl}/chat/completions`, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        authorization: `Bearer ${this.apiKey}`,
      },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const errBody = await res.text().catch(() => '');
      throw new Error(`OpenAI-compatible API error ${res.status}: ${errBody.slice(0, 500)}`);
    }
    const data: any = await res.json();
    const message = data.choices?.[0]?.message ?? {};
    const toolCalls: ToolCall[] = [];
    for (const call of message.tool_calls ?? []) {
      let input: unknown = {};
      try {
        input = call.function?.arguments ? JSON.parse(call.function.arguments) : {};
      } catch {
        input = { __unparsedArguments: call.function?.arguments };
      }
      toolCalls.push({ id: call.id, name: call.function?.name, input });
    }
    return {
      text: typeof message.content === 'string' ? message.content : '',
      toolCalls,
      usage: {
        inputTokens: data.usage?.prompt_tokens ?? 0,
        outputTokens: data.usage?.completion_tokens ?? 0,
      },
      stopReason: data.choices?.[0]?.finish_reason ?? undefined,
    };
  }
}
