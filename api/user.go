package handler

import (
	"encoding/json"
	"net/http"

	"react-go-vercel-app/pkg/auth"
	"react-go-vercel-app/pkg/db"
	"react-go-vercel-app/pkg/models"
)

func UserHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	sub, accessToken, err := auth.ValidateRequest(r)
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	pool, err := db.GetDB()
	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	// Fetch email from Auth0 for user creation
	email, _ := auth.FetchUserEmail(accessToken)
	if email == "" {
		email = sub // fallback
	}

	user, err := models.GetOrCreateUser(pool, sub, email)
	if err != nil {
		http.Error(w, `{"error":"failed to get user"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}
