import { describe, expect, it } from 'vitest';
import { buildSystemPrompt, renderPacketMessage } from '../src/loop.js';
import { createToolRegistry } from '../src/tools.js';
import { fixturePacket, TEST_MODEL } from './helpers.js';

// Prompt v1 asked for "the alternative conclusions you considered … why it was
// set aside" and called an empty list "work not shown". Claude Opus 5.5,
// Sonnet 5.5, and Opus 5 refused 9 of 9 identical requests with
// stop_details.category "reasoning_extraction" (DESIGN §5, prompt history).
const REASONING_EXTRACTION_TRIGGERS = ['you considered', 'set aside', 'work not shown', 'reasoning with'];

function modelFacingText(): string {
  const packet = fixturePacket();
  const tools = createToolRegistry(packet, { modelIdentity: TEST_MODEL }).tools;
  return [
    buildSystemPrompt(),
    renderPacketMessage(packet),
    ...tools.map((t) => `${t.description}\n${JSON.stringify(t.inputSchema)}`),
  ]
    .join('\n')
    .toLowerCase();
}

describe('model-facing prompt surface', () => {
  it.each(REASONING_EXTRACTION_TRIGGERS)('does not ask for %j', (phrase) => {
    expect(modelFacingText()).not.toContain(phrase);
  });

  it('still requires judgments to itemize alternatives', () => {
    const judgment = createToolRegistry(fixturePacket(), { modelIdentity: TEST_MODEL }).tools.find((t) => t.name === 'add_judgment');
    expect(judgment?.inputSchema).toMatchObject({
      properties: { alternatives: { type: 'array', items: { required: ['conclusion'] } } },
    });
    expect(buildSystemPrompt()).toContain('itemized via `alternatives`');
  });
});
