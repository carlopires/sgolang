# cli: commands defined at run time (D184)

`cli` is a command-line library whose commands are values. `undisk` is its acceptance example:
the `undisk-cli` shape, where the command tree depends on state read at run time.

- **`cli/`**: the library. It has these parts:
  - **Commands:** `cli.Command` holds `Name`, `Summary`, `Description`, `Examples`, `Group`, `Hidden`, `Version`, `Flags`, `Args`, and a `Runner`.
  - **Tree:** the tree is built and extended at run time with `Add` (which replaces a command of the same name), `Find`, and `Ensure`.
  - **Flags and arguments:** typed as `String`, `Int`, `Bool`, `Duration`, `Strings`, `Ints`. They can be required or have defaults, and a list argument takes the remaining words.
  - **Help and errors:** grouped help, `--help`, and `--version`. Errors name the command.
  - **Handlers:** a handler is an interface (`Run(ctx *sgo.Context, c *cli.Call) error`). There are no function fields and no generics.
- **`daemon/`**: a fake undisk daemon. It serves a profile over the granted socket
  `unix:undiskd.sock`, listing the commands the user's role may run (alice is an admin, bob is a viewer, anyone else is not logged in). It runs those commands against an in-memory store.
- **`undisk/`**: the CLI. It has static commands (`version`, `whoami`, `shell`) and commands built from the profile, which forward their values to the daemon. `shell` reads command lines from stdin and runs them over the same tree.
- **`main.sgo`**: starts the daemon in a nursery and runs one command line. With no arguments, it runs a scripted session and prints the transcript (`expected.txt`).

```
sgo run .                                   # the transcript
sgo build -o undisk . && ./undisk --user alice files ls --long
printf 'files ls\nwhoami\n' | ./undisk --user bob shell
sgo test ./...
sgo lab sweep -seeds 0..63 -expect expected.txt .
```

Floating-point flags wait for D185; durations are integer nanoseconds (`std/time`).
