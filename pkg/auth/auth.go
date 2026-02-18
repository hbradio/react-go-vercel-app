package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/auth0/go-jwt-middleware/v2/jwks"
	"github.com/auth0/go-jwt-middleware/v2/validator"
)

var (
	jwtValidator *validator.Validator
	once         sync.Once
	initErr      error
)

func getValidator() (*validator.Validator, error) {
	once.Do(func() {
		domain := os.Getenv("AUTH0_DOMAIN")
		audience := os.Getenv("AUTH0_AUDIENCE")

		issuerURL, e := url.Parse("https://" + domain + "/")
		if e != nil {
			initErr = e
			return
		}

		provider := jwks.NewCachingProvider(issuerURL, 5*time.Minute)

		jwtValidator, initErr = validator.New(
			provider.KeyFunc,
			validator.RS256,
			issuerURL.String(),
			[]string{audience},
		)
	})
	return jwtValidator, initErr
}

// ValidateRequest extracts and validates the Bearer token from the request.
// Returns the Auth0 subject (user ID) on success.
func ValidateRequest(r *http.Request) (string, string, error) {
	v, err := getValidator()
	if err != nil {
		return "", "", fmt.Errorf("validator init: %w", err)
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", "", fmt.Errorf("missing authorization header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", "", fmt.Errorf("invalid authorization header")
	}

	token := parts[1]
	claims, err := v.ValidateToken(r.Context(), token)
	if err != nil {
		return "", "", fmt.Errorf("invalid token: %w", err)
	}

	validated := claims.(*validator.ValidatedClaims)
	sub := validated.RegisteredClaims.Subject

	return sub, token, nil
}

// FetchUserEmail calls Auth0's /userinfo endpoint to get the user's email.
func FetchUserEmail(accessToken string) (string, error) {
	domain := os.Getenv("AUTH0_DOMAIN")
	req, err := http.NewRequest("GET", "https://"+domain+"/userinfo", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var info struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}
	return info.Email, nil
}
