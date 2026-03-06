# Database Migrations

## Standards

Before creating or modifying migration scripts, always review the existing scripts in the `scripts/` folder to understand and follow the established conventions for naming, formatting, SQL style, and structure.

## Creating Migrations

Use the `migrate.sh` script to create new migration files:

```bash
./migrate.sh create <migration_name>
```

This runs `goose create <migration_name> sql`, which generates a timestamped SQL file in the `scripts/` folder (e.g., `scripts/20260306181339_add_balances_table.sql`).

## Allowed Commands

Only the following `migrate.sh` commands may be run by Claude:

- `./migrate.sh create <name>` — Create a new migration file
- `./migrate.sh status` — Show migration status
- `./migrate.sh version` — Show current migration version

**Do NOT run** `./migrate.sh up`, `down`, `redo`, `reset`, or any other commands that modify the database. Those must be run manually by the developer.
