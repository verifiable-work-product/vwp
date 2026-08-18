import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const html = fs.readFileSync(new URL('./public/index.html', import.meta.url), 'utf8');
const scriptMatch = html.match(/<script>([\s\S]*)<\/script>/);
assert(scriptMatch, 'index.html must contain its self-contained script');
const script = scriptMatch[1];

// A syntax regression in the no-build frontend would otherwise only appear
// after deployment.
new Function(script);

const rendererStart = script.indexOf('function renderMarkdown');
const rendererEnd = script.indexOf('async function downloadReadableCopy');
const escapeStart = script.indexOf('function escapeHtml');
assert(rendererStart >= 0 && rendererEnd > rendererStart && escapeStart >= 0);

const context = {};
vm.createContext(context);
vm.runInContext(
  `${script.slice(rendererStart, rendererEnd)}\n${script.slice(escapeStart)}\n` +
  'globalThis.renderForTest = renderMarkdown;',
  context
);

const rendered = context.renderForTest(`# Sample work product

> **Review required.** Derived rendering.

| Place | Revenue |
|---|---:|
| New York City | 125000 |

- Evidence-backed
- Recomputable

[Methodology](https://example.com/method?a=1&b=2)
`);
assert.match(rendered, /<h1>Sample work product<\/h1>/);
assert.match(rendered, /<blockquote>/);
assert.match(rendered, /<table>/);
assert.match(rendered, /<ul>/);
assert.match(rendered, /href="https:\/\/example\.com\/method\?a=1&amp;b=2"/);
assert(!rendered.includes('&amp;amp;'), 'safe links must not be double-escaped');

const hostile = context.renderForTest(`
<script>alert('executed')</script>

[bad](javascript:alert(1))

<img src=x onerror=alert(1)>
`);
assert(!hostile.includes('<script>'));
assert(!hostile.includes('<img'));
assert(!hostile.includes('href="javascript:'));
assert(hostile.includes('&lt;script&gt;'));

assert.match(script, /default-src 'none'/, 'readable copies need a restrictive CSP');
assert.match(script, /crypto\.subtle\.digest\('SHA-256'/, 'readable copies must identify their source bytes');
assert.equal((html.match(/id="demoTicks"/g) || []).length, 1);

console.log('frontend reader tests passed');
