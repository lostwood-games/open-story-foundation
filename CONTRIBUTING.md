# Contributing

## Questions, ideas and bugs

Open an issue. For a bug, include the smallest project or file that shows it, the
command you ran and what you expected to happen. For a change to the format itself,
describe the story or game situation that the current format cannot express.

## Pull requests

- Keep a pull request to one idea, small enough to review in one sitting.
- Code comes with its tests, and the checks on the pull request must pass.
- A change to the format changes the specification, the schema and the examples in the
  same pull request, so they never disagree.
- For anything larger than a fix, open an issue first so the approach can be agreed
  before the work is done.

## Releasing

Maintainers release by pushing a tag from `main`:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The `Release` workflow tests the code, builds `osf` for Linux, macOS and Windows with
`scripts/dist.sh`, and publishes the archives and `checksums.txt` as a GitHub release
with generated notes. A tag with a `-`, such as `v0.2.0-rc.1`, is published as a
pre-release, so the install scripts keep installing the last full release.

The VS Code extension in `src/vscode` is released on its own, with a tag
`osf-vscode-v<version>` that matches `version` in `src/vscode/package.json`:

```sh
git tag osf-vscode-v0.1.0
git push origin osf-vscode-v0.1.0
```

The `VS Code extension release` workflow tests and packages it, publishes it to the
Visual Studio Marketplace and Open VSX, and attaches the `.vsix` to a GitHub release
that is not marked latest, so the install scripts keep finding `osf`.

## License

By contributing you agree that your contribution is licensed under the
[MIT license](LICENSE) of this repository.
