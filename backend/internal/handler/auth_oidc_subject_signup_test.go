package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/ent/authidentity"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// A subject-only account must never take an email owner's existing account or
// bypass the administrator's verification and invitation requirements.
func TestOIDCSubjectSignupTrustBoundaries(t *testing.T) {
	trusted := config.OIDCConnectConfig{ValidateIDToken: true}
	require.True(t, oidcSubjectSignupAllowed(trusted, "", false, false, false))
	require.False(t, oidcSubjectSignupAllowed(trusted, "someone@example.com", false, false, false))
	require.False(t, oidcSubjectSignupAllowed(config.OIDCConnectConfig{}, "", false, false, false))
	require.False(t, oidcSubjectSignupAllowed(config.OIDCConnectConfig{
		ValidateIDToken: true, RequireEmailVerified: true,
	}, "", false, false, false))
	require.False(t, oidcSubjectSignupAllowed(trusted, "", true, false, false))
	require.False(t, oidcSubjectSignupAllowed(trusted, "", false, true, false))
	require.False(t, oidcSubjectSignupAllowed(trusted, "", false, false, true))

	// Account keys are independent of display name, email, and other clients.
	a := oidcSyntheticEmailFromIdentityKey(oidcIdentityKey("https://auth.example/identity", "subject-a"))
	require.Equal(t, a, oidcSyntheticEmailFromIdentityKey(oidcIdentityKey("https://auth.example/identity", "subject-a")))
	require.NotEqual(t, a, oidcSyntheticEmailFromIdentityKey(oidcIdentityKey("https://auth.example/identity", "subject-b")))
	require.NotEqual(t, a, oidcSyntheticEmailFromIdentityKey(oidcIdentityKey("https://another.example/identity", "subject-a")))
}

func TestOIDCSubjectSignupCreatesAndBindsAccountWithoutMailbox(t *testing.T) {
	cfg, cleanup := newOIDCTestProvider(t, oidcProviderFixture{
		Subject: "subject-only", PreferredUsername: "Feishu Member",
	})
	defer cleanup()
	h, db := newOIDCOAuthHandlerAndClientWithSettings(t, false, cfg, map[string]string{
		"auth_source_default_oidc_balance":         "200000",
		"auth_source_default_oidc_grant_on_signup": "true",
	})
	t.Cleanup(func() { _ = db.Close() })
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/oidc/callback?code=oidc-code&state=subject-state", nil)
	for _, pair := range [][2]string{
		{oidcOAuthStateCookieName, "subject-state"},
		{oidcOAuthRedirectCookie, "/dashboard"},
		{oidcOAuthVerifierCookie, "verifier"},
		{oidcOAuthNonceCookie, "nonce-subject-only"},
		{oidcOAuthIntentCookieName, oauthIntentLogin},
		{oauthPendingBrowserCookieName, "subject-browser"},
	} {
		r.AddCookie(encodedCookie(pair[0], pair[1]))
	}
	c.Request = r
	h.OIDCOAuthCallback(c)
	require.Equal(t, http.StatusFound, w.Code)
	require.NotContains(t, w.Header().Get("Location"), "error=")
	u, err := db.User.Query().Only(context.Background())
	require.NoError(t, err)
	require.Equal(t, oidcSyntheticEmailFromIdentityKey(oidcIdentityKey(cfg.IssuerURL, "subject-only")), u.Email)
	require.Equal(t, "oidc", u.SignupSource)
	require.Equal(t, float64(200000), u.Balance)

	exchange := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(exchange)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/pending/exchange",
		strings.NewReader(`{"adopt_display_name":false,"adopt_avatar":false}`))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range w.Result().Cookies() {
		if cookie.MaxAge >= 0 {
			req.AddCookie(cookie)
		}
	}
	req.AddCookie(encodedCookie(oauthPendingBrowserCookieName, "subject-browser"))
	ctx.Request = req
	h.ExchangePendingOAuthCompletion(ctx)
	require.Equal(t, http.StatusOK, exchange.Code, exchange.Body.String())
	require.Contains(t, exchange.Body.String(), "access_token")
	identity, err := db.AuthIdentity.Query().Where(authidentity.ProviderSubjectEQ("subject-only")).Only(context.Background())
	require.NoError(t, err)
	require.Equal(t, u.ID, identity.UserID)
	count, err := db.User.Query().Count(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, count)
}
