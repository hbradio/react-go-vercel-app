package handler

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"react-go-vercel-app/pkg/db"
	"react-go-vercel-app/pkg/models"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"
)

func WebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	sig := r.Header.Get("Stripe-Signature")
	secret := os.Getenv("STRIPE_WEBHOOK_SECRET")

	event, err := webhook.ConstructEventWithOptions(body, sig, secret, webhook.ConstructEventOptions{
		IgnoreAPIVersionMismatch: true,
	})
	if err != nil {
		log.Printf("webhook signature error: %v", err)
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	log.Printf("webhook: received %s", event.Type)

	if event.Type == "checkout.session.completed" {
		var session stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			log.Printf("webhook parse error: %v", err)
			http.Error(w, "parse error", http.StatusBadRequest)
			return
		}

		pool, err := db.GetDB()
		if err != nil {
			log.Printf("webhook db error: %v", err)
			http.Error(w, "database error", http.StatusInternalServerError)
			return
		}

		if session.Customer != nil {
			log.Printf("webhook: marking customer %s as purchased", session.Customer.ID)
			models.MarkPurchased(pool, session.Customer.ID)
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"received":true}`))
}
