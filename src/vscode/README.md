# Open Story Format for VS Code

Syntax highlighting and editing support for `.osf` files, the
[Open Story Format](https://openstoryfoundation.com) language for writing branching
dialogue by hand.

```osf
Dialogue 01_intro_scene {
  Node node_03 {
    speaker: protagonist
    text: "Am I dreaming?"

    Option yes {
      text: "Yes, you are."
      set: { is_dreaming: true }
      next: node_04
    }
  }
}
```

## Features

- Highlighting for the whole language as of OSF 0.6.0: `Story`, `Character`, `Document`,
  `Variable`, `Dialogue`, `Node`, `Option`, `I18n` and `Locale` blocks, `{{ }}` text
  references, `"""` long text with `//` writer's notes, `set` and `requires` maps,
  `or` alternatives, conditional `next`, play-history conditions and
  `next: <node>.options`.
- `//` and `/* */` comment toggling, bracket matching and folding, auto-closing braces
  and quotes.
- Two-space indentation by default, the layout `osf fmt` writes.

Checking, formatting and playing a project is done by the `osf` command; see the
[repository](https://github.com/lostwood-games/open-story-foundation) for how to install
it. The language is described in
[docs/language.md](https://github.com/lostwood-games/open-story-foundation/blob/main/docs/language.md).

## Developing

```sh
cd src/vscode
npm ci
npm test          # grammar assertions, then every .osf file in the repository
npm run package   # builds osf.vsix
```

`test/grammar.test.osf` holds the scope assertions
([vscode-tmgrammar-test](https://github.com/PanAeon/vscode-tmgrammar-test) format).
`test/examples.mjs` tokenizes every `.osf` file in the repository and fails on any
`invalid.*` token or text no rule matches.

To try the extension, open `src/vscode` in VS Code and press F5, or install the package
with `code --install-extension osf.vsix`.

## Releasing

Bump `version` in `package.json` and add it to `CHANGELOG.md`, then push a tag
`osf-vscode-v<version>` from `main`:

```sh
git tag osf-vscode-v1.0.0
git push origin osf-vscode-v1.0.0
```

The `VS Code extension release` workflow tests and packages the extension, publishes it
to the Visual Studio Marketplace, and attaches the `.vsix` to a GitHub
release. It publishes with trusted publishing (GitHub OIDC), so no tokens are stored.
