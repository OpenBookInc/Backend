# OpenBook Backend

## Build Rules

- Never use `go build` to compile Go code. Use `go build ./...` instead, which checks that the code compiles without writing binary files to disk.
