# Integration tests for `osf convert`

Each test is a folder under `cases/to-<format>/`:

```
cases/to-json/<name>/
  input/       a project in any form (.osf, JSON or YAML)
  expected/    the files `osf convert -o json` must produce from it
```

`to-osf/` and `to-yaml/` work the same way for `-o osf` and `-o yaml`.

`run.sh` converts every `input/` into a temporary folder and compares the output with
`expected/` file by file. A missing file, an extra file or any difference in content
fails the test. It also checks that the output passes `osf validate`.

```
(cd src/cli && go build -o ../../osf ./cmd/osf)
bash tests/convert/run.sh
```

To add a case, create `cases/to-<format>/<name>/input/` and write the files you expect
into `expected/`. `UPDATE=1 bash tests/convert/run.sh` writes `expected/` from the
current output instead; read the result before committing it, because the test can only
catch what the expected files say.

| Case | What it covers |
| --- | --- |
| `minimal` | A story, one character, one dialogue with two nodes. |
| `variables` | The three variable types, `set` with add and subtract, `requires` with comparisons, text references, a `"""` long text, a jump to another dialogue. |
| `translations` | The i18n plugin: locale settings, an enforced locale and its translation file. |

The `to-osf` cases use the `expected/` JSON of the `to-json` cases as their input, so
together they show a project going from `.osf` to JSON and back. The `.osf` that comes
back has the standard file layout (`versions.osf`, `story.osf`, `characters.osf`, ...),
because JSON does not record which file a resource was written in.
