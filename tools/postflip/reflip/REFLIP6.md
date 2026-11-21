# The sixth flip (2026-11-21): the fifth flip's list on main's translator with rule "typed library interfaces"

STATUS: IN PROGRESS (this file is rewritten at the end; until then it says where the work stands).

    overlay branch   main-r13-flip6        worktree $B/wt/flip6     (steps.txt, toolfix.sh 37-39, patches)
    replay worktree  $B/wt/replay6         (detached, per-step commits; the working replay)
    driver           tools/postflip/reflip/replay6.sh [NAME]
    base             64ac1148 (freeze 57031208, tools 91e8a048, rename + gofmt 3c672fd7, erased runtime files removed 64ac1148)

State when this was written: the working replay is at step 553 (262 of 271 commits); gated so far (four
MATCH, runtime tests ok): after 71 commits (step 142), after step 402 (commit 180, an earlier pass without
toolfix 37), after 248 commits (step 494). Notes for steps.txt not yet inserted are in the scratch file
notes6.txt of the session (their content is in the commit messages of this branch).

Known open item: native (step 403) types 259 list declarations as slices (fifth flip 423): it does not
read the views asListT / asCollT / asIterable (239 + 23 + 19 declarations stay for them).
