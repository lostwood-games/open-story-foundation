// Tokenizes every .osf file in the repository with the grammar and fails when a
// token is scoped invalid.* or text matches no rule at all, so a grammar change
// cannot break the examples.
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { dirname, join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';
import vsctm from 'vscode-textmate';
import oniguruma from 'vscode-oniguruma';

const require = createRequire(import.meta.url);
const here = dirname(fileURLToPath(import.meta.url));
const repo = join(here, '..', '..', '..');
const grammarPath = join(here, '..', 'syntaxes', 'osf.tmLanguage.json');

const wasm = readFileSync(require.resolve('vscode-oniguruma/release/onig.wasm'));
await oniguruma.loadWASM(wasm.buffer.slice(wasm.byteOffset, wasm.byteOffset + wasm.byteLength));

const registry = new vsctm.Registry({
  onigLib: Promise.resolve({
    createOnigScanner: (patterns) => new oniguruma.OnigScanner(patterns),
    createOnigString: (s) => new oniguruma.OnigString(s),
  }),
  loadGrammar: async (scopeName) =>
    scopeName === 'source.osf'
      ? vsctm.parseRawGrammar(readFileSync(grammarPath, 'utf8'), grammarPath)
      : null,
});
const grammar = await registry.loadGrammar('source.osf');

function* osfFiles(dir) {
  for (const name of readdirSync(dir)) {
    if (name === 'node_modules' || name.startsWith('.')) continue;
    const path = join(dir, name);
    if (statSync(path).isDirectory()) yield* osfFiles(path);
    else if (name.endsWith('.osf') && !name.endsWith('.test.osf')) yield path;
  }
}

let files = 0;
let problems = 0;
for (const path of osfFiles(repo)) {
  files++;
  let state = vsctm.INITIAL;
  const lines = readFileSync(path, 'utf8').split(/\r?\n/);
  lines.forEach((line, i) => {
    const { tokens, ruleStack } = grammar.tokenizeLine(line, state);
    state = ruleStack;
    for (const t of tokens) {
      const text = line.slice(t.startIndex, t.endIndex);
      const invalid = t.scopes.some((s) => s.startsWith('invalid.'));
      const unmatched = t.scopes.length === 1 && text.trim() !== '';
      if (invalid || unmatched) {
        problems++;
        const why = invalid ? t.scopes.at(-1) : 'matched by no rule';
        console.error(`${relative(repo, path)}:${i + 1}:${t.startIndex + 1}: ${JSON.stringify(text)} is ${why}`);
      }
    }
  });
}

if (files === 0) {
  console.error('no .osf files found');
  process.exit(1);
}
console.log(`${files} .osf files tokenized, ${problems} problems`);
process.exit(problems === 0 ? 0 : 1);
