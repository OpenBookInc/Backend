# Sports Data Population Scripts

This Go application fetches NBA and NFL team, player, and roster data from the Sportradar API and stores it in an in-memory data structure.

## Features

- Fetches NBA and NFL team hierarchies (conferences, divisions)
- Retrieves detailed rosters for each team
- Stores data in a well-structured in-memory data store
- Generic data models that work across multiple sports
- Clean architecture with separation of concerns

## Project Structure

```
PopulationScripts/
├── main.go              # Application entry point
├── config/
│   └── config.go        # Configuration management
├── models/
│   └── models.go        # Data models (Team, Individual, Roster, DataStore)
├── client/
│   └── client.go        # Sportradar API client
├── fetcher/
│   ├── nba.go          # NBA data fetching logic
│   ├── nfl.go          # NFL data fetching logic
│   └── utils.go        # Utility functions
├── go.mod              # Go module definition
└── README.md           # This file
```

## Prerequisites

- Go 1.21 or higher
- Sportradar API key (get one at https://developer.sportradar.com)

## Installation

1. Navigate to the project directory:
```bash
cd "/Users/emersonboyd/Documents/source/OpenBook Source/Database/PopulationScripts"
```

2. Install dependencies:
```bash
go mod download
```

3. Copy `.env.example` to `.env` and configure your settings:
```bash
cp .env.example .env
# Then edit .env and set your Sportradar API key
```

## Configuration

The application automatically loads configuration from a `.env` file if present. You can also specify a custom environment file using the `--env` flag.

### Configuration Variables

| Variable | Required | Description | Default |
|----------|----------|-------------|---------|
| `SPORTRADAR_API_KEY` | **Yes** | Your Sportradar API key | None |
| `RATE_LIMIT_DELAY_MS` | No | Delay between API requests in milliseconds | `1000` |

**Note**: The application fetches current roster data from the Sportradar API. Historical season data is not supported by the Team Profile/Roster endpoints used.

### Configuration Methods

1. **Using `.env` file (Recommended)**
   ```bash
   # .env file is automatically loaded
   go run main.go
   ```

2. **Using a custom environment file**
   ```bash
   go run main.go --env=.env.production
   go run main.go --env=/path/to/custom.env
   ```

3. **Using environment variables directly**
   ```bash
   export SPORTRADAR_API_KEY=your_api_key_here
   go run main.go
   ```

4. **Using inline environment variables**
   ```bash
   SPORTRADAR_API_KEY=your_api_key_here go run main.go
   ```

**Note**: Environment variables set in your shell take precedence over values in `.env` files.

## Usage

Run the application:
```bash
go run main.go
```

The application will:
1. Fetch all NBA teams and their rosters
2. Fetch all NFL teams and their rosters
3. Store the data in an in-memory data store
4. Print a summary of the fetched data

## Data Models

### Team
- Generic team model for all sports
- Includes: ID, Name, Market, Alias, Sport, Conference, Division, Venue

### Individual
- Generic player model for all sports
- Includes: ID, Name, Sport, Position, Jersey Number, Height, Weight, Birth Date, Status

### Roster
- Links teams to their players
- Includes: Team ID, Sport, Season, List of Players

### DataStore
- In-memory storage for all data
- Provides methods to query data by sport
- Uses maps for efficient lookups

## API Endpoints Used

### NBA
- League Hierarchy: `/nba/trial/v8/en/league/hierarchy.json`
- Team Profile: `/nba/trial/v8/en/teams/{teamID}/profile.json`

### NFL
- League Hierarchy: `/nfl/official/trial/v7/en/league/hierarchy.json`
- Team Roster: `/nfl/official/trial/v7/en/teams/{teamID}/full_roster.json`

## Code Design Principles

1. **Separation of Concerns**: Client, fetcher, and models are in separate packages
2. **Generic Models**: Same data structures work for all sports
3. **Error Handling**: Graceful error handling with informative messages
4. **Rate Limiting**: Built-in delays to respect API rate limits
5. **Extensibility**: Easy to add more sports or data sources

## Extending the Application

To add support for another sport:

1. Create a new fetcher file (e.g., `fetcher/mlb.go`)
2. Implement fetch functions similar to NBA/NFL
3. Use the same generic models (Team, Individual, Roster)
4. Add the fetch call in `main.go`

## Notes

- The application uses trial API endpoints. Update to production endpoints as needed.
- Rate limiting is implemented with 500ms delays between team roster requests
- All data is stored in memory and will be lost when the application exits
- API responses are parsed to fit the generic data model structure

## Dependencies

- `github.com/go-resty/resty/v2` - HTTP client for API requests
- `github.com/joho/godotenv` - Load environment variables from .env files

## License

Proprietary - OpenBook Source
