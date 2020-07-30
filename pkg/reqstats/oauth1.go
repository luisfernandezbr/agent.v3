package reqstats

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/dghubble/oauth1"
)

func OAuth1HTTPClient(url, consumerKey, token string) (*http.Client, error) {
	ctx := context.Background()
	keyDERBlock, _ := pem.Decode([]byte(os.Getenv("PP_JIRA_PRIVATE_KEY")))
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

	tok := &oauth1.Token{Token: token, TokenSecret: os.Getenv("PP_JIRA_TOKEN_SECRET")}

	return config.Client(ctx, tok), nil

}
