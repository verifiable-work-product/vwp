import * as path from 'node:path';
import { fileURLToPath } from 'node:url';
import { loadLocalPacket, type Packet } from '../src/packet.js';
import type { ModelIdentity } from '../src/draft.js';

export const FIXTURE_PACKET_DIR = path.join(path.dirname(fileURLToPath(import.meta.url)), 'fixtures', 'packet');

export function fixturePacket(): Packet {
  return loadLocalPacket(FIXTURE_PACKET_DIR);
}

export const TEST_MODEL: ModelIdentity = { provider: 'fake', name: 'scripted-test', version: '0' };
