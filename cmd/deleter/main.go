package main

import (
	"log"
	"flag"
	"context"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
	"os"
	"encoding/json"
	"io/ioutil"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"golang.org/x/oauth2"
	"net/http"
	"fmt"
	"sync"

	"gmail-deleter/internal"
	"gmail-deleter/internal/database"
)

func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
			tok = getTokenFromWeb(config)
			saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
			return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
			"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
			log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
			log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
			log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func main() {
	gmailClientSecret := flag.String("client-secret", "client_secret.json", "Google API client secret JSON file, downloaded from \n https://console.developers.google.com/apis/api/gmail.googleapis.com/credentials")
	mongoConnectionString := flag.String("mongo", "mongodb://localhost:27017", "MongoDB connection string")
	analyze := flag.Bool("download", false, "Download metadata from gmail and catalog inbox")
	report := flag.Bool("report", false, "Show summary of emails")
	consumers := flag.Int("workers", 1, "Number of simultaneous email processor workers")
	toDelete := flag.String("delete-from", "", "Delete all emails from this address")

	flag.Parse()

	ctx := context.TODO()
	mongoClient, err := mongo.Connect(
		ctx,
		options.Client().ApplyURI(*mongoConnectionString),
	)
	if (err != nil) {
		log.Fatal(err)
	}
	defer mongoClient.Disconnect(ctx)

	db := database.Db{}
	db.MongoClient = mongoClient
	d := mongoClient.Database("gmail_deleter")
	threadsCollection := d.Collection("threads")
	windowsCollection := d.Collection("windows")

	mod := mongo.IndexModel{
		Keys: bson.M{
			"id": 1,
		},
		Options: options.Index().SetUnique(true),
	}
	_, err = threadsCollection.Indexes().CreateOne(ctx, mod)
	if (err != nil) {
		log.Fatalf("Unable to create index: %v", err)
	}

	mod = mongo.IndexModel{
		Keys: bson.M{
			"window_name": 1,
			"ts": 1,
		},
		Options: options.Index().SetUnique(true),
	}
	_, err = windowsCollection.Indexes().CreateOne(ctx, mod)
	if (err != nil) {
		if database.IndexExists(err) == false {
			log.Fatalf("Unable to create index: %v", err)
		}
	}

	b, err := ioutil.ReadFile(*gmailClientSecret)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	 // If modifying these scopes, delete your previously saved token.json.
	gmailConfig, err := google.ConfigFromJSON(b, gmail.GmailModifyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	gmailClient := getClient(gmailConfig)

	srv, err := gmail.New(gmailClient)
	if err != nil {
		log.Fatalf("Unable to retrieve Gmail client: %v", err)
	}

	var wg sync.WaitGroup

	if (*analyze) {
		internal.ListThreads(srv, db)

		wg.Add(*consumers)
		for tid := 0; tid < *consumers; tid++ {
			go internal.FetchEmailWorker(tid, &wg, srv, db)
		}
	} else if (*report) {
		internal.Summarize(db)
	} else if (*toDelete != "") {
		wg.Add(*consumers)
		for tid := 0; tid < *consumers; tid++ {
			go internal.DeleteEmailWorker(tid, &wg, srv, db, *toDelete)
		}
	}

	wg.Wait()
}