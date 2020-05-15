/*
https://developer.atlassian.com/cloud/jira/platform/jira-rest-api-oauth-authentication/

This is based on
https://gist.github.com/Lupus/edafe9a7c5c6b13407293d795442fe67

To the extent possible under law, Konstantin Olkhovskiy has waived
all copyright and related or neighboring rights to this snippet.

CC0 license: http://creativecommons.org/publicdomain/zero/1.0/
*/
package main

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/andygrunwald/go-jira"
	"github.com/dghubble/oauth1"
	"golang.org/x/net/context"
)

// https://example.atlassian.net
var jiraURL = os.Getenv("OJA_JIRA_URL")

/*
   $ openssl genrsa -out jira.pem 1024
   $ openssl rsa -in jira.pem -pubout -out jira.pub
-----BEGIN RSA PRIVATE KEY-----
....
.....
.....
-----END RSA PRIVATE KEY-----
*/
var jiraPrivateKey = os.Getenv("OJA_JIRA_PRIVATE_KEY")

func init() {
	if jiraURL == "" || jiraPrivateKey == "" {
		panic("set env vars")
	}
}

const jiraConsumerKey = "OauthKey"

func getJIRAHTTPClient(ctx context.Context, config *oauth1.Config) *http.Client {
	cacheFile, err := jiraTokenCacheFile()
	if err != nil {
		log.Fatalf("Unable to get path to cached credential file. %v", err)
	}
	tok, err := jiraTokenFromFile(cacheFile)
	if err != nil {
		tok = getJIRATokenFromWeb(config)
		saveJIRAToken(cacheFile, tok)
	}
	return config.Client(ctx, tok)
}

func getJIRATokenFromWeb(config *oauth1.Config) *oauth1.Token {
	requestToken, requestSecret, err := config.RequestToken()
	if err != nil {
		log.Fatalf("Unable to get request token. %v", err)
	}
	authorizationURL, err := config.AuthorizationURL(requestToken)
	if err != nil {
		log.Fatalf("Unable to get authorization url. %v", err)
	}
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authorizationURL.String())

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		log.Fatalf("Unable to read authorization code. %v", err)
	}

	accessToken, accessSecret, err := config.AccessToken(requestToken, requestSecret, code)
	if err != nil {
		log.Fatalf("Unable to get access token. %v", err)
	}
	return oauth1.NewToken(accessToken, accessSecret)
}

func jiraTokenCacheFile() (string, error) {
	tokenCacheDir := "credentials"
	os.MkdirAll(tokenCacheDir, 0700)
	return filepath.Join(tokenCacheDir,
		url.QueryEscape((jiraURL)+".json")), nil
}

func jiraTokenFromFile(file string) (*oauth1.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	t := &oauth1.Token{}
	err = json.NewDecoder(f).Decode(t)
	defer f.Close()
	return t, err
}

func saveJIRAToken(file string, token *oauth1.Token) {
	fmt.Printf("Saving credential file to: %s\n", file)
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func getJIRAClient() *jira.Client {
	ctx := context.Background()
	keyDERBlock, _ := pem.Decode([]byte(jiraPrivateKey))
	if keyDERBlock == nil {
		log.Fatal("unable to decode key PEM block")
	}
	if !(keyDERBlock.Type == "PRIVATE KEY" || strings.HasSuffix(keyDERBlock.Type, " PRIVATE KEY")) {
		log.Fatalf("unexpected key DER block type: %s", keyDERBlock.Type)
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(keyDERBlock.Bytes)
	if err != nil {
		log.Fatalf("unable to parse PKCS1 private key. %v", err)
	}
	config := oauth1.Config{
		ConsumerKey: jiraConsumerKey,
		CallbackURL: "oob", /* for command line usage */
		Endpoint: oauth1.Endpoint{
			RequestTokenURL: jiraURL + "/plugins/servlet/oauth/request-token",
			AuthorizeURL:    jiraURL + "/plugins/servlet/oauth/authorize",
			AccessTokenURL:  jiraURL + "/plugins/servlet/oauth/access-token",
		},
		Signer: &oauth1.RSASigner{
			PrivateKey: privateKey,
		},
	}
	jiraClient, err := jira.NewClient(getJIRAHTTPClient(ctx, &config), jiraURL)
	if err != nil {
		log.Fatalf("unable to create new JIRA client. %v", err)
	}
	return jiraClient
}

func main() {
	client := getJIRAClient()
	items, _, err := client.Board.GetAllBoards(nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v", items)
}
