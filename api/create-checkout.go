package handler

import (
	"encoding/json"
	"net/http"
	"os"

	"react-go-vercel-app/pkg/auth"
	"react-go-vercel-app/pkg/db"
	"react-go-vercel-app/pkg/models"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
)

func CreateCheckoutHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	sub, accessToken, err := auth.ValidateRequest(r)
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")

	pool, err := db.GetDB()
	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	// Get or create user
	email, _ := auth.FetchUserEmail(accessToken)
	if email == "" {
		email = sub
	}
	user, err := models.GetOrCreateUser(pool, sub, email)
	if err != nil {
		http.Error(w, `{"error":"failed to get user"}`, http.StatusInternalServerError)
		return
	}

	// Create or reuse Stripe customer
	var customerID string
	if user.StripeCustomerID != nil {
		customerID = *user.StripeCustomerID
	} else {
		c, err := customer.New(&stripe.CustomerParams{
			Email: stripe.String(user.Email),
			Params: stripe.Params{
				Metadata: map[string]string{"auth0_id": sub},
			},
		})
		if err != nil {
			http.Error(w, `{"error":"failed to create customer"}`, http.StatusInternalServerError)
			return
		}
		customerID = c.ID
		models.SetStripeCustomer(pool, sub, customerID)
	}

	// Determine success/cancel URLs
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = r.Header.Get("Referer")
	}
	if origin == "" {
		origin = "http://localhost:5173"
	}

	// Create Checkout Session
	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(os.Getenv("STRIPE_PRICE_ID")),
				Quantity: stripe.Int64(1),
			},
		},
		SuccessURL: stripe.String(origin + "/dashboard?purchased=true"),
		CancelURL:  stripe.String(origin + "/dashboard"),
		Params: stripe.Params{
			Metadata: map[string]string{"auth0_id": sub},
		},
	}

	s, err := session.New(params)
	if err != nil {
		http.Error(w, `{"error":"failed to create checkout session"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": s.URL})
}
