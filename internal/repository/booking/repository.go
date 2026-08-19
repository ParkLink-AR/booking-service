package booking

import "database/sql"

// Repository provides booking data access against a PostgreSQL database.
type Repository struct {
	database *sql.DB
}

func NewRepository(database *sql.DB) *Repository {
	return &Repository{database: database}
}
