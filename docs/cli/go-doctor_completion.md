## go-doctor completion

Show shell completion install instructions.

### Synopsis

Show concise installation instructions for shell completions. Run without a shell to see every supported shell, or use 'go-doctor completion script <shell>' to print the raw script.

```
go-doctor completion [bash|zsh|fish|powershell] [flags]
```

### Examples

```
go-doctor completion
go-doctor completion zsh
go-doctor completion script zsh > ~/.zsh/completions/_go-doctor
```

### Options

```
  -h, --help   help for completion
```

### SEE ALSO

* [go-doctor](go-doctor.md)	 - Check Go repositories for code health issues.
* [go-doctor completion script](go-doctor_completion_script.md)	 - Print the raw shell completion script.

