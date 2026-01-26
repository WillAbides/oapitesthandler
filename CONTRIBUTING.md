# Contributing to oapitesthandler

Your contributions are welcome here. Feel free to open issues and pull requests.
If you have a non-trivial change, you may want to open an issue before spending
much time coding, so we can discuss whether the change will be a good fit for
oapitesthandler. But don't let that stop you from coding. Just be aware that
while all changes are welcome, not all will be merged.

## Releasing

Releases are automated
with [release-train](https://github.com/WillAbides/release-train). All PRs must
have a release label. See the release-train readme for more details.

## Scripts

oapitesthandler uses a number of scripts to automate common tasks. They are found in the
`script` directory.

<!--- start script descriptions --->

### bindown

script/bindown runs bindown with the given arguments

### cibuild

script/cibuild is run by CI to test this project. It can also be run locally.

### fmt

script/fmt formats go code and shell scripts.

### generate

script/generate runs all generators for this repo.
`script/generate --check` checks that the generated files are up to date.

### lint

script/lint runs linters on the project.

### oapitesthandler

script/oapitesthandler builds and runs the project with the given arguments.

### release-hook

script/release-hook is run by release-train as pre-tag-hook

### test

script/test runs tests on the project.

### update-docs

script/generate-readme updates documentation.
- For projects with binaries, it updates the usage output in README.md.
- Adds script descriptions to CONTRIBUTING.md.

<!--- end script descriptions --->
