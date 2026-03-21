.
├── cli/ # flag parsing, subcommand routing
├── main.go # thin entrypoint: cli.Run() -> os.Exit()
├── pkg/ # reusable logical parts (tmux, cpu)
└── run/ # command implementations
