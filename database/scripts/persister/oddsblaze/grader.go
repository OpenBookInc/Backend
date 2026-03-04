package oddsblaze

import (
	"context"
	"fmt"

	fetcher_oddsblaze "github.com/openbook/population-scripts/fetcher/oddsblaze"
	"github.com/openbook/population-scripts/store"
)

// mapGraderResultToDB maps the OddsBlaze grader result string to the database enum value.
func mapGraderResultToDB(result string) (string, error) {
	switch result {
	case "Win":
		return "Win", nil
	case "Lose":
		return "Loss", nil
	case "Push":
		return "", fmt.Errorf("received Push result — Push is not a supported market outcome")
	default:
		return "", fmt.Errorf("unknown grader result: %q", result)
	}
}

// mapGraderMessageToDB maps known OddsBlaze grader error messages to database values.
// Returns the mapped result and true if the message was handled, or empty string and false
// if the message is unrecognized and should be treated as an error.
func mapGraderMessageToDB(message string) (string, bool) {
	switch message {
	case "Player not found":
		return "Void", true
	default:
		return "", false
	}
}

// PersistMarketOutcome maps a grader response result and inserts it into the database.
// We use the caller-provided oddsBlazeID rather than graderResp.ID because the grader API
// returns an empty id field in error cases (e.g. "Player not found"), which would cause
// duplicate key violations on insert.
func PersistMarketOutcome(ctx context.Context, dbStore *store.Store, oddsBlazeID string, graderResp *fetcher_oddsblaze.GraderResponse) error {
	// Handle grader error messages before checking the result field
	if graderResp.Message != "" {
		dbResult, ok := mapGraderMessageToDB(graderResp.Message)
		if !ok {
			return fmt.Errorf("unhandled grader endpoint message %q for id %s", graderResp.Message, oddsBlazeID)
		}
		return dbStore.InsertOddsBlazeMarketOutcome(ctx, &store.OddsBlazeMarketOutcomeForInsert{
			OddsBlazeID: oddsBlazeID,
			Result:      dbResult,
		})
	}

	dbResult, err := mapGraderResultToDB(graderResp.Result)
	if err != nil {
		return fmt.Errorf("failed to map grader result for id %s: %w", oddsBlazeID, err)
	}

	err = dbStore.InsertOddsBlazeMarketOutcome(ctx, &store.OddsBlazeMarketOutcomeForInsert{
		OddsBlazeID: oddsBlazeID,
		Result:      dbResult,
	})
	if err != nil {
		return fmt.Errorf("failed to persist market outcome for id %s: %w", oddsBlazeID, err)
	}

	return nil
}
