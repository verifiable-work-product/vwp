import { describe, expect, it } from 'vitest';
import Ajv2020 from 'ajv/dist/2020';
import * as fs from 'fs';
import * as path from 'path';

const schema = JSON.parse(
  fs.readFileSync(path.resolve(__dirname, '..', '..', 'schemas', 'v0.2', 'presentation-map.schema.json'), 'utf8')
);
const validate = new Ajv2020({ strict: false }).compile(schema);

function validMap(): any {
  return {
    presentationMapVersion: '0.1',
    links: [
      {
        objectType: 'fact',
        objectId: 'f_nyc_revenue',
        artifactPath: 'artifacts/nyc-revenue-allocation.xlsx',
        anchors: [
          {
            cells: { sheet: 'Allocation', range: 'B14' },
            quote: 'New York City revenue',
          },
        ],
      },
    ],
  };
}

describe('proposed v0.2 presentation-map schema', () => {
  it('accepts a structured-object link into an embedded artifact', () => {
    expect(validate(validMap())).toBe(true);
  });

  it.each(['../evil.xlsx', 'artifacts/../evil.xlsx', '/tmp/evil.xlsx', 'C:\\evil.xlsx'])(
    'rejects unsafe artifact path %s',
    (artifactPath) => {
      const value = validMap();
      value.links[0].artifactPath = artifactPath;
      expect(validate(value)).toBe(false);
    }
  );

  it('requires at least one non-empty presentation anchor', () => {
    const value = validMap();
    value.links[0].anchors = [{}];
    expect(validate(value)).toBe(false);
  });
});
