/**
 * Anthropic Messages API adapter. Env: ANTHROPIC_API_KEY.
 */
import type { ChatMessage, CompletionResult, ProviderAdapter, ToolCall, ToolSpec } from './types.js';
import { fetchWithRetry } from './types.js';

const API_VERSION = '2023-06-01';

/**
 * Claude models that reject sampling parameters: any `temperature` returns
 * 400 "`temperature` is deprecated for this model". They run at the provider
 * default, recorded in the transcript and provenance notes (DESIGN §6).
 * Verified against the Messages API on 2026-11-03 for every model it listed;
 * the Mythos ids follow Anthropic's docs (same request surface as Fable). A
 * model missing from this list fails loudly on its first request.
 */
export const MODELS_WITHOUT_TEMPERATURE: ReadonlySet<string> = new Set([
  'claude-fable-5-1',
  'claude-fable-5',
  'claude-mythos-5-1',
  'claude-mythos-5',
  'claude-opus-5-5',
  'claude-opus-5',
  'claude-opus-4-8',
  'claude-opus-4-7',
  'claude-sonnet-5-5',
  'claude-sonnet-5',
  'claude-haiku-5-5',
]);

/**
 * Fixed output cap per response (DESIGN §6). Requests are non-streaming: at
 * the ~120 tokens/s Opus 5.5 showed on 2026-11-03 a capped response takes
 * ~2.2 min, inside Node fetch's 5-minute response-header timeout. A higher
 * cap needs streaming.
 */
export const DEFAULT_MAX_TOKENS = 16_000;

/* eslint-disable @typescript-eslint/no-explicit-any */

interface AnthropicOptions {
  apiKey?: string;
  baseUrl?: string;
  maxTokens?: number;
}

export class AnthropicAdapter implements ProviderAdapter {
  readonly provider = 'anthropic';
  readonly temperature: number | null;
  private readonly apiKey: string;
  private readonly baseUrl: string;
  private readonly maxTokens: number;

  constructor(readonly model: string, opts: AnthropicOptions = {}) {
    const apiKey = opts.apiKey ?? process.env.ANTHROPIC_API_KEY;
    if (!apiKey) throw new Error('ANTHROPIC_API_KEY is not set');
    this.apiKey = apiKey;
    this.baseUrl = (opts.baseUrl ?? 'https://api.anthropic.com').replace(/\/$/, '');
    this.maxTokens = opts.maxTokens ?? DEFAULT_MAX_TOKENS;
    this.temperature = MODELS_WITHOUT_TEMPERATURE.has(model) ? null : 0;
  }

  async complete(messages: ChatMessage[], tools: ToolSpec[]): Promise<CompletionResult> {
    let system = '';
    const converted: any[] = [];
    for (const msg of messages) {
      if (msg.role === 'system') {
        system = system.length > 0 ? `${system}\n\n${msg.content}` : msg.content;
      } else if (msg.role === 'user') {
        converted.push({ role: 'user', content: msg.content });
      } else if (msg.role === 'assistant') {
        const blocks: any[] = [];
        if (msg.content.length > 0) blocks.push({ type: 'text', text: msg.content });
        for (const call of msg.toolCalls) {
          blocks.push({ type: 'tool_use', id: call.id, name: call.name, input: call.input ?? {} });
        }
        converted.push({ role: 'assistant', content: blocks });
      } else {
        // Tool results are user-role tool_result blocks; group consecutive ones.
        const block = {
          type: 'tool_result',
          tool_use_id: msg.toolCallId,
          content: msg.content,
          ...(msg.isError ? { is_error: true } : {}),
        };
        const last = converted[converted.length - 1];
        if (last && last.role === 'user' && Array.isArray(last.content) && last.content[0]?.type === 'tool_result') {
          last.content.push(block);
        } else {
          converted.push({ role: 'user', content: [block] });
        }
      }
    }

    const body = {
      model: this.model,
      max_tokens: this.maxTokens,
      ...(this.temperature === null ? {} : { temperature: this.temperature }),
      ...(system.length > 0 ? { system } : {}),
      messages: converted,
      tools: tools.map((t) => ({ name: t.name, description: t.description, input_schema: t.inputSchema })),
    };

    const res = await fetchWithRetry(`${this.baseUrl}/v1/messages`, {
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        'x-api-key': this.apiKey,
        'anthropic-version': API_VERSION,
      },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const errBody = await res.text().catch(() => '');
      throw new Error(`Anthropic API error ${res.status}: ${errBody.slice(0, 500)}`);
    }
    const data: any = await res.json();
    let text = '';
    const toolCalls: ToolCall[] = [];
    for (const block of data.content ?? []) {
      if (block.type === 'text') text += block.text;
      if (block.type === 'tool_use') toolCalls.push({ id: block.id, name: block.name, input: block.input });
    }
    // Cut off at max_tokens inside a tool_use block: that call's input is incomplete.
    const lastBlock = (data.content ?? []).at(-1);
    return {
      text,
      toolCalls,
      usage: {
        inputTokens: data.usage?.input_tokens ?? 0,
        outputTokens: data.usage?.output_tokens ?? 0,
      },
      stopReason: data.stop_reason ?? undefined,
      // Branch on stop_reason; stop_details is informational and may be null.
      ...(data.stop_reason === 'refusal'
        ? { refusal: { category: data.stop_details?.category ?? null, explanation: data.stop_details?.explanation ?? null } }
        : {}),
      ...(data.stop_reason === 'max_tokens' && lastBlock?.type === 'tool_use' ? { truncatedToolCallId: lastBlock.id } : {}),
    };
  }
}
