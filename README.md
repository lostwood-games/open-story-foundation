# Open Story Format (OSF)
[![Go Build, Test and Lint](https://github.com/lostwood-games/open-story-foundation/actions/workflows/go.yml/badge.svg)](https://github.com/lostwood-games/open-story-foundation/actions/workflows/go.yml)
[![Validate YAML](https://github.com/lostwood-games/open-story-foundation/actions/workflows/validate.yml/badge.svg)](https://github.com/lostwood-games/open-story-foundation/actions/workflows/validate.yml)

Open Story Format is a way to keep a game's story as code: dialogues, characters,
documents, variables and translations live in plain text files that sit in git next to
the rest of the game. It is inspired by tools such as Terraform, Bicep and Kubernetes
manifests, where every file is one resource with a kind and a version.

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


## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE). The license covers the specification, the schema, the examples and all
the tools in this repository.

## Contact

Website: <https://openstoryfoundation.com>. Questions: <hello@openstoryfoundation.com>.
