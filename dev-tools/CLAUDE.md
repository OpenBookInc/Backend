# Dev Tools

Development and testing utilities. Not used in production.

## Build Rules

- Never use `go build` to compile Go code. Use `go build ./...` instead, which checks that the code compiles without writing binary files to disk.

## Tools

### set-balance

Sets a user's balance in the database by calling the `set_balance(uuid, bigint)` SQL function directly.

```bash
./run_set_balance.sh
```

Configured via `.env`:
- `SET_BALANCE_USER_ID`: UUID of the user
- `SET_BALANCE_NEW_BALANCE`: integer balance amount (must be non-negative)

## Configuration

Requires a `.env` file in the working directory with database connection variables. See `.env.example`.
