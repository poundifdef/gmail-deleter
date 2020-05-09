package internal

import (
	"log"
	"fmt"
	"sync"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
	"go.mongodb.org/mongo-driver/bson"
	"strings"
	"time"

	"gmail-deleter/internal/database"
	"gmail-deleter/internal/models"
)

func Summarize(db database.Db) {
	fmt.Println("From,Count")

	report := db.Summarize()
	for _, r := range report {
		fmt.Print(r.From)
		fmt.Print(",")
		fmt.Print(r.Count)
		fmt.Println()
	}
}

func createThread(wg *sync.WaitGroup, t *gmail.Thread, db database.Db) {
	defer wg.Done()

	var thread models.Thread
	thread.Id = t.Id
	thread.Status = "NEW"

	err := db.Create(thread)
	if (database.IsDup(err)) {
		return
	}

	if (err != nil) {
		log.Fatal("Unable to retrieve threads: %v", err)
	}
}

func DeleteEmailWorker(tid int, wg *sync.WaitGroup, gmail *gmail.Service, db database.Db, from string) {
	defer wg.Done()
	for {
		thread := db.FindOne(bson.M{"status": "FETCHED", "from": from}, "DELETING")
		if (thread.Id == "") {
			log.Println(tid, "No threads to fetch")
			return
		}

		_, err := gmail.Users.Threads.
			Trash("me", thread.Id).
			Do()

		if (err == nil) {
			db.DeleteOne(thread.Id)
		} else {
			e, _ := err.(*googleapi.Error)
			if (e.Code == 404) {
				db.DeleteOne(thread.Id)
			} else {
				log.Fatalf("Could not trash thread", e)
			}
		}
	}
}

func parseEmail(e string) string {
	if strings.Contains(e, "<") && strings.Contains(e, ">") {
		f := func(c rune) bool {
			return (c == '<') || (c == '>')
		}
		tokens := strings.FieldsFunc(e, f)

		if len(tokens) == 1 {
			return tokens[0]
		}
		return tokens[1]
	}

	return e
}

func waitForWindow(cost int, db database.Db) {
	for {
		canProcess := db.ReserveWindow(cost)

		if canProcess {
			break
		}

		log.Println("Backing off", canProcess)
		time.Sleep(1 * time.Second)
	}
}

func FetchEmailWorker(tid int, wg *sync.WaitGroup, gmail *gmail.Service, db database.Db) {
	defer wg.Done()
	for {
		waitForWindow(10, db)

		thread := db.FindOne(bson.M{"status": "NEW"}, "FETCHING_THREAD")
		if (thread.Id == "") {
			log.Println(tid, "Finished deleting")
			return
		}

		r, err := gmail.Users.Threads.
			Get("me", thread.Id).
			Do()

		if (err != nil) {
			log.Fatal("Unable to get gmail thread", err)
		}

		message := r.Messages[0]
		thread.Created = time.Unix(message.InternalDate/1000, 0)

		messagePart := message.Payload
		headers := messagePart.Headers

		// TODO: filter out chats

		for _, h := range(headers) {
			if (strings.EqualFold(h.Name, "from")) {
				thread.From = parseEmail(strings.ToLower(h.Value))
			}
			if (strings.EqualFold(h.Name, "to")) {
				thread.To = parseEmail(strings.ToLower(h.Value))
			}

			// TODO: BCC, subject, snippet for richer searching
		}

		thread.Status = "FETCHED"

		db.Populate(thread)
	}

}

func ListThreads(gmail *gmail.Service, db database.Db) {
	var wg sync.WaitGroup

	user := "me"
	nextPageToken := ""

	for hasPage := true; hasPage; {
		waitForWindow(10, db)

		r, err := gmail.Users.Threads.
			List(user).
			MaxResults(500).
			PageToken(nextPageToken).
			Fields("nextPageToken", "threads/id").
			Do()

		if err != nil {
			log.Fatalf("Unable to retrieve threads: %v", err)
		}

		for _, l := range r.Threads {
			wg.Add(1)
			go createThread(&wg, l, db)
		}

		// TODO: save this token in case we want to resume
		nextPageToken = r.NextPageToken
		if (nextPageToken == "") {
			hasPage = false
		}
	} 

	wg.Wait()
}