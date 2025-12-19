package copilot

// CopilotTokenData represents the OAuth credentials for GitHub Copilot.
type CopilotTokenData struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
	// Expire indicates the expiration date and time of the access token.
	Expire string `json:"expiry_date,omitempty"`
}

// DeviceFlow represents the response from the device authorization endpoint.
type DeviceFlow struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// CopilotTokenResponse represents the successful token response from the token endpoint.
type CopilotTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope,omitempty"`
	Error        string `json:"error,omitempty"`
}

// CopilotTokenStorage is the struct used for persisting token data.
type CopilotTokenStorage struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	LastRefresh  string `json:"last_refresh,omitempty"`
	Expire       string `json:"expire,omitempty"`
}

// SaveTokenToFile persists authentication tokens to the specified file path.
func (ts *CopilotTokenStorage) SaveTokenToFile(authFilePath string) error {
	// Logic to save token to file would go here if needed,
	// but the core auth manager handles persistence via the Storage field
	// being marshalled into the Auth struct.
	// This method satisfies the TokenStorage interface.
	return nil
}
