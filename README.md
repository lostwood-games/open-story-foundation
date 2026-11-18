# Open Story Format (OSF)
[![Release](https://github.com/lostwood-games/open-story-foundation/actions/workflows/release.yml/badge.svg)](https://github.com/lostwood-games/open-story-foundation/actions/workflows/release.yml)
[![Go Build, Test and Lint](https://github.com/lostwood-games/open-story-foundation/actions/workflows/go.yml/badge.svg)](https://github.com/lostwood-games/open-story-foundation/actions/workflows/go.yml)
[![OSF Validate Check](https://github.com/lostwood-games/open-story-foundation/actions/workflows/validate.yml/badge.svg)](https://github.com/lostwood-games/open-story-foundation/actions/workflows/validate.yml)
[![VS Code extension release](https://github.com/lostwood-games/open-story-foundation/actions/workflows/vscode-release.yml/badge.svg)](https://github.com/lostwood-games/open-story-foundation/actions/workflows/vscode-release.yml)

[![CodeQL](https://github.com/lostwood-games/open-story-foundation/actions/workflows/codeql.yml/badge.svg)](https://github.com/lostwood-games/open-story-foundation/actions/workflows/codeql.yml)



Open Story Format is a way to keep a game's story as code: dialogues, characters,
documents, variables and translations live in plain text files that sit in git next to
the rest of the game. It is inspired by tools such as Terraform, Bicep and Kubernetes
manifests, where every file is one resource with a kind and a version.

OSF started as an internal tool at Lostwood, where we use it to build narrative-driven
games. We decided to open-source it to grow a community around it, so that everyone can
make better games.

A project can be written in two equivalent forms.

As YAML or JSON resources, one per file, which tools and game engines read:

```yaml
apiVersion: dialogue/v1
kind: Dialogue
data:
  id: 01_intro_scene
  startNode: node_01
  nodes:
    node_01:
      content:
        text: Where am I?
```

Or by hand in the OSF language (`.osf` files), which compiles to those resources:

```
Story LDOD {
  name  = "Leviathan: The Last Day of The Decade"
  start = Dialogue.01_intro_scene
}

Variable is_dreaming: Boolean = false
```


## Install

On macOS and Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/lostwood-games/open-story-foundation/main/install.sh | sh
```

On Windows, in PowerShell:

```powershell
irm https://raw.githubusercontent.com/lostwood-games/open-story-foundation/main/install.ps1 | iex
```

Both download the latest release for your system, check its checksum and put `osf` on
your path. You can also download an archive from the
[releases page](https://github.com/lostwood-games/open-story-foundation/releases), or
build from source with Go 1.26 or newer:

```sh
cd src/cli && go build -o ../../osf ./cmd/osf
```

## Usage

```sh
osf validate             # check the project in the current folder
osf convert -o osf       # write a JSON or YAML project as .osf files for writers
osf convert              # write a project as JSON for a game to load
osf fmt                  # put .osf files in the standard formatting
osf play                 # play the story in the terminal
osf play -coverage       # walk every path and report dead ends
```

A project is a folder of `.osf` files, or of JSON or YAML resources. Every command takes
a folder as its last argument and prints its options with `-help`. The format is defined
in [`docs/spec.md`](docs/spec.md), the `.osf` language in
[`docs/language.md`](docs/language.md), and every error code in
[`docs/diagnostics.md`](docs/diagnostics.md).

## Editor support

[`src/vscode`](src/vscode) is the VS Code extension for `.osf` files: syntax
highlighting, comment toggling, bracket matching and two-space indentation. It is
published to the Visual Studio Marketplace as `lostwood.osf`.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE). The license covers the specification, the schema, the examples and all
the tools in this repository.

## Contact

Website: <https://openstoryfoundation.com>. Questions: <hello@openstoryfoundation.com>.
