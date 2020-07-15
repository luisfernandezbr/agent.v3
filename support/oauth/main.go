package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/pinpt/go-common/v10/fileutil"
	pjson "github.com/pinpt/go-common/v10/json"
	"github.com/pinpt/go-common/v10/log"
	yaml "gopkg.in/yaml.v2"
)

const serverPort = ":1985"
const redirectURI = "http://localhost" + serverPort + "/callback"

func main() {
	logger := log.NewLogger(os.Stdout, log.ConsoleLogFormat, log.DarkLogColorTheme, log.InfoLevel, "oath")
	providerfile := flag.String("provider", "", "yaml file of provider")
	outfile := flag.String("out", "", "file to save credentials")
	creds := flag.String("creds", "", "yaml file to get credentials from instead of passing in client_id and client_secret")
	clientID := flag.String("client_id", "", "client_id")
	clientSecret := flag.String("client_secret", "", "client_secret")
	flag.Parse()
	if *providerfile == "" {
		log.Fatal(logger, "--provider not passed in")
	}
	if *outfile == "" {
		log.Fatal(logger, "--out not passed in")
	}
	if *creds == "" {
		if *clientID == "" {
			log.Fatal(logger, "--client_id not passed in")
		}
		if *clientSecret == "" {
			log.Fatal(logger, "--client_secret not passed in")
		}
	} else {
		var c struct {
			ClientID     *string `yaml:"client_id"`
			ClientSecret *string `yaml:"client_secret"`
		}
		if err := readFromYaml(*creds, &c); err != nil {
			log.Fatal(logger, "error reading creds yaml", "err", err)
		}
		if c.ClientID == nil || *c.ClientID == "" {
			log.Fatal(logger, "client_id missing from creds yaml")
		}
		if c.ClientSecret == nil || *c.ClientSecret == "" {
			log.Fatal(logger, "client_secret missing from creds yaml")
		}
		clientID = c.ClientID
		clientSecret = c.ClientSecret
	}

	var provider Provider
	if err := readFromYaml(*providerfile, &provider); err != nil {
		log.Fatal(logger, "error creating provider", "err", err)
	}
	oauth := NewOauth(logger, *clientID, *clientSecret, &provider, *outfile)
	if err := oauth.Authenticate(); err != nil {
		log.Fatal(logger, "error refreshing token", "err", err)
	}
	fmt.Println("ACCESS TOKEN")
	fmt.Println(oauth.Token.AccessToken)
	fmt.Println()
	fmt.Println("REFRESH TOKEN")
	fmt.Println(oauth.Token.RefreshToken)
	fmt.Println()
}

func readFromYaml(file string, out interface{}) error {
	if !fileutil.FileExists(file) {
		return fmt.Errorf("File not found %v", file)
	}
	yml, err := os.Open(file)
	defer yml.Close()
	if err != nil {
		return err
	}
	b, err := ioutil.ReadAll(yml)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(b, out)

}

// Provider provider type
type Provider struct {
	Auth        string            `yaml:"auth_uri"`
	Token       string            `yaml:"token_uri"`
	Scope       string            `yaml:"scope"`
	ExtraValues map[string]string `yaml:"extra_values"`
}

// AuthURI authentication url
func (s *Provider) AuthURI() (*url.URL, error) {
	return url.Parse(s.Auth)
}

// TokenURI token url
func (s *Provider) TokenURI() (*url.URL, error) {
	return url.Parse(s.Token)
}

// AddExtraValues extra values for fetching auth token
func (s *Provider) AddExtraValues(values *url.Values) {
	for k, v := range s.ExtraValues {
		values.Set(k, v)
	}
}

// TokenDetails auth token response
type TokenDetails struct {
	ExpiresIn    int    `json:"expires_in"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	// computed here
	ExpireDate time.Time `json:"expire_date"`
}

// Oauth main oath object
type Oauth struct {
	ClientID     string
	ClientSecret string
	Logger       log.Logger
	Provider     *Provider
	Token        *TokenDetails
	OutFile      string
}

// NewOauth creates a new oauth object
func NewOauth(logger log.Logger, clientID string, clientSecret string, provider *Provider, out string) *Oauth {
	o := &Oauth{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Provider:     provider,
		Logger:       logger,
		OutFile:      out,
	}
	if err := pjson.ReadFile(out, &o.Token); err != nil {
		if !strings.HasPrefix(err.Error(), "File not found") {
			panic(err)
		}
	}
	return o
}

// Authenticate checks if token exists and is not expired before requesting a new one
func (s *Oauth) Authenticate() error {
	if s.Token == nil {
		log.Info(s.Logger, "token is nil, will need to authenticate")
		code, err := s.fetchAuthCode()
		if err != nil {
			return err
		}
		err = s.fetchAuthToken(code)
		if err != nil {
			return err
		}
		return nil
	}

	if s.tokenExpired() {
		log.Info(s.Logger, "token is expired, will fetch a new one")
		err := s.refreshAuthToken()
		if err != nil {
			return err
		}
	}
	return nil
}

// AccessToken gets the access token
func (s *Oauth) AccessToken() (string, error) {
	if err := s.Authenticate(); err != nil {
		return "", err
	}
	return s.Token.AccessToken, nil
}
func (s *Oauth) tokenExpired() bool {
	return s.Token.ExpireDate.Before(time.Now())
}
func (s *Oauth) fetchToken(authCode string, grantType string) error {
	tokenurl, err := s.Provider.TokenURI()
	if err != nil {
		return fmt.Errorf("error parsing token uri. err %v", err)
	}
	values := url.Values{}
	values.Set("client_id", s.ClientID)
	values.Set("client_secret", s.ClientSecret)
	if authCode != "" {
		values.Set("code", authCode)
		values.Set("redirect_uri", redirectURI)
	} else {
		values.Set("refresh_token", s.Token.RefreshToken)
	}
	values.Set("grant_type", grantType)
	body := bytes.NewBuffer([]byte(values.Encode()))
	req, err := http.NewRequest(http.MethodPost, tokenurl.String(), body)
	if err != nil {
		return fmt.Errorf("error fetching token. err %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error with http request. err %v", err)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response. err %v", err)
	}
	var details TokenDetails
	if err := json.Unmarshal(b, &details); err != nil {
		return fmt.Errorf("error Unmarshaling token. err %v", err)
	}
	if details.AccessToken == "" {
		return fmt.Errorf("auth token is empty. err %v", err)
	}
	if details.RefreshToken == "" {
		if s.Token.RefreshToken == "" {
			return fmt.Errorf("refresh token is empty. err %v", err)
		}
		details.RefreshToken = s.Token.RefreshToken
	}

	details.ExpireDate = time.Now().Add(time.Duration(details.ExpiresIn) * time.Second)
	b, err = json.MarshalIndent(details, "", "  ")
	if err != nil {
		return fmt.Errorf("error Marshaling token. err %v", err)
	}
	if err := ioutil.WriteFile(s.OutFile, b, 0755); err != nil {
		return fmt.Errorf("error saving token. err %v", err)
	}
	s.Token = &details
	return nil
}
func (s *Oauth) fetchAuthToken(authCode string) error {
	log.Info(s.Logger, "fetching auth token")
	return s.fetchToken(authCode, "authorization_code")
}
func (s *Oauth) refreshAuthToken() error {
	log.Info(s.Logger, "fetching refresh token")
	return s.fetchToken("", "refresh_token")
}
func (s *Oauth) fetchAuthCode() (authCode string, err error) {
	log.Info(s.Logger, "fetching auth code")

	authurl, err := s.Provider.AuthURI()
	if err != nil {
		return "", fmt.Errorf("error parsing auth uri. err %v", err)
	}
	values := authurl.Query()
	if scopes := s.Provider.Scope; len(scopes) > 0 {
		values.Set("scope", scopes)
	}
	values.Set("client_id", s.ClientID)
	values.Set("redirect_uri", redirectURI)
	values.Set("response_type", "code")
	s.Provider.AddExtraValues(&values)
	authurl.RawQuery = values.Encode()

	fmt.Println("open in browser:", authurl.String())
	s.openBrowser(authurl.String(), func(vals url.Values) {
		authCode = vals.Get("code")
	})
	return
}
func (s *Oauth) openBrowser(url string, callback func(vals url.Values)) error {

	server := &http.Server{
		Addr:    serverPort,
		Handler: nil,
	}
	http.HandleFunc("/callback", func(resp http.ResponseWriter, req *http.Request) {
		resp.Write([]byte(`
				<!doctype html>
				<html lang="en">
				<head></head>
				<body><center><h1>You can close this window now</h1></center></body>
				</html>
			`))
		callback(req.URL.Query())
		go func() {
			timer := time.NewTimer(250 * time.Microsecond)
			<-timer.C
			server.Close()
		}()
	})
	{
		var err error
		switch runtime.GOOS {
		case "linux":
			err = exec.Command("xdg-open", url).Run()
		case "windows":
			err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Run()
		case "darwin":
			err = exec.Command("open", url).Run()
		default:
			err = fmt.Errorf("unsupported platform")
		}
		if err != nil {
			return fmt.Errorf("error opening in browser. err %s", err.Error())
		}
	}
	if err := server.ListenAndServe(); err != nil {
		if !strings.Contains(err.Error(), "Server closed") {
			return fmt.Errorf("error with local server. err %s", err.Error())
		}
	}
	return nil
}
