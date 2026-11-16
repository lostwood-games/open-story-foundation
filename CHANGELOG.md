# Changelog

Each release adds to the format without changing what earlier files mean: a project
that `osf` 0.1.0 accepts is still valid and means the same thing.

## 0.2.0

- `Variable` takes an optional `description`, a note for writers and programmers that
  players never see and that is not translated. In `.osf`:
  `Variable gold: UnsignedNumber = 0 { description = "Coins the player carries." }`.

## 0.1.0

- First release: the OSF v1 format, the `.osf` language and the `osf` command.
