// db.go : used for connecting to the DB (in this case Postgres)

package platform

import (
    "context"
    "database/sql"
    "fmt"
    "log"

    _ "github.com/lib/pq" // Import the PostgreSQL driver (blank import)
)

// ConnectDB establishes a connection to the PostgreSQL database.
func ConnectDB(connectionURL string) (*sql.DB, error) {
    db, err := sql.Open("postgres", connectionURL)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to database: %w", err)
    }

    // Ping the database to ensure the connection is good.
    if err := db.PingContext(context.Background()); err != nil {
        return nil, fmt.Errorf("failed to ping database: %w", err)
    }

    log.Println("Successfully connected to PostgreSQL database.")
    return db, nil

}

// CloseDB closes the database connection.  This is typically called in main function's `defer` statement
func CloseDB(db *sql.DB) {
    if err := db.Close(); err != nil {
        log.Println("Error closing database connection:", err)
    }
}
