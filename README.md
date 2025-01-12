# backup-home

## Development

### Prerequisites

- [direnv](https://direnv.net/)
- [Nix](https://nixos.org/download.html) with flakes enabled

### Setup

Allow direnv to automatically load the development environment:

```console
direnv allow
```

### Building and Running

#### On Windows
Use the PowerShell script:
```console
# Build the project
.\build.ps1 build

# Run the program
.\build.ps1 run

# Test run with preview
.\build.ps1 dry-run
```

#### On Unix/Linux/macOS
Use Make:
```console
# Build the project
make build

# Run the program
make run

# Test run with preview
make dry-run
```

## Configure project

```console
gh repo create --public backup-home

go mod init backup-home

echo "use flake" > .envrc

echo "\
.direnv
dist/
.DS_Store
" > .gitignore

go mod tidy # Updates go.mod
```

## TODO

- [ ] Add progress reporting
- [ ] Implement proper error handling for various failure scenarios
- [ ] Add retry logic for interrupted operations
- [ ] Add tests
- [ ] Add CI/CD to build all platforms binaries
- [ ] Use libs instead of binaries

## Reasoning

After trying out to implement all that in rust, I decided to try out go, since
I thought rclone has a library and is also written in go. But it turns out that
it's kind of more complex:
https://github.com/rclone/rclone/issues/361#issuecomment-1611890274, but maybe
I will try to do it someday next time.
