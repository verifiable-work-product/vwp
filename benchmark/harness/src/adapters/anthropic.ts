/**
 * Anthropic Messages API adapter. Env: ANTHROPIC_API_KEY.
 */
import type { ChatMessage, CompletionResult, ProviderAdapter, ToolCall, ToolSpec } from './types.js';
import { fetchWithRetry } from './types.js';

const API_VERSION = '2023-06-01';

/* eslint-disable @typescript-eslint/no-explicit-any */

interface AnthropicOptions {
  apiKey?: string;
  baseUrl?: string;
  maxTokens?: number;
}

export class AnthropicAdapter implements ProviderAdapter {
  readonly provider = 'anthropic';
  private readonly apiKey: string;
  private readonly baseUrl: string;
  private readonly maxTokens: number;

  constructor(readonly model: string, opts: AnthropicOptions = {}) {
    const apiKey = opts.apiKey ?? process.env.ANTHROPIC_API_KEY;
    if (!apiKey) throw new Error('ANTHROPIC_API_KEY is not set');
    this.apiKey = apiKey;
    this.baseUrl = (opts.baseUrl ?? 'https://api.anthropic.com').replace(/\/$/, '');
    this.maxTokens = opts.maxTokens ?? 8192;
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
      temperature: 0,
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
    return {
      text,
      toolCalls,
      usage: {
        inputTokens: data.usage?.input_tokens ?? 0,
        outputTokens: data.usage?.output_tokens ?? 0,
      },
      stopReason: data.stop_reason ?? undefined,
    };
  }
}
