package db

import (
	"database/sql"
	"log"
	"os"
	"sync"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	pool    *sql.DB
	once    sync.Once
	initErr error
)

const migration = `
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    auth0_id TEXT UNIQUE NOT NULL,
    email TEXT NOT NULL,
    stripe_customer_id TEXT,
    has_purchased BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);
`

func GetDB() (*sql.DB, error) {
	once.Do(func() {
		pool, initErr = sql.Open("pgx", os.Getenv("DATABASE_URL"))
		if initErr != nil {
			return
		}
		pool.SetMaxOpenConns(5)
		pool.SetMaxIdleConns(2)

		if _, err := pool.Exec(migration); err != nil {
			log.Printf("migration warning: %v", err)
		}
	})
	return pool, initErr
}
