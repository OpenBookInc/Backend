# shared/models CLAUDE.md

This document provides guidance for working with the models package and its thread-safe registry pattern.

## Overview

The `shared/models` package defines data structures representing database entities and provides a thread-safe singleton registry for managing model instances with pointer-based relationships.

## Thread-Safe Model Registry

### Purpose

The registry ensures:
- **Single instance per entity**: Each database entity (by ID) has exactly one instance in memory
- **Pointer-based relationships**: Parent structs hold pointers to child structs, enabling shared references
- **Thread-safe access**: All operations are protected by `sync.RWMutex` for future concurrent use
- **Automatic caching**: Store methods check the registry before querying the database

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     models.Registry (global)                     │
│                                                                  │
│  ┌──────────────┐    ┌──────────────────────────────────────┐   │
│  │   Slices     │    │         Maps (ID → pointer)           │   │
│  │ (own data)   │    │                                       │   │
│  │              │    │  leaguesByID:     map[int]*League     │   │
│  │  leagues     │───►│  conferencesByID: map[int]*Conference │   │
│  │  conferences │    │  divisionsByID:   map[int]*Division   │   │
│  │  divisions   │    │  teamsByID:       map[int]*Team       │   │
│  │  teams       │    │  individualsByID: map[int]*Individual │   │
│  │  ...         │    │  gamesByID:       map[int]*Game       │   │
│  └──────────────┘    │  ...                                  │   │
│                      └──────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

**Why slices + maps?**
- Slices provide stable storage (appending doesn't invalidate existing pointers)
- Maps provide O(1) lookup by database ID
- Map values are pointers into the slice elements

### Usage Pattern

Store methods follow this pattern:

```go
func (s *Store) GetTeamByID(ctx context.Context, id int) (*models.Team, error) {
    // 1. Check registry first
    if team := models.Registry.GetTeam(id); team != nil {
        return team, nil
    }

    // 2. Query database
    var team models.Team
    err := s.pool.QueryRow(ctx, query, id).Scan(&team.ID, &team.DivisionID, ...)
    if err != nil {
        return nil, err
    }

    // 2.5. Resolve nested struct pointers (recursive)
    division, err := s.GetDivisionByID(ctx, int(team.DivisionID))
    if err != nil {
        return nil, err
    }
    team.Division = division

    // 3. Register and return pointer to registry-managed instance
    return models.Registry.RegisterTeam(&team), nil
}
```

### Key Points

1. **Registry lookup first**: Always check the registry before querying the database
2. **Recursive resolution**: Resolve nested struct pointers before registering the parent
3. **Return registered pointer**: The `Register*` method returns a pointer to the registry's copy
4. **Automatic deduplication**: If an entity is already registered, `Register*` returns the existing pointer

## Model Hierarchy

```
League
  ↓
Conference (has *League)
  ↓
Division (has *Conference)
  ↓
Team (has *Division)

Individual (has *League)

Game (has *Team for TeamA and TeamB)

Roster (has *Team and []*Individual)

IndividualStatus (has *Individual)
```

## Sport-Specific Registries

Sport-specific models (NBAStats, NFLStats) have their own registries to avoid circular imports:

- `models/nba.Registry` - for NBAStats
- `models/nfl.Registry` - for NFLStats

These follow the same pattern as the base registry.

## Clearing the Registry

For testing or resetting state:

```go
models.Registry.Clear()
models_nba.Registry.Clear()
models_nfl.Registry.Clear()
```

## Thread Safety

The registry uses a single `sync.RWMutex`:
- `Get*` methods use `RLock()` (multiple readers allowed)
- `Register*` and `Clear()` methods use `Lock()` (exclusive access)

## Best Practices

1. **Always use store methods**: Don't create model instances directly; let store methods handle registration
2. **Don't modify registered entities**: Treat registered entities as immutable to avoid data races
3. **Clear in tests**: Call `Clear()` in test setup/teardown for isolation
4. **Check nil pointers**: Nested struct pointers may be nil if resolution failed
