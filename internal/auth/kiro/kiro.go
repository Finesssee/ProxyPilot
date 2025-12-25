package kiro

// PKCECodes holds the verification codes for the OAuth2 PKCE (Proof Key for Code Exchange) flow.
// PKCE is an extension to the Authorization Code flow to prevent CSRF and authorization code injection attacks.
type PKCECodes struct {
	// CodeVerifier is the cryptographically random string used to correlate
	// the authorization request to the token request
	CodeVerifier string `json:"code_verifier"`
	// CodeChallenge is the SHA256 hash of the code verifier, base64url-encoded
	CodeChallenge string `json:"code_challenge"`
}

// KiroTokenData holds the OAuth token information obtained from AWS Kiro.
// It includes access token, refresh token, ID token, and associated user details.
type KiroTokenData struct {
	// AccessToken is the OAuth2 access token for API access
	AccessToken string `json:"access_token"`
	// RefreshToken is used to obtain new access tokens
	RefreshToken string `json:"refresh_token"`
	// IDToken is the JWT ID token containing user claims (optional)
	IDToken string `json:"id_token,omitempty"`
	// Email is the user's email address
	Email string `json:"email"`
	// ExpiresAt is the timestamp when the token expires
	ExpiresAt string `json:"expires_at"`
	// AuthMethod indicates the authentication method used ("builder_id" or "google")
	AuthMethod string `json:"auth_method"`
	// ProfileArn is the AWS CodeWhisperer profile ARN (optional)
	ProfileArn string `json:"profile_arn,omitempty"`
	// ClientID is the AWS SSO client ID (for Builder ID auth)
	ClientID string `json:"client_id,omitempty"`
	// ClientSecret is the AWS SSO client secret (for Builder ID auth)
	ClientSecret string `json:"client_secret,omitempty"`
	// Provider indicates the OAuth provider ("google" or "aws")
	Provider string `json:"provider,omitempty"`
}

// KiroAuthBundle aggregates all authentication-related data after the OAuth flow is complete.
// This includes the token data and the timestamp of the last refresh.
type KiroAuthBundle struct {
	// TokenData contains the OAuth tokens from the authentication flow
	TokenData KiroTokenData `json:"token_data"`
	// LastRefresh is the timestamp of the last token refresh
	LastRefresh string `json:"last_refresh"`
}
