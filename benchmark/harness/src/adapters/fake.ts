/**
 * Scripted adapter for tests and dry runs: replays a fixed tool-call
 * sequence, ignoring the conversation. No network, no tokens.
 */
import type { ChatMessage, CompletionResult, ProviderAdapter, ToolSpec } from './types.js';

export interface FakeStep {
  text?: string;
  toolCalls?: Array<{ name: string; input: unknown }>;
}

export class FakeAdapter implements ProviderAdapter {
  readonly provider = 'fake';
  readonly model = 'scripted';
  private stepIndex = 0;
  private callCounter = 0;

  constructor(private readonly steps: FakeStep[]) {}

  async complete(_messages: ChatMessage[], _tools: ToolSpec[]): Promise<CompletionResult> {
    const step = this.steps[this.stepIndex++];
    if (!step) {
      return { text: '(script exhausted)', toolCalls: [], usage: { inputTokens: 0, outputTokens: 0 }, stopReason: 'end_turn' };
    }
    return {
      text: step.text ?? '',
      toolCalls: (step.toolCalls ?? []).map((c) => ({ id: `fake_${++this.callCounter}`, name: c.name, input: c.input })),
      usage: { inputTokens: 0, outputTokens: 0 },
      stopReason: (step.toolCalls?.length ?? 0) > 0 ? 'tool_use' : 'end_turn',
    };
  }
}
