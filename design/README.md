# Design

How declscope is built, and why. The user-facing contract is in [the README](../README.md), and the formal properties are in [the specs](../spec/README.md).

| File | What it holds | Read it |
| --- | --- | --- |
| [`implementation.md`](implementation.md) | The design decisions and constraints, with the evidence and the rejected alternatives behind each | Before changing a rule, a fix, configuration, `shrink`, or their tests |
| [`analyzer_en.mermaid`](analyzer_en.mermaid), [`analyzer_ja.mermaid`](analyzer_ja.mermaid) | A flowchart of one package through the analyzer, from config resolution to the last diagnostic | To find which stage decides something before reading its code |
| [`shrink_en.mermaid`](shrink_en.mermaid), [`shrink_ja.mermaid`](shrink_ja.mermaid) | A flowchart of one `declscope shrink` run, from loading the module to the exit status | The same, for `shrink` |

The flowcharts are drawn by hand from the code. A change to a stage they draw has to change them too, in both languages.

To view a flowchart, paste it into a fenced `mermaid` block on GitHub, or open it in [Mermaid Live Editor](https://mermaid.live).
