package reqstats

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dghubble/oauth1"
)

func OAuth1HTTPClient(url, consumerKey, token string) (*http.Client, error) {
	ctx := context.Background()
	keyDERBlock, _ := pem.Decode([]byte(os.Getenv("OJA_JIRA_PRIVATE_KEY")))
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
		ConsumerKey: consumerKey,
		CallbackURL: "oob", /* for command line usage */
		Endpoint: oauth1.Endpoint{
			RequestTokenURL: url + "/plugins/servlet/oauth/request-token",
			AuthorizeURL:    url + "/plugins/servlet/oauth/authorize",
			AccessTokenURL:  url + "/plugins/servlet/oauth/access-token",
		},
		Signer: &oauth1.RSASigner{
			PrivateKey: privateKey,
		},
	}

	tok := &oauth1.Token{Token: token, TokenSecret: os.Getenv("OJA_JIRA_TOKEN_SECRET")}

	return config.Client(ctx, tok), nil

}

func GetJIRAHTTPClient(ctx context.Context, config *oauth1.Config) (*http.Client, error) {
	cacheFile, err := jiraTokenCacheFile()
	if err != nil {
		log.Fatalf("Unable to get path to cached credential file. %v", err)
	}
	tok, err := jiraTokenFromFile(cacheFile)
	if err != nil {
		return nil, err
	}
	return config.Client(ctx, tok), nil
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

func jiraTokenCacheFile() (string, error) {
	tokenCacheDir := "/Users/carlos/go/src/github.com/pinpt/agent/support/oauthjiraagile/credentials"
	os.MkdirAll(tokenCacheDir, 0700)
	return filepath.Join(tokenCacheDir, "http%3A%2F%2Flocalhost%3A8084.json"), nil
}
