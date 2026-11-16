# Play transcripts

Each `.txt` file here is one run of `osf play` on [`example/osf`](../../example/osf). The
first line is the command without the project folder, such as
`# osf play -locale Rus -choices 2` or `# osf play -coverage`, and the rest is the exact
output. `go test ./...` in `src/cli` replays every file and fails when the output
differs.

To add a transcript, create a file that holds only the command line, then run:

```sh
cd src/cli && go test ./cmd/osf -update
```

The same command rewrites every transcript after an intended change to the output.

## Transcript format

```
== Dialogue 01_intro_scene          # printed whenever a dialogue is entered
...: Where am I?                    # title or speaker name, then the text
Total Darkness surrounds you.       # narration: no speaker
You: What is it all about?
Am I dreaming?                      # a multi-line text continues on plain lines
  1. Yes, you are.                  # options, 1-based
  2. No. Fool
> 1                                 # the choice taken
  [set is_dreaming = true]          # the option's set (a node's set is printed the same way)
Strange
== End
```

When the choices run out with options pending, the options are printed followed by
`(stopped: 2 options pending)`.

## Coverage format

`osf play -coverage` explores every reachable state (dialogue, node, variable values)
from the start dialogue instead of playing, and prints:

```
== Coverage from Dialogue 01_intro_scene
01_intro_scene: 4/4 nodes reached
02_shadows_speak: 1/2 nodes reached (not reached: node_02)
03_epilogue: not reached
6 states, 2 endings
dead end: 01_intro_scene.node_03 has no available option when is_dreaming = true
unreachable option: 01_intro_scene.node_03.node_03_option_01 (requires is_dreaming = true)
loop with no exit: 02_shadows_speak.node_01, 02_shadows_speak.node_02
== 3 findings
```

- A **dead end** is a node with options where, in some reachable variable state, none is
  available.
- An **unreachable option** is one whose `requires` is never met on any reachable path to
  its node.
- A **loop with no exit** is a set of reachable states the play cannot leave to reach an
  ending.

The last line is `== No findings` or the count. The exit status is 1 when there are
findings.
