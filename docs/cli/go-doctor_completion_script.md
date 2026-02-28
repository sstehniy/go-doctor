## go-doctor completion script

Print the raw shell completion script.

### Synopsis

Print the raw shell completion script for a specific shell. This is intended for redirection into a completion file or sourcing in your shell startup flow.

```
go-doctor completion script <bash|zsh|fish|powershell> [flags]
```

### Examples

```
go-doctor completion script bash
go-doctor completion script zsh > ~/.zsh/completions/_go-doctor
```

### Options

```
  -h, --help   help for script
```

### SEE ALSO

* [go-doctor completion](go-doctor_completion.md)	 - Show shell completion install instructions.

