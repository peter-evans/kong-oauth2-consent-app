/*
	Kong OAuth 2.0 Consent Application

	A sample consent application demonstrating the OAuth 2.0 authorization code grant
	flow with Kong, the microservice API gateway.

	Peter Evans
	MIT License (See LICENSE file)
*/

package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/sessions"
)

var (
	demoClientID           = os.Getenv("DEMO_CLIENT_ID")
	kongAdminEndpoint      = os.Getenv("KONG_ADMIN_ENDPOINT")
	kongProxyEndpoint      = os.Getenv("KONG_PROXY_ENDPOINT")
	apiPath                = os.Getenv("API_PATH")
	provisionKey           = os.Getenv("PROVISION_KEY")
	cookieNameForSessionID = "kongOAuthConsentApp"
	sess                   = sessions.New(sessions.Config{Cookie: cookieNameForSessionID})
	userAgent              = "kong-oauth2-consent-app"
)

// Credentials represents a set of user credentials for the consent application
type Credentials struct {
	Username string
	Password string
}

// ConsentRequest represents a request for user consent made by the client application
type ConsentRequest struct {
	ClientID     string
	ResponseType string
	Scopes       string
}

// OAuth2Credential is a partial representation of Kong's OAuth 2.0 credential resource
type OAuth2Credential struct {
	ApplicationName string `json:"name"`
}

// OAuth2Credentials is a partial representation of Kong's OAuth 2.0 credentials resource
type OAuth2Credentials struct {
	Data []OAuth2Credential `json:"data"`
}

// AuthorizeResponse is a partial representation of the response from Kong's '/oauth2/authorize' endpoint
type AuthorizeResponse struct {
	RedirectURI string `json:"redirect_uri"`
}

// main is the entrypoint for the consent application
func main() {
	// For testing purposes only TLS certificate verification is disabled
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	app := iris.New()

	// Register html templates for views
	app.RegisterView(iris.HTML("./templates", ".html"))

	// Register routes
	app.Get("/", getIndex)
	app.Get("/consent", getConsent)
	app.Post("/consent", postConsent)
	app.Get("/login", getLogin)
	app.Post("/login", postLogin)
	app.Get("/logout", getLogout)

	// Now listening on: http://localhost:8080
	// Application started. Press CTRL+C to shut down.
	app.Run(iris.Addr("localhost:8080"))
}

// executeRequest executes an HTTP request and returns the response body
func executeRequest(req *http.Request) ([]byte, error) {
	req.Header.Set("User-Agent", userAgent)

	httpClient := http.Client{
		Timeout: time.Second * 2,
	}

	res, getErr := httpClient.Do(req)
	if getErr != nil {
		return nil, getErr
	}

	defer res.Body.Close()
	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		return nil, readErr
	}

	return body, nil
}

// getApplicationName queries the OAuth 2.0 credentials on Kong to fetch the application name
func getApplicationName(clientID string) (string, error) {
	url := kongAdminEndpoint + "/oauth2?client_id=" + clientID

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	body, exErr := executeRequest(req)
	if exErr != nil {
		return "", exErr
	}

	creds := OAuth2Credentials{}
	jsonErr := json.Unmarshal(body, &creds)
	if jsonErr != nil {
		return "", jsonErr
	}

	return creds.Data[0].ApplicationName, nil
}

// getRedirectURI queries Kong's '/oauth2/authorize' endpoint and returns the 'redirect_uri' property
func getRedirectURI(consent ConsentRequest) (string, error) {
	authPath := kongProxyEndpoint + apiPath + "/oauth2/authorize"

	data := url.Values{}
	data.Set("client_id", consent.ClientID)
	data.Add("response_type", consent.ResponseType)
	data.Add("scope", strings.Replace(consent.Scopes, ",", " ", -1))
	data.Add("provision_key", provisionKey)
	// This should be the ID that you use to identify the client in your system
	data.Add("authenticated_userid", "client-userid")

	req, err := http.NewRequest(http.MethodPost, authPath, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; param=value")
	body, exErr := executeRequest(req)
	if exErr != nil {
		return "", exErr
	}

	response := AuthorizeResponse{}
	jsonErr := json.Unmarshal(body, &response)
	if jsonErr != nil {
		return "", jsonErr
	}

	return response.RedirectURI, nil
}

// getIndex returns the home view on a GET request
func getIndex(ctx iris.Context) {
	// To begin the OAuth 2.0 Authorization Code Grant flow the client application should redirect the user to
	// the consent endpoint, passing client_id, response_type and scope parameters.
	// For demonstration purposes we construct this URI and display it on the home page.
	consentURI := "/consent?client_id=" + demoClientID + "&response_type=code&scopes=email%2Cphone%2Caddress"
	ctx.ViewData("consentURI", consentURI)
	ctx.View("index.html")
}

// getConsent returns the consent view on a GET request
//
// If the user is not authenticated they will be redirected to the login page.
// If the user is authenticated they will be asked to authorize the client application.
func getConsent(ctx iris.Context) {
	var (
		clientID     = ctx.URLParam("client_id")
		responseType = ctx.URLParam("response_type")
		scopes       = ctx.URLParam("scopes")
	)

	session := sess.Start(ctx)

	// If the user is not authenticated redirect to the login page
	if auth, _ := session.GetBoolean("authenticated"); !auth {
		session.Set("clientID", clientID)
		session.Set("responseType", responseType)
		session.Set("scopes", scopes)
		ctx.Redirect("/login", iris.StatusTemporaryRedirect)
		return
	}

	// Retrieve the name of the client application registered with Kong
	applicationName, err := getApplicationName(clientID)
	if err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		ctx.WriteString(err.Error())
		return
	}

	// Return the consent view
	ctx.ViewData("ApplicationName", applicationName)
	ctx.ViewData("ClientID", clientID)
	ctx.ViewData("ResponseType", responseType)
	ctx.ViewData("Scopes", scopes)
	ctx.ViewData("RequestedScopes", strings.Split(scopes, ","))
	ctx.View("consent.html")
}

// postConsent handles POST requests to the consent endpoint
//
// On user authorization of the client application a request is made to Kong for an authorization code.
func postConsent(ctx iris.Context) {
	consent := ConsentRequest{}
	err := ctx.ReadForm(&consent)
	if err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		ctx.WriteString(err.Error())
		return
	}

	// Call the '/oauth2/authorize' endpoint to request an authorization code. Kong will
	// respond with either a 200 OK or 400 Bad request response code. In -both- cases,
	// redirect the user to the URI returned in the redirect_url property.
	redirectURI, err := getRedirectURI(consent)
	if err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		ctx.WriteString(err.Error())
		return
	}

	// At this point the user should be redirected back to the client application.
	// For demonstration purposes the redirect URI is simply output.
	ctx.WriteString("redirect_uri: " + redirectURI)
}

// getLogin returns the login view on a GET request
func getLogin(ctx iris.Context) {
	ctx.View("login.html")
}

// postLogin handles POST requests to the login endpoint
//
// On successful authentication the user is redirected to the consent page
func postLogin(ctx iris.Context) {
	credentials := Credentials{}
	err := ctx.ReadForm(&credentials)
	if err != nil {
		ctx.StatusCode(iris.StatusInternalServerError)
		ctx.WriteString(err.Error())
		return
	}

	// *** Authenticate the user here ***
	// credentials.Username
	// credentials.Password

	session := sess.Start(ctx)

	// Set user as authenticated
	session.Set("authenticated", true)

	consentURL := "/consent?client_id=" + session.GetString("clientID") +
		"&response_type=" + session.GetString("responseType") +
		"&scopes=" + session.GetString("scopes")

	// Redirect to the consent page with status code 303 "See Other"
	ctx.Redirect(consentURL, iris.StatusSeeOther)
}

// getLogout initiates a logout and redirect to the home page on a GET request
func getLogout(ctx iris.Context) {
	session := sess.Start(ctx)
	// Clear the user's session
	session.Clear()
	ctx.Redirect("/", iris.StatusTemporaryRedirect)
}
