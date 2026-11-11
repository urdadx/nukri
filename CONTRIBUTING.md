# Contributing to Nukri

Thanks for helping out. Nukri is a terminal file manager for Linux, written in Go with
[Bubble Tea](https://github.com/charmbracelet/bubbletea) and
[Lip Gloss](https://github.com/charmbracelet/lipgloss).

## Getting started

You need Go (the version in [`go.mod`](go.mod)) on Linux. Nukri targets Linux only for
now, so changes for macOS or Windows are out of scope.

```bash
git clone https://github.com/urdadx/nukri.git
cd nukri
go run ./cmd/nukri
```

To try your build from any folder, `go install ./cmd/nukri` puts `nukri` in
`~/go/bin`.

## Before you open a pull request

CI runs these on every pull request, so run them first:

```bash
gofmt -l .
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@2025.1.1 ./...
go test -race ./...
```

`gofmt -l .` should print nothing.

## Tests

- Put tests in a `test` folder next to the package they test, such as
  `internal/theme/test` for `internal/theme`. Create the folder if it doesn't exist.
- Test logic and behavior. Don't write tests for how things look: Lip Gloss styling
  and layout rendering aren't tested.
- Tests must not touch your real files. Use `t.TempDir()`, and point `HOME` and
  `XDG_DATA_HOME` at temporary folders when the code reads them.
- A test that needs an external tool, such as `ffmpeg`, should skip when the tool isn't
  installed.

## Previews

Nukri doesn't bundle decoders for heavy formats. A preview calls a tool the user
already has, and when that tool is missing the preview pane names what to install.
Keep new previews that way.

## Commits and pull requests

- Keep each pull request to one change.
- Write commit subjects in lowercase, saying what the change does, like
  `toggle the preview pane with P`. Use the body to explain why.
- Describe what you changed and how you tested it in the pull request.

## License

By contributing, you agree that your contributions are licensed under the
[MIT License](LICENSE).
