package main

import (
	"log"


	"chess-ws-go/internal/config"
	"chess-ws-go/internal/platform"

)

func main ()  {
	config, err := config.LoadConfig()

	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}


    // Platform initialization (Database connection)
    db, err := platform.ConnectDB(config.DatabaseURL)
    if err != nil {
        log.Fatal("Error connecting to database:", err)
    }
    defer db.Close() // Close the database connection when the server exits

}