/**
 * Scripted adapter for tests and dry runs: replays a fixed tool-call
 * sequence, ignoring the conversation. No network, no tokens.
 */
import type { ChatMessage, CompletionResult, ProviderAdapter, Refusal, ToolSpec } from './types.js';

export interface FakeStep {
  text?: string;
  toolCalls?: Array<{ name: string; input: unknown }>;
  /** Script a provider refusal for this turn. */
  refusal?: Refusal;
  /** Script a response cut off at the token cap (inside its last tool call, if any). */
  truncated?: boolean;
}

export class FakeAdapter implements ProviderAdapter {
  readonly provider = 'fake';
  readonly model = 'scripted';
  readonly temperature: number | null = 0;
  private stepIndex = 0;
  private callCounter = 0;

  constructor(private readonly steps: FakeStep[]) {}

  async complete(_messages: ChatMessage[], _tools: ToolSpec[]): Promise<CompletionResult> {
    const step = this.steps[this.stepIndex++];
    if (!step) {
      return { text: '(script exhausted)', toolCalls: [], usage: { inputTokens: 0, outputTokens: 0 }, stopReason: 'end_turn' };
    }
    if (step.refusal) {
      return { text: '', toolCalls: [], usage: { inputTokens: 0, outputTokens: 0 }, stopReason: 'refusal', refusal: step.refusal };
    }
    const toolCalls = (step.toolCalls ?? []).map((c) => ({ id: `fake_${++this.callCounter}`, name: c.name, input: c.input }));
    if (step.truncated) {
      return {
        text: step.text ?? '',
        toolCalls,
        usage: { inputTokens: 0, outputTokens: 0 },
        stopReason: 'max_tokens',
        hitOutputCap: true,
        lastToolCallTruncated: toolCalls.length > 0,
      };
    }
    return {
      text: step.text ?? '',
      toolCalls,
      usage: { inputTokens: 0, outputTokens: 0 },
      stopReason: toolCalls.length > 0 ? 'tool_use' : 'end_turn',
    };
  }
}
