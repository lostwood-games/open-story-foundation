# Integration tests for `osf build`

Each folder in `cases/` is one test:

```
cases/<name>/
  project/     an .osf project, the input
  expected/    the JSON files `osf build -o json` must produce from it
```

`run.sh` builds every project into a temporary folder and compares the output with
`expected/` file by file. A missing file, an extra file or any difference in content
fails the test. It also checks that the output passes `osf validate`.

```
(cd src/cli && go build -o ../../osf ./cmd/osf)
bash tests/build/run.sh
```

To add a case, create `cases/<name>/project/` with `.osf` files and write the JSON you
expect into `cases/<name>/expected/`. `UPDATE=1 bash tests/build/run.sh` writes `expected/`
from the current output instead; read the result before committing it, because the
test can only catch what the expected files say.

| Case | What it covers |
| --- | --- |
| `minimal` | A story, one character, one dialogue with two nodes. |
| `variables` | The three variable types, `set` with add and subtract, `requires` with comparisons, text references, a `"""` long text, a jump to another dialogue. |
| `translations` | The i18n plugin: locale settings, an enforced locale and its translation file. |
