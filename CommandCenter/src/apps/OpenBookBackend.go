package main

import (
	"fmt"

	"github.com/openbook/shared/models"
)

func main() {
	// Example: Create a new DataStore
	dataStore := models.NewDataStore()

	fmt.Println("Command Center starting...")
	fmt.Printf("DataStore initialized with %d teams\n", len(dataStore.Teams))

	// TODO: Add your logic here to fetch data from the database
	// and run operations on it
}
