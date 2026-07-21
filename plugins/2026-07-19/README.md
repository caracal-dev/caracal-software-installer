# Dev Agent handoff — plugins

Date: 2026-07-19
Generated: 2026-07-19T00:22:31Z

## Where to look

- `all-changes.patch` — combined diff of all uncommitted changes across both repos
- `caracal-software-installer-dev/` — copy of files with uncommitted changes

## What to do

1. Read `all-changes.patch` to see the proposed changes
2. Verify the changes look correct
3. Tell the dev agent what to do next:
   - `commit` — apply the local changes as a git commit
   - `fix X` — make a specific change
   - `discard` — throw away the changes

**The agent will not commit or push without your explicit instruction.**
