package models

import (
	"database/sql"
	"time"
)

type User struct {
	ID               string    `json:"id"`
	Auth0ID          string    `json:"auth0_id"`
	Email            string    `json:"email"`
	StripeCustomerID *string   `json:"stripe_customer_id,omitempty"`
	HasPurchased     bool      `json:"has_purchased"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func GetOrCreateUser(db *sql.DB, auth0ID, email string) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		`SELECT id, auth0_id, email, stripe_customer_id, has_purchased, created_at, updated_at
		 FROM users WHERE auth0_id = $1`, auth0ID,
	).Scan(&u.ID, &u.Auth0ID, &u.Email, &u.StripeCustomerID, &u.HasPurchased, &u.CreatedAt, &u.UpdatedAt)

	if err == sql.ErrNoRows {
		err = db.QueryRow(
			`INSERT INTO users (auth0_id, email) VALUES ($1, $2)
			 RETURNING id, auth0_id, email, stripe_customer_id, has_purchased, created_at, updated_at`,
			auth0ID, email,
		).Scan(&u.ID, &u.Auth0ID, &u.Email, &u.StripeCustomerID, &u.HasPurchased, &u.CreatedAt, &u.UpdatedAt)
		if err != nil {
			return nil, err
		}
		return u, nil
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func SetStripeCustomer(db *sql.DB, auth0ID, customerID string) error {
	_, err := db.Exec(
		`UPDATE users SET stripe_customer_id = $1, updated_at = now() WHERE auth0_id = $2`,
		customerID, auth0ID,
	)
	return err
}

func MarkPurchased(db *sql.DB, stripeCustomerID string) error {
	_, err := db.Exec(
		`UPDATE users SET has_purchased = true, updated_at = now() WHERE stripe_customer_id = $1`,
		stripeCustomerID,
	)
	return err
}
