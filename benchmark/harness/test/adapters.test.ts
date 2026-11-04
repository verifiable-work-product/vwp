import * as http from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { AnthropicAdapter, MODELS_WITHOUT_TEMPERATURE } from '../src/adapters/anthropic.js';
import { OpenAIAdapter } from '../src/adapters/openai.js';
import type { ChatMessage, ProviderAdapter } from '../src/adapters/types.js';

// A local stand-in for the provider APIs: records every request body and
// answers in the provider's wire format — a minimal completion, or a refusal
// for the model names below.
const REFUSED = {
  anthropic: 'claude-refusal-test',
  openaiContentFilter: 'gpt-content-filter-test',
  openaiRefusal: 'gpt-refusal-test',
};
const CUT_OFF = {
  anthropicInToolCall: 'claude-cut-in-tool-call-test',
  anthropicInText: 'claude-cut-in-text-test',
  openaiLength: 'gpt-length-test',
};
const bodies: Record<string, unknown>[] = [];
let server: http.Server;
let baseUrl: string;

function anthropicResponse(model: string): unknown {
  if (model === REFUSED.anthropic) {
    return {
      content: [],
      stop_reason: 'refusal',
      stop_details: { type: 'refusal', category: 'reasoning_extraction', explanation: 'Blocked by policy.' },
      usage: { input_tokens: 1, output_tokens: 0 },
    };
  }
  const complete = { type: 'tool_use', id: 'toolu_1', name: 'add_fact', input: { factType: 'a', anchors: [] } };
  if (model === CUT_OFF.anthropicInToolCall) {
    const cut = { type: 'tool_use', id: 'toolu_2', name: 'add_fact', input: { factType: 'b' } };
    return { content: [{ type: 'text', text: 'Adding facts.' }, complete, cut], stop_reason: 'max_tokens', usage: { input_tokens: 1, output_tokens: 16000 } };
  }
  if (model === CUT_OFF.anthropicInText) {
    return { content: [complete, { type: 'text', text: 'Next I will' }], stop_reason: 'max_tokens', usage: { input_tokens: 1, output_tokens: 16000 } };
  }
  return { content: [{ type: 'text', text: 'ok' }], stop_reason: 'end_turn', usage: { input_tokens: 1, output_tokens: 1 } };
}

function openaiResponse(model: string): unknown {
  const usage = { prompt_tokens: 1, completion_tokens: 1 };
  if (model === REFUSED.openaiContentFilter) return { choices: [{ message: { content: null }, finish_reason: 'content_filter' }], usage };
  if (model === REFUSED.openaiRefusal) {
    return { choices: [{ message: { content: null, refusal: "I can't help with that." }, finish_reason: 'stop' }], usage };
  }
  if (model === CUT_OFF.openaiLength) {
    const call = (id: string, args: string) => ({ id, type: 'function', function: { name: 'add_fact', arguments: args } });
    return { choices: [{ message: { content: null, tool_calls: [call('call_1', '{"factType":"a"}'), call('call_2', '{"factType":"b","anch')] }, finish_reason: 'length' }], usage };
  }
  return { choices: [{ message: { content: 'ok' }, finish_reason: 'stop' }], usage };
}

beforeAll(async () => {
  server = http.createServer((req, res) => {
    let raw = '';
    req.on('data', (chunk) => (raw += chunk));
    req.on('end', () => {
      const body = JSON.parse(raw);
      bodies.push(body);
      res.setHeader('content-type', 'application/json');
      res.end(JSON.stringify(req.url?.endsWith('/chat/completions') ? openaiResponse(body.model) : anthropicResponse(body.model)));
    });
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  baseUrl = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
});

afterAll(() => new Promise<void>((resolve) => server.close(() => resolve())));

async function sentBody(adapter: ProviderAdapter): Promise<Record<string, unknown>> {
  const before = bodies.length;
  const res = await adapter.complete([{ role: 'user', content: 'hi' }], []);
  expect(res.text).toBe('ok');
  expect(bodies).toHaveLength(before + 1);
  return bodies[before];
}

describe('anthropic adapter temperature', () => {
  it.each([...MODELS_WITHOUT_TEMPERATURE])('omits temperature for %s and reports the provider default', async (model) => {
    const adapter = new AnthropicAdapter(model, { apiKey: 'test', baseUrl });
    expect(adapter.temperature).toBeNull();
    const body = await sentBody(adapter);
    expect(body.model).toBe(model);
    expect(body).not.toHaveProperty('temperature');
  });

  // Accepted temperature 0 when checked against the Messages API on 2026-11-03.
  it.each(['claude-sonnet-4-6', 'claude-opus-4-6', 'claude-opus-4-5-20251101', 'claude-haiku-4-5-20251001', 'claude-sonnet-4-5-20250929'])(
    'sends temperature 0 for %s',
    async (model) => {
      const adapter = new AnthropicAdapter(model, { apiKey: 'test', baseUrl });
      expect(adapter.temperature).toBe(0);
      expect((await sentBody(adapter)).temperature).toBe(0);
    },
  );
});

describe('openai-compatible adapter temperature', () => {
  it('sends temperature 0', async () => {
    const adapter = new OpenAIAdapter('gpt-test', { apiKey: 'test', baseUrl: `${baseUrl}/v1` });
    expect(adapter.temperature).toBe(0);
    expect((await sentBody(adapter)).temperature).toBe(0);
  });
});

const HI: ChatMessage[] = [{ role: 'user', content: 'hi' }];

describe('refusals', () => {
  it('anthropic: reports stop_reason refusal with its stop_details category and explanation', async () => {
    const res = await new AnthropicAdapter(REFUSED.anthropic, { apiKey: 'test', baseUrl }).complete(HI, []);
    expect(res.stopReason).toBe('refusal');
    expect(res.toolCalls).toEqual([]);
    expect(res.refusal).toEqual({ category: 'reasoning_extraction', explanation: 'Blocked by policy.' });
  });

  it('openai: reports finish_reason content_filter', async () => {
    const res = await new OpenAIAdapter(REFUSED.openaiContentFilter, { apiKey: 'test', baseUrl: `${baseUrl}/v1` }).complete(HI, []);
    expect(res.refusal).toEqual({ category: 'content_filter', explanation: null });
  });

  it('openai: reports a message.refusal', async () => {
    const res = await new OpenAIAdapter(REFUSED.openaiRefusal, { apiKey: 'test', baseUrl: `${baseUrl}/v1` }).complete(HI, []);
    expect(res.refusal).toEqual({ category: null, explanation: "I can't help with that." });
  });

  it('reports no refusal on a normal completion', async () => {
    expect((await new AnthropicAdapter('claude-opus-5-5', { apiKey: 'test', baseUrl }).complete(HI, [])).refusal).toBeUndefined();
    expect((await new OpenAIAdapter('gpt-test', { apiKey: 'test', baseUrl: `${baseUrl}/v1` }).complete(HI, [])).refusal).toBeUndefined();
  });
});

describe('output token cap', () => {
  it('anthropic: sends max_tokens 16000 by default', async () => {
    expect((await sentBody(new AnthropicAdapter('claude-opus-5-5', { apiKey: 'test', baseUrl }))).max_tokens).toBe(16_000);
  });

  it('anthropic: flags the last tool call when max_tokens cut it off', async () => {
    const res = await new AnthropicAdapter(CUT_OFF.anthropicInToolCall, { apiKey: 'test', baseUrl }).complete(HI, []);
    expect(res.stopReason).toBe('max_tokens');
    expect(res.toolCalls.map((c) => c.id)).toEqual(['toolu_1', 'toolu_2']);
    expect(res.truncatedToolCallId).toBe('toolu_2');
  });

  it('anthropic: flags nothing when max_tokens cut off text after complete tool calls', async () => {
    const res = await new AnthropicAdapter(CUT_OFF.anthropicInText, { apiKey: 'test', baseUrl }).complete(HI, []);
    expect(res.stopReason).toBe('max_tokens');
    expect(res.truncatedToolCallId).toBeUndefined();
  });

  it('openai: flags the last tool call on finish_reason length', async () => {
    const res = await new OpenAIAdapter(CUT_OFF.openaiLength, { apiKey: 'test', baseUrl: `${baseUrl}/v1` }).complete(HI, []);
    expect(res.truncatedToolCallId).toBe('call_2');
  });

  it('flags nothing on a normal completion', async () => {
    expect((await new AnthropicAdapter('claude-opus-5-5', { apiKey: 'test', baseUrl }).complete(HI, [])).truncatedToolCallId).toBeUndefined();
    expect((await new OpenAIAdapter('gpt-test', { apiKey: 'test', baseUrl: `${baseUrl}/v1` }).complete(HI, [])).truncatedToolCallId).toBeUndefined();
  });
});
