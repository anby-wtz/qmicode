# Release Pre-flight Checklist

- [ ] `git status --porcelain` is empty (clean tree)
- [ ] On default branch (`main` / `master`)
- [ ] All tests green: `go test ./...`
- [ ] CHANGELOG.md has an "Unreleased" section describing the changes since last tag
- [ ] Last release tag exists locally so diff range is valid

If any item fails, stop and tell the user exactly which one failed.
