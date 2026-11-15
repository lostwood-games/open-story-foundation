# Changelog

Each release adds to the format without changing what earlier files mean: a project
that `osf` 0.1.0 accepts is still valid and means the same thing.

## 0.3.0

- `next` can branch on variables. A node or option may have several conditional
  `next`s, tried in the order written; the first whose `requires` are all met is
  followed, else the plain `next`, else the dialogue ends. In `.osf`:
  `next: rich requires { gold >= 10 }`, written once per branch, plus at most one plain
  `next:`. In YAML and JSON, `next` takes a list of `{ to, requires }`, with only the
  last entry allowed to leave out `requires` (OSF125).

## 0.2.0

- `Variable` takes an optional `description`, a note for writers and programmers that
  players never see and that is not translated. In `.osf`:
  `Variable gold: UnsignedNumber = 0 { description = "Coins the player carries." }`.

## 0.1.0

- First release: the OSF v1 format, the `.osf` language and the `osf` command.
