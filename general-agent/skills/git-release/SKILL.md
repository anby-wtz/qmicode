---
name: git-release
description: Cut a semantic-version release on this repo - verify clean tree, bump version, tag, and publish notes. Use when the user asks to "make/release/cut a release" or "tag vX.Y.Z".
license: MIT
references:
  - checklists
---

# Git Release Skill

Cut a release with these steps, in order:

1. **Pre-flight** (see `references/checklist.md`):
   - Working tree must be clean; branch is the default branch.
   - Tests pass (`go test ./...` for Go projects).
2. Decide the bump type from the changelog since last tag: breaking -> major, feature -> minor, fix/other -> patch.
3. Update version strings in manifest files (e.g., `go.mod`, `package.json`) if the project tracks a version there.
4. Commit version bumps, create an annotated tag `vX.Y.Z` with release notes as the tag message.
5. Push branch and tag; never force-push a release commit or delete the tag afterwards.

Rules:
- Never rewrite history between two existing tags.
- If anything in pre-flight fails, stop and report what failed instead of proceeding.
