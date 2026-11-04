import * as http from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { AnthropicAdapter, MODELS_WITHOUT_TEMPERATURE, readMessageStream } from '../src/adapters/anthropic.js';
import { OpenAIAdapter } from '../src/adapters/openai.js';
import type { ChatMessage, ProviderAdapter } from '../src/adapters/types.js';

// A local stand-in for the provider APIs: records every request body and
// answers in the provider's wire format (an SSE event stream for Anthropic,
// as the adapter requests stream: true). Special model names select the
// responses below; anything else gets a minimal "ok" completion.
const MODEL = {
  anthropicRefusal: 'claude-refusal-test',
  anthropicCutInToolCall: 'claude-cut-in-tool-call-test',
  anthropicCutInText: 'claude-cut-in-text-test',
  anthropicCutInThinking: 'claude-cut-in-thinking-test',
  anthropicTemperature400: 'claude-temperature-400-test',
  anthropicOther400: 'claude-other-400-test',
  anthropicNoMessageStop: 'claude-no-message-stop-test',
  anthropicStreamError: 'claude-stream-error-test',
  anthropicCrlf: 'claude-crlf-test',
  openaiContentFilter: 'gpt-content-filter-test',
  openaiRefusal: 'gpt-refusal-test',
  openaiLengthCut: 'gpt-length-cut-test',
  openaiLengthComplete: 'gpt-length-complete-test',
  openaiNoIds: 'gpt-no-ids-test',
};

type StreamBlock =
  | { type: 'text'; text: string; stopped?: boolean }
  | { type: 'thinking'; stopped?: boolean }
  | { type: 'tool_use'; id: string; name: string; json: string; stopped?: boolean };

interface StreamSpec {
  blocks: StreamBlock[];
  stopReason: string;
  stopDetails?: unknown;
  error?: unknown;
  omitMessageStop?: boolean;
}

/** Renders a Messages API event stream the way the live API sends it (checked 2026-11-04). */
function messageStream(spec: StreamSpec): string {
  const events: Array<[string, unknown]> = [
    ['message_start', { type: 'message_start', message: { id: 'msg_test', role: 'assistant', content: [], stop_reason: null, usage: { input_tokens: 11, output_tokens: 1 } } }],
    ['ping', { type: 'ping' }],
  ];
  spec.blocks.forEach((block, index) => {
    if (block.type === 'text') {
      events.push(['content_block_start', { type: 'content_block_start', index, content_block: { type: 'text', text: '' } }]);
      events.push(['content_block_delta', { type: 'content_block_delta', index, delta: { type: 'text_delta', text: block.text } }]);
    } else if (block.type === 'thinking') {
      events.push(['content_block_start', { type: 'content_block_start', index, content_block: { type: 'thinking', thinking: '', signature: '' } }]);
      events.push(['content_block_delta', { type: 'content_block_delta', index, delta: { type: 'signature_delta', signature: 'sig' } }]);
    } else {
      events.push(['content_block_start', { type: 'content_block_start', index, content_block: { type: 'tool_use', id: block.id, name: block.name, input: {} } }]);
      events.push(['content_block_delta', { type: 'content_block_delta', index, delta: { type: 'input_json_delta', partial_json: block.json } }]);
    }
    // A block cut off by max_tokens gets no content_block_stop.
    if (block.stopped !== false) events.push(['content_block_stop', { type: 'content_block_stop', index }]);
  });
  if (spec.error) events.push(['error', { type: 'error', error: spec.error }]);
  events.push([
    'message_delta',
    { type: 'message_delta', delta: { stop_reason: spec.stopReason, stop_sequence: null, stop_details: spec.stopDetails ?? null }, usage: { output_tokens: 7 } },
  ]);
  if (!spec.omitMessageStop) events.push(['message_stop', { type: 'message_stop' }]);
  return events.map(([name, data]) => `event: ${name}\ndata: ${JSON.stringify(data)}\n\n`).join('');
}

const OK_STREAM: StreamSpec = { blocks: [{ type: 'text', text: 'ok' }], stopReason: 'end_turn' };
const COMPLETE_CALL: StreamBlock = { type: 'tool_use', id: 'toolu_1', name: 'add_fact', json: '{"factType":"a","anchors":[]}' };

function anthropicStream(model: string): string {
  switch (model) {
    case MODEL.anthropicRefusal:
      return messageStream({
        blocks: [],
        stopReason: 'refusal',
        stopDetails: { type: 'refusal', category: 'reasoning_extraction', explanation: 'Blocked by policy.' },
      });
    case MODEL.anthropicCutInToolCall:
      return messageStream({
        blocks: [{ type: 'text', text: 'Adding facts.' }, COMPLETE_CALL, { type: 'tool_use', id: 'toolu_2', name: 'add_fact', json: '', stopped: false }],
        stopReason: 'max_tokens',
      });
    case MODEL.anthropicCutInText:
      return messageStream({ blocks: [COMPLETE_CALL, { type: 'text', text: 'Next I will', stopped: false }], stopReason: 'max_tokens' });
    case MODEL.anthropicCutInThinking:
      return messageStream({ blocks: [{ type: 'thinking', stopped: false }], stopReason: 'max_tokens' });
    case MODEL.anthropicNoMessageStop:
      return messageStream({ ...OK_STREAM, omitMessageStop: true });
    case MODEL.anthropicStreamError:
      return messageStream({ ...OK_STREAM, error: { type: 'overloaded_error', message: 'Overloaded' } });
    case MODEL.anthropicCrlf:
      return messageStream(OK_STREAM).replace(/\n/g, '\r\n');
    default:
      return messageStream(OK_STREAM);
  }
}

function openaiResponse(model: string): unknown {
  const usage = { prompt_tokens: 1, completion_tokens: 1 };
  const call = (id: string | undefined, args: string) => ({ ...(id ? { id } : {}), type: 'function', function: { name: 'add_fact', arguments: args } });
  const withCalls = (calls: unknown[], finishReason: string) => ({ choices: [{ message: { content: null, tool_calls: calls }, finish_reason: finishReason }], usage });
  switch (model) {
    case MODEL.openaiContentFilter:
      return { choices: [{ message: { content: null }, finish_reason: 'content_filter' }], usage };
    case MODEL.openaiRefusal:
      return { choices: [{ message: { content: null, refusal: "I can't help with that." }, finish_reason: 'stop' }], usage };
    case MODEL.openaiLengthCut:
      return withCalls([call('call_1', '{"factType":"a"}'), call('call_2', '{"factType":"b","anch')], 'length');
    case MODEL.openaiLengthComplete:
      return withCalls([call('call_1', '{"factType":"a"}')], 'length');
    case MODEL.openaiNoIds:
      return withCalls([call(undefined, '{"factType":"a"}'), call(undefined, '{"factType":"b"}')], 'tool_calls');
    default:
      return { choices: [{ message: { content: 'ok' }, finish_reason: 'stop' }], usage };
  }
}

const bodies: Record<string, unknown>[] = [];
let server: http.Server;
let baseUrl: string;

beforeAll(async () => {
  server = http.createServer((req, res) => {
    let raw = '';
    req.on('data', (chunk) => (raw += chunk));
    req.on('end', () => {
      const body = JSON.parse(raw);
      bodies.push(body);
      if (req.url?.endsWith('/chat/completions')) {
        res.setHeader('content-type', 'application/json');
        res.end(JSON.stringify(openaiResponse(body.model)));
      } else if (body.model === MODEL.anthropicTemperature400 || body.model === MODEL.anthropicOther400) {
        const message = body.model === MODEL.anthropicTemperature400 ? '`temperature` is deprecated for this model.' : 'messages: field required';
        res.statusCode = 400;
        res.setHeader('content-type', 'application/json');
        res.end(JSON.stringify({ type: 'error', error: { type: 'invalid_request_error', message } }));
      } else {
        res.setHeader('content-type', 'text/event-stream');
        res.end(anthropicStream(body.model));
      }
    });
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  baseUrl = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
});

afterAll(() => new Promise<void>((resolve) => server.close(() => resolve())));

const HI: ChatMessage[] = [{ role: 'user', content: 'hi' }];
const anthropic = (model: string) => new AnthropicAdapter(model, { apiKey: 'test', baseUrl });
const openai = (model: string) => new OpenAIAdapter(model, { apiKey: 'test', baseUrl: `${baseUrl}/v1` });

async function sentBody(adapter: ProviderAdapter): Promise<Record<string, unknown>> {
  const before = bodies.length;
  const res = await adapter.complete(HI, []);
  expect(res.text).toBe('ok');
  expect(bodies).toHaveLength(before + 1);
  return bodies[before];
}

describe('anthropic adapter temperature', () => {
  // These returned 400 "`temperature` is deprecated for this model" from the
  // Messages API on 2026-11-03; the Mythos ids follow Anthropic's docs.
  // Re-verify against the API before editing either list.
  it('lists exactly the models verified to reject sampling parameters', () => {
    expect([...MODELS_WITHOUT_TEMPERATURE].sort()).toEqual(
      [
        'claude-fable-5',
        'claude-fable-5-1',
        'claude-haiku-5-5',
        'claude-mythos-5',
        'claude-mythos-5-1',
        'claude-opus-4-7',
        'claude-opus-4-8',
        'claude-opus-5',
        'claude-opus-5-5',
        'claude-sonnet-5',
        'claude-sonnet-5-5',
      ].sort(),
    );
  });

  it.each([...MODELS_WITHOUT_TEMPERATURE])('omits temperature for %s and reports the provider default', async (model) => {
    const adapter = anthropic(model);
    expect(adapter.temperature).toBeNull();
    const body = await sentBody(adapter);
    expect(body.model).toBe(model);
    expect(body).not.toHaveProperty('temperature');
  });

  // Accepted temperature 0 when checked against the Messages API on 2026-11-03.
  it.each(['claude-sonnet-4-6', 'claude-opus-4-6', 'claude-opus-4-5-20251101', 'claude-haiku-4-5-20251001', 'claude-sonnet-4-5-20250929'])(
    'sends temperature 0 for %s',
    async (model) => {
      const adapter = anthropic(model);
      expect(adapter.temperature).toBe(0);
      expect((await sentBody(adapter)).temperature).toBe(0);
    },
  );

  it('points at the model list when an unlisted model rejects temperature', async () => {
    await expect(anthropic(MODEL.anthropicTemperature400).complete(HI, [])).rejects.toThrow(
      /400.*temperature.*add 'claude-temperature-400-test' to MODELS_WITHOUT_TEMPERATURE/s,
    );
  });

  it('adds no model-list hint to other API errors', async () => {
    const error = await anthropic(MODEL.anthropicOther400).complete(HI, []).catch((err: Error) => err);
    expect(String(error)).toContain('Anthropic API error 400');
    expect(String(error)).toContain('messages: field required');
    expect(String(error)).not.toContain('MODELS_WITHOUT_TEMPERATURE');
  });
});

describe('openai-compatible adapter temperature', () => {
  it('sends temperature 0', async () => {
    const adapter = openai('gpt-test');
    expect(adapter.temperature).toBe(0);
    expect((await sentBody(adapter)).temperature).toBe(0);
  });
});

describe('anthropic streaming', () => {
  it('requests a stream and folds it into text, tool calls, and usage', async () => {
    expect((await sentBody(anthropic('claude-opus-5-5'))).stream).toBe(true);
    const res = await anthropic(MODEL.anthropicCutInText).complete(HI, []);
    expect(res.toolCalls).toEqual([{ id: 'toolu_1', name: 'add_fact', input: { factType: 'a', anchors: [] } }]);
    expect(res.text).toBe('Next I will');
    expect(res.usage).toEqual({ inputTokens: 11, outputTokens: 7 });
  });

  it('accepts CRLF line endings', async () => {
    expect((await anthropic(MODEL.anthropicCrlf).complete(HI, [])).text).toBe('ok');
  });

  it('fails loudly when the stream ends before message_stop', async () => {
    await expect(anthropic(MODEL.anthropicNoMessageStop).complete(HI, [])).rejects.toThrow('ended before message_stop');
  });

  it('fails loudly on an error event', async () => {
    await expect(anthropic(MODEL.anthropicStreamError).complete(HI, [])).rejects.toThrow(/stream error.*Overloaded/);
  });

  it('reassembles events split across arbitrary chunk boundaries, including mid-character', async () => {
    const wire = new TextEncoder().encode(
      messageStream({ blocks: [{ type: 'text', text: 'Revenue — €162,000' }, COMPLETE_CALL], stopReason: 'tool_use' }),
    );
    const chunks: Uint8Array[] = [];
    for (let i = 0; i < wire.length; i += 7) chunks.push(wire.slice(i, i + 7));
    const stream = new ReadableStream<Uint8Array>({
      start(controller) {
        for (const chunk of chunks) controller.enqueue(chunk);
        controller.close();
      },
    });
    const res = await readMessageStream(stream);
    expect(res.text).toBe('Revenue — €162,000');
    expect(res.toolCalls).toHaveLength(1);
    expect(res.stopReason).toBe('tool_use');
  });

  it('keeps an unparseable completed tool input visible instead of dropping it', async () => {
    const wire = messageStream({ blocks: [{ type: 'tool_use', id: 'toolu_x', name: 'add_fact', json: '{"factType":' }], stopReason: 'tool_use' });
    const res = await readMessageStream(new Response(wire).body!);
    expect(res.toolCalls[0].input).toEqual({ __unparsedInput: '{"factType":' });
    expect(res.lastToolCallTruncated).toBeUndefined();
  });
});

describe('refusals', () => {
  it('anthropic: reports stop_reason refusal with its stop_details category and explanation', async () => {
    const res = await anthropic(MODEL.anthropicRefusal).complete(HI, []);
    expect(res.stopReason).toBe('refusal');
    expect(res.toolCalls).toEqual([]);
    expect(res.refusal).toEqual({ category: 'reasoning_extraction', explanation: 'Blocked by policy.' });
  });

  it('openai: reports finish_reason content_filter', async () => {
    expect((await openai(MODEL.openaiContentFilter).complete(HI, [])).refusal).toEqual({ category: 'content_filter', explanation: null });
  });

  it('openai: reports a message.refusal', async () => {
    expect((await openai(MODEL.openaiRefusal).complete(HI, [])).refusal).toEqual({ category: null, explanation: "I can't help with that." });
  });

  it('reports no refusal on a normal completion', async () => {
    expect((await anthropic('claude-opus-5-5').complete(HI, [])).refusal).toBeUndefined();
    expect((await openai('gpt-test').complete(HI, [])).refusal).toBeUndefined();
  });
});

describe('output token cap', () => {
  it('anthropic: sends max_tokens 16000 by default', async () => {
    expect((await sentBody(anthropic('claude-opus-5-5'))).max_tokens).toBe(16_000);
  });

  it('anthropic: flags the last tool call when max_tokens cut it off', async () => {
    const res = await anthropic(MODEL.anthropicCutInToolCall).complete(HI, []);
    expect(res.stopReason).toBe('max_tokens');
    expect(res.toolCalls.map((c) => c.id)).toEqual(['toolu_1', 'toolu_2']);
    expect(res.toolCalls[1].input).toEqual({});
    expect(res.hitOutputCap).toBe(true);
    expect(res.lastToolCallTruncated).toBe(true);
  });

  it('anthropic: flags no call when max_tokens cut off text after complete tool calls', async () => {
    const res = await anthropic(MODEL.anthropicCutInText).complete(HI, []);
    expect(res.hitOutputCap).toBe(true);
    expect(res.lastToolCallTruncated).toBeUndefined();
  });

  it('anthropic: reports the cap when it was hit while thinking, before any tool call', async () => {
    const res = await anthropic(MODEL.anthropicCutInThinking).complete(HI, []);
    expect(res.toolCalls).toEqual([]);
    expect(res.hitOutputCap).toBe(true);
    expect(res.lastToolCallTruncated).toBeUndefined();
  });

  it('openai: flags the last tool call on finish_reason length when its arguments do not parse', async () => {
    const res = await openai(MODEL.openaiLengthCut).complete(HI, []);
    expect(res.hitOutputCap).toBe(true);
    expect(res.lastToolCallTruncated).toBe(true);
  });

  it('openai: flags no call on finish_reason length when every call parsed (the server dropped the cut one)', async () => {
    const res = await openai(MODEL.openaiLengthComplete).complete(HI, []);
    expect(res.hitOutputCap).toBe(true);
    expect(res.lastToolCallTruncated).toBeUndefined();
    expect(res.toolCalls[0].input).toEqual({ factType: 'a' });
  });

  it('flags nothing on normal completions, including tool calls without ids', async () => {
    for (const res of [await anthropic('claude-opus-5-5').complete(HI, []), await openai(MODEL.openaiNoIds).complete(HI, [])]) {
      expect(res.hitOutputCap).toBeUndefined();
      expect(res.lastToolCallTruncated).toBeUndefined();
    }
  });
});
