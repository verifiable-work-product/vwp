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
 * Fixed output cap per response, thinking included (DESIGN §6). Responses are
 * streamed, so a capped response is never cut short by an HTTP timeout: on
 * 2026-11-04 Fable 5.1 measured ~68 tokens/s, ~3.9 min for 16k tokens.
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
      // Streamed so a long response never waits on a single HTTP response
      // (Node fetch gives up on response headers after 5 minutes).
      stream: true,
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
      const hint = errBody.includes('`temperature` is deprecated')
        ? ` — add '${this.model}' to MODELS_WITHOUT_TEMPERATURE in src/adapters/anthropic.ts`
        : '';
      throw new Error(`Anthropic API error ${res.status}: ${errBody.slice(0, 500)}${hint}`);
    }
    if (!res.body) throw new Error('Anthropic API returned no response body');
    return readMessageStream(res.body);
  }
}

interface StreamedBlock {
  type: string;
  id?: string;
  name?: string;
  text: string;
  partialJson: string;
  stopped: boolean;
}

/** Folds a Messages API event stream into a CompletionResult. */
export async function readMessageStream(stream: ReadableStream<Uint8Array>): Promise<CompletionResult> {
  const blocks: StreamedBlock[] = [];
  let stopReason: string | undefined;
  let stopDetails: any = null;
  let inputTokens = 0;
  let outputTokens = 0;
  let finished = false;

  for await (const event of serverSentEvents(stream)) {
    switch (event.type) {
      case 'message_start':
        inputTokens = event.message?.usage?.input_tokens ?? 0;
        outputTokens = event.message?.usage?.output_tokens ?? 0;
        break;
      case 'content_block_start':
        blocks[event.index] = {
          type: event.content_block?.type,
          id: event.content_block?.id,
          name: event.content_block?.name,
          text: event.content_block?.text ?? '',
          partialJson: '',
          stopped: false,
        };
        break;
      case 'content_block_delta': {
        const block = blocks[event.index];
        if (event.delta?.type === 'text_delta') block.text += event.delta.text;
        if (event.delta?.type === 'input_json_delta') block.partialJson += event.delta.partial_json;
        break;
      }
      case 'content_block_stop':
        blocks[event.index].stopped = true;
        break;
      case 'message_delta':
        stopReason = event.delta?.stop_reason ?? stopReason;
        stopDetails = event.delta?.stop_details ?? stopDetails;
        inputTokens = event.usage?.input_tokens ?? inputTokens;
        outputTokens = event.usage?.output_tokens ?? outputTokens;
        break;
      case 'message_stop':
        finished = true;
        break;
      case 'error':
        throw new Error(`Anthropic API stream error: ${JSON.stringify(event.error).slice(0, 500)}`);
      default:
        // ping, and thinking/signature deltas: nothing to keep.
        break;
    }
  }
  if (!finished) throw new Error('Anthropic API stream ended before message_stop');

  const toolCalls: ToolCall[] = blocks
    .filter((b) => b?.type === 'tool_use')
    .map((b) => ({ id: b.id ?? '', name: b.name ?? '', input: parseToolInput(b) }));
  // A cut-off block gets no content_block_stop, so its input is incomplete.
  const last = blocks.at(-1);
  const hitOutputCap = stopReason === 'max_tokens';
  return {
    text: blocks.filter((b) => b?.type === 'text').map((b) => b.text).join(''),
    toolCalls,
    usage: { inputTokens, outputTokens },
    stopReason,
    // Branch on stop_reason; stop_details is informational and may be null.
    ...(stopReason === 'refusal'
      ? { refusal: { category: stopDetails?.category ?? null, explanation: stopDetails?.explanation ?? null } }
      : {}),
    ...(hitOutputCap ? { hitOutputCap } : {}),
    ...(hitOutputCap && last?.type === 'tool_use' && !last.stopped ? { lastToolCallTruncated: true } : {}),
  };
}

function parseToolInput(block: StreamedBlock): unknown {
  if (block.partialJson.length === 0) return {};
  try {
    return JSON.parse(block.partialJson);
  } catch {
    return { __unparsedInput: block.partialJson };
  }
}

/** Yields each server-sent event's JSON data payload. */
async function* serverSentEvents(stream: ReadableStream<Uint8Array>): AsyncGenerator<any> {
  const decoder = new TextDecoder();
  let buffer = '';
  for await (const chunk of stream) {
    buffer += decoder.decode(chunk, { stream: true }).replace(/\r/g, '');
    let boundary = buffer.indexOf('\n\n');
    while (boundary !== -1) {
      const data = buffer
        .slice(0, boundary)
        .split('\n')
        .filter((line) => line.startsWith('data:'))
        .map((line) => line.slice(5).trimStart())
        .join('\n');
      buffer = buffer.slice(boundary + 2);
      if (data.length > 0) yield JSON.parse(data);
      boundary = buffer.indexOf('\n\n');
    }
  }
}
