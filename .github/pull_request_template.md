## What and why

<!-- What changes for a user of the viewer, the desktop app or the library, and why. -->

## Checks

- [ ] Test first: a test that failed before this change (CONTRIBUTING.md, TDD)
- [ ] `make lint test` pass; `make e2e` for screens, `make e2e-visual` if the look changed
- [ ] The library still needs only `php` and `ext-json`, and runs on PHP 7.4
- [ ] Log format v2 and API v1 stay backward compatible (spec/README.md), or the break is explained
- [ ] Docs in English and Russian (README.md, README.ru.md) when behavior changes
- [ ] Commit messages follow Conventional Commits: they make the release notes
