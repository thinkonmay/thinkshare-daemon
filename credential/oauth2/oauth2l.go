// Copyright 2019 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package oauth2l

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// Common prefix for google oauth scope
	scopePrefix = "https://www.googleapis.com/auth/"

	// Default state parameter used for 3LO flow
	defaultState = "state"
)

var (
	// Holds the parsed command-line flags
	opts commandOptions

	// Multiple scopes are separate by comma, space, or comma-space.
	scopeDelimiter = regexp.MustCompile("[, ] *")

	// OpenId scopes should not be prefixed with scopePrefix.
	openIdScopes = regexp.MustCompile("^(openid|profile|email)$")
)

// Top level command-line flags (first argument after program name).
type commandOptions struct {
	Fetch  fetchOptions  `command:"fetch" description:"Fetch an access token."`
	Header headerOptions `command:"header" description:"Fetch an access token and return it in header format."`
	Curl   curlOptions   `command:"curl" description:"Fetch an access token and use it to make a curl request."`
	Info   infoOptions   `command:"info" description:"Display info about an OAuth access token."`
	Test   infoOptions   `command:"test" description:"Tests an OAuth access token. Returns 0 for valid token."`
	Reset  resetOptions  `command:"reset" description:"Resets the cache."`
	Web    webOptions    `command:"web"   description:"Launches a local instance of the OAuth2l Playground web app. This feature is experimental."`
}

// Common options for "fetch", "header", and "curl" commands.
type commonFetchOptions struct {
	// Currently there are 3 authentication types that are mutually exclusive:
	//
	// oauth - Executes 2LO flow for Service Account and 3LO flow for OAuth Client ID. Returns OAuth token.
	// jwt - Signs claims (in JWT format) using PK. Returns signature as token. Only works for Service Account.
	// sso - Exchanges LOAS credential to OAuth token.
	AuthType string `long:"type" choice:"oauth" choice:"jwt" choice:"sso" description:"The authentication type." default:"oauth"`

	// GUAC parameters
	Credentials    string `long:"credentials" description:"Credentials file containing OAuth Client Id or Service Account Key. Optional if environment variable GOOGLE_APPLICATION_CREDENTIALS is set."`
	Scope          string `long:"scope" description:"List of OAuth scopes requested. Required for oauth and sso authentication type. Comma delimited."`
	Audience       string `long:"audience" description:"Audience used for JWT self-signed token and STS. Required for jwt authentication type."`
	Email          string `long:"email" description:"Email associated with SSO. Required for sso authentication type."`
	QuotaProject   string `long:"quota_project" description:"Project override for quota and billing. Used for STS."`
	Sts            bool   `long:"sts" description:"Perform STS token exchange."`
	ServiceAccount string `long:"impersonate-service-account" description:"Exchange User acccess token for Service Account access token."`

	// Client parameters
	SsoCli string `long:"ssocli" description:"Path to SSO CLI. Optional."`

	// Cache is declared as a pointer type and can be one of nil, empty (""), or a custom file path.
	Cache *string `long:"cache" description:"Path to the credential cache file. Disables caching if set to empty. Defaults to ~/.oauth2l."`

	// Refresh is used for 3LO flow. When used in conjunction with caching, the user can avoid re-authorizing.
	Refresh bool `long:"refresh" description:"If the cached access token is expired, attempt to refresh it using refreshToken."`

	// Consent page parameters.
	DisableAutoOpenConsentPage         bool   `long:"disableAutoOpenConsentPage" description:"Disables the ability to open the consent page automatically."`
	ConsentPageInteractionTimeout      int    `long:"consentPageInteractionTimeout" description:"Maximum wait time for user to interact with consent page." default:"2"`
	ConsentPageInteractionTimeoutUnits string `long:"consentPageInteractionTimeoutUnits" choice:"seconds" choice:"minutes" description:"Consent page timeout units." default:"minutes"`

	// Deprecated flags kept for backwards compatibility. Hidden from help page.
	Json      string `long:"json" description:"Deprecated. Same as --credentials." hidden:"true"`
	Jwt       bool   `long:"jwt" description:"Deprecated. Same as --type jwt." hidden:"true"`
	Sso       bool   `long:"sso" description:"Deprecated. Same as --type sso." hidden:"true"`
	OldFormat string `long:"credentials_format" choice:"bare" choice:"header" choice:"json" choice:"json_compact" choice:"pretty" description:"Deprecated. Same as --output_format" hidden:"true"`
}

// Additional options for "fetch" command.
type fetchOptions struct {
	commonFetchOptions
	Format string `long:"output_format" choice:"bare" choice:"header" choice:"json" choice:"json_compact" choice:"pretty" choice:"refresh_token" description:"Token's output format." default:"bare"`
}

// Additional options for "header" command.
type headerOptions struct {
	commonFetchOptions
}

// Additional options for "curl" command.
type curlOptions struct {
	commonFetchOptions
	CurlCli string `long:"curlcli" description:"Path to Curl CLI. Optional."`
	Url     string `long:"url" description:"URL endpoint for the curl request." required:"true"`
}

// Options for "info" and "test" commands.
type infoOptions struct {
	Token string `long:"token" description:"OAuth access token to analyze."`
}

// Options for "reset" command.
type resetOptions struct {
	// Cache is declared as a pointer type and can be one of nil or a custom file path.
	Cache *string `long:"cache" description:"Path to the credential cache file to remove. Defaults to ~/.oauth2l."`
}

// Options for "web" command
type webOptions struct {
	Stop      bool   `long:"stop" description:"Stops the OAuth2l Playground where OAuth2l-web should be located."`
	Directory string `long:"directory" description:"Sets the directory of where OAuth2l-web should be located. Defaults to ~/.oauth2l-web." `
}



// Extracts the common fetch options based on chosen command.
func getCommonFetchOptions(cmdOpts commandOptions, cmd string) commonFetchOptions {
	var commonOpts commonFetchOptions
	switch cmd {
	case "fetch":
		commonOpts = cmdOpts.Fetch.commonFetchOptions
	case "header":
		commonOpts = cmdOpts.Header.commonFetchOptions
	case "curl":
		commonOpts = cmdOpts.Curl.commonFetchOptions
	}
	return commonOpts
}

// Generates a time duration
func getTimeDuration(quantity int, units string) (time.Duration, error) {
	switch units {
	case "seconds":
		return time.Duration(quantity) * time.Second, nil
	case "minutes":
		return time.Duration(quantity) * time.Minute, nil
	default:
		return time.Duration(0), fmt.Errorf("Invalid units: %s", units)
	}
}


func StartAuth(clientID string, redirectPort int) (string, error) {
	commonOpts := getCommonFetchOptions(opts, "fetch")

	var authCodeServer AuthorizationCodeServer = nil
	var consentPageSettings ConsentPageSettings
	interactionTimeout,_ := getTimeDuration(2, "minutes")
	consentPageSettings = ConsentPageSettings{
		DisableAutoOpenConsentPage: commonOpts.DisableAutoOpenConsentPage,
		InteractionTimeout:         interactionTimeout,
	}
	authCodeServer = &AuthorizationCodeLocalhost{
		ConsentPageSettings: consentPageSettings,
		AuthCodeReqStatus: AuthorizationCodeStatus{
			Status: WAITING, Details: "Authorization code not yet set."},
	}


	// Start localhost server
	_,err := authCodeServer.ListenAndServe(fmt.Sprintf("localhost:%d",redirectPort))
	if err != nil { return "", err }
	defer authCodeServer.Close()

	src := authHandlerSource{
		config: &Config{
			ClientID:    clientID,


			Scopes:      []string{"openid","email","profile"},
			RedirectURL: fmt.Sprintf("http://localhost:%d",redirectPort),
			Endpoint: Endpoint{
				AuthURL:  "https://accounts.google.com/o/oauth2/auth",
				TokenURL: "https://oauth2.googleapis.com/token",
			},
		} , 
		state: "state", 
		ctx: context.Background(), 
		authHandler: Get3LOAuthorizationHandler(defaultState, consentPageSettings, &authCodeServer), 
		pkce: GeneratePKCEParams(),
	}

	token,err := src.Token()
	if err != nil {
		return "", err
	}

	return token,nil
}


// CredentialsParams holds user supplied parameters that are used together
// with a credentials file for building a Credentials object.
type CredentialsParams struct {
	// Scopes is the list OAuth scopes. Required.
	// Example: https://www.googleapis.com/auth/cloud-platform
	Scopes []string

	// Subject is the user email used for domain wide delegation (see
	// https://developers.google.com/identity/protocols/oauth2/service-account#delegatingauthority).
	// Optional.
	Subject string

	// AuthHandler is the AuthorizationHandler used for 3-legged OAuth flow. Required for 3LO flow.
	AuthHandler AuthorizationHandler

	// State is a unique string used with AuthHandler. Required for 3LO flow.
	State string

	// PKCE is used to support PKCE flow. `Optional for 3LO flow.
	PKCE *PKCEParams
}

// AuthorizationHandler is a 3-legged-OAuth helper that prompts
// the user for OAuth consent at the specified auth code URL
// and returns an auth code and state upon approval.
type AuthorizationHandler func(authCodeURL string) (code string, state string, err error)


const (
	// Parameter keys for AuthCodeURL method to support PKCE.
	codeChallengeKey       = "code_challenge"
	codeChallengeMethodKey = "code_challenge_method"

	// Parameter key for Exchange method to support PKCE.
	codeVerifierKey = "code_verifier"
)


type authHandlerSource struct {
	ctx         context.Context
	config      *Config
	authHandler AuthorizationHandler
	state       string
	pkce        *PKCEParams
}

func (source authHandlerSource) Token() (string, error) {
	// Step 1: Obtain auth code.
	var authCodeUrlOptions []AuthCodeOption
	if source.pkce != nil && source.pkce.Challenge != "" && source.pkce.ChallengeMethod != "" {
		authCodeUrlOptions = []AuthCodeOption{
			SetAuthURLParam(codeChallengeKey, source.pkce.Challenge),
			SetAuthURLParam(codeChallengeMethodKey, source.pkce.ChallengeMethod)}
	}
	authurl := source.config.AuthCodeURL(source.state, authCodeUrlOptions...)
	code, state, err := source.authHandler(authurl)
	if err != nil {
		return "", err
	}
	if state != source.state {
		return "", errors.New("state mismatch in 3-legged-OAuth flow")
	}

	// Step 2: Exchange auth code for access token.
	var exchangeOptions []AuthCodeOption
	if source.pkce != nil && source.pkce.Verifier != "" {
		exchangeOptions = []AuthCodeOption{SetAuthURLParam(codeVerifierKey, source.pkce.Verifier)}
	}



	v := url.Values{
		"redirect_uri": {source.config.RedirectURL},
		"grant_type": {"authorization_code"},
		"code":       {code},
	}

	for _, opt := range exchangeOptions {
		opt.setValue(v)
	}

	return v.Encode(),nil
}







// PKCEParams holds parameters to support PKCE.
type PKCEParams struct {
	Challenge       string // The unpadded, base64-url-encoded string of the encrypted code verifier.
	ChallengeMethod string // The encryption method (ex. S256).
	Verifier        string // The original, non-encrypted secret.
}

// GeneratePKCEParams generates a unique PKCE challenge and verifier combination,
// using UUID, SHA256 encryption, and base64 URL encoding with no padding.
func GeneratePKCEParams() *PKCEParams {
	verifier := uuid.New().String()
	sha := sha256.Sum256([]byte(verifier))
	challenge := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(sha[:])

	return &PKCEParams{
		Challenge:       challenge,
		ChallengeMethod: "S256",
		Verifier:        verifier,
	}
}


// 3LO authorization handler. Determines what algorithm to use
// to get the authorization code.
//
// Note that the "state" parameter is used to prevent CSRF attacks.
func Get3LOAuthorizationHandler(state string, consentSettings ConsentPageSettings,
	authCodeServer *AuthorizationCodeServer) AuthorizationHandler {
	return func(authCodeURL string) (string, string, error) {
		decodedValue, _ := url.ParseQuery(authCodeURL)
		redirectURL := decodedValue.Get("redirect_uri")

		if strings.Contains(redirectURL, "localhost") {
			return authorization3LOLoopback(authCodeURL, consentSettings, authCodeServer)
		}

		return authorization3LOOutOfBand(state, authCodeURL)
	}
}

// authorization3LOOutOfBand prints the authorization URL on stdout
// and reads the authorization code from stdin.
//
// Note that the "state" parameter is used to prevent CSRF attacks.
// For convenience, authorization3LOOutOfBand returns a pre-configured state
// instead of requiring the user to copy it from the browser.
func authorization3LOOutOfBand(state string, authCodeURL string) (string, string, error) {
	fmt.Printf("Go to the following link in your browser:\n\n   %s\n\n", authCodeURL)
	fmt.Println("Enter authorization code:")
	var code string
	fmt.Scanln(&code)
	return code, state, nil
}


// authorization3LOLoopback prints the authorization URL on stdout
// and redirects the user to the authCodeURL in a new browser's tab.
// if `DisableAutoOpenConsentPage` is set, then the user is instructed
// to manually open the authCodeURL in a new browser's tab.
//
// The code and state output parameters in this function are the same
// as the ones generated after the user grants permission on the consent page.
// When the user interacts with the consent page, an error or a code-state-tuple
// is expected to be returned to the Auth Code Localhost Server endpoint
// (see loopback.go for more info).
func authorization3LOLoopback(authCodeURL string, consentSettings ConsentPageSettings,
	authCodeServer *AuthorizationCodeServer) (string, string, error) {
	const (
		// Max wait time for the server to start listening and serving
		maxWaitForListenAndServe time.Duration = 10 * time.Second
	)

	// (Step 1) Start local Auth Code Server
	if started, _ := (*authCodeServer).WaitForListeningAndServing(maxWaitForListenAndServe); started {
		// (Step 2) Provide access to the consent page
		if consentSettings.DisableAutoOpenConsentPage { // Auto open consent disabled
			fmt.Println("Go to the following link in your browser:")
			fmt.Println("\n", authCodeURL)
		} else { // Auto open consent
			b := Browser{}
			if be := b.OpenURL(authCodeURL); be != nil {
				fmt.Println("Your browser could not be opened to visit:")
				fmt.Println("\n", authCodeURL)
				fmt.Println("\nError:", be)
			} else {
				fmt.Println("Your browser has been opened to visit:")
				fmt.Println("\n", authCodeURL)
			}
		}

		// (Step 3) Wait for user to interact with consent page
		(*authCodeServer).WaitForConsentPageToReturnControl()
	}

	// (Step 4) Attempt to get Authorization code. If one was not received
	// default string values are returned.
	code, err := (*authCodeServer).GetAuthenticationCode()
	return code.Code, code.State, err
}



// Endpoint represents an OAuth 2.0 provider's authorization and token
// endpoint URLs.
type Endpoint struct {
	AuthURL  string
	TokenURL string

	// AuthStyle optionally specifies how the endpoint wants the
	// client ID & client secret sent. The zero value means to
	// auto-detect.
	AuthStyle AuthStyle
}


// AuthStyle represents how requests for tokens are authenticated
// to the server.
type AuthStyle int

const (
	// AuthStyleAutoDetect means to auto-detect which authentication
	// style the provider wants by trying both ways and caching
	// the successful way for the future.
	AuthStyleAutoDetect AuthStyle = 0

	// AuthStyleInParams sends the "client_id" and "client_secret"
	// in the POST body as application/x-www-form-urlencoded parameters.
	AuthStyleInParams AuthStyle = 1

	// AuthStyleInHeader sends the client_id and client_password
	// using HTTP Basic Authorization. This is an optional style
	// described in the OAuth2 RFC 6749 section 2.3.1.
	AuthStyleInHeader AuthStyle = 2
)



// Config describes a typical 3-legged OAuth2 flow, with both the
// client application information and the server's endpoint URLs.
// For the client credentials 2-legged OAuth2 flow, see the clientcredentials
// package (https://github.com/pigeatgarlic/oauth2l/tools/oauth2/clientcredentials).
type Config struct {
	// ClientID is the application's ID.
	ClientID string

	// Endpoint contains the resource server's token endpoint
	// URLs. These are constants specific to each server and are
	// often available via site-specific packages, such as
	// google.Endpoint or github.Endpoint.
	Endpoint Endpoint

	// RedirectURL is the URL to redirect users going through
	// the OAuth flow, after the resource owner's URLs.
	RedirectURL string

	// Scope specifies optional requested permissions.
	Scopes []string
}



// AuthCodeURL returns a URL to OAuth 2.0 provider's consent page
// that asks for permissions for the required scopes explicitly.
//
// State is a token to protect the user from CSRF attacks. You must
// always provide a non-empty string and validate that it matches the
// the state query parameter on your redirect callback.
// See http://tools.ietf.org/html/rfc6749#section-10.12 for more info.
//
// Opts may include AccessTypeOnline or AccessTypeOffline, as well
// as ApprovalForce.
// It can also be used to pass the PKCE challenge.
// See https://www.oauth.com/oauth2-servers/pkce/ for more info.
func (c *Config) AuthCodeURL(state string, opts ...AuthCodeOption) string {
	var buf bytes.Buffer
	buf.WriteString(c.Endpoint.AuthURL)
	v := url.Values{
		"response_type": {"code"},
		"client_id":     {c.ClientID},
	}
	if c.RedirectURL != "" {
		v.Set("redirect_uri", c.RedirectURL)
	}
	if len(c.Scopes) > 0 {
		v.Set("scope", strings.Join(c.Scopes, " "))
	}
	if state != "" {
		// TODO(light): Docs say never to omit state; don't allow empty.
		v.Set("state", state)
	}
	for _, opt := range opts {
		opt.setValue(v)
	}
	if strings.Contains(c.Endpoint.AuthURL, "?") {
		buf.WriteByte('&')
	} else {
		buf.WriteByte('?')
	}
	buf.WriteString(v.Encode())
	return buf.String()
}




type setParam struct{ k, v string }
func (p setParam) setValue(m url.Values) { m.Set(p.k, p.v) }

var (
	// AccessTypeOnline and AccessTypeOffline are options passed
	// to the Options.AuthCodeURL method. They modify the
	// "access_type" field that gets sent in the URL returned by
	// AuthCodeURL.
	//
	// Online is the default if neither is specified. If your
	// application needs to refresh access tokens when the user
	// is not present at the browser, then use offline. This will
	// result in your application obtaining a refresh token the
	// first time your application exchanges an authorization
	// code for a user.
	AccessTypeOnline  AuthCodeOption = SetAuthURLParam("access_type", "online")
	AccessTypeOffline AuthCodeOption = SetAuthURLParam("access_type", "offline")

	// ApprovalForce forces the users to view the consent dialog
	// and confirm the permissions request at the URL returned
	// from AuthCodeURL, even if they've already done so.
	ApprovalForce AuthCodeOption = SetAuthURLParam("prompt", "consent")
)

// An AuthCodeOption is passed to Config.AuthCodeURL.
type AuthCodeOption interface {
	setValue(url.Values)
}


// SetAuthURLParam builds an AuthCodeOption which passes key/value parameters
// to a provider's authorization endpoint.
func SetAuthURLParam(key, value string) AuthCodeOption {
	return setParam{key, value}
}
