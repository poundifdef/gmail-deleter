package database

import (
	"context"
	"errors"
	"log"
	"time"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"gmail-deleter/internal/models"
)

type MongoDB struct {
	ConnectionString string
	MongoClient *mongo.Client
}

func (db *MongoDB) Init() {
	mongoClient, err := mongo.Connect(
		context.TODO(),
		options.Client().ApplyURI(db.ConnectionString),
	)
	if (err != nil) {
		log.Fatal(err)
	}
	db.MongoClient = mongoClient


	d := db.MongoClient.Database("gmail_deleter")
	threadsCollection := d.Collection("threads")
	windowsCollection := d.Collection("windows")

	//ctx := context.TODO()
	mod := mongo.IndexModel{
		Keys: bson.M{
			"id": 1,
		},
		Options: options.Index().SetUnique(true),
	}
	_, err = threadsCollection.Indexes().CreateOne(context.TODO(), mod)
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
	_, err = windowsCollection.Indexes().CreateOne(context.TODO(), mod)
	if (err != nil) {
		if IndexExists(err) == false {
			log.Fatalf("Unable to create index: %v", err)
		}
	}

}

func (db *MongoDB) Close() {
	db.MongoClient.Disconnect(context.TODO())
}

// error codes: https://github.com/mongodb/mongo/blob/master/src/mongo/base/error_codes.yml#L110
func IndexExists(err error) bool {
    var e mongo.CommandError
    if errors.As(err, &e) {
		if e.Code == 86 {
			return true
		}
    }
    return false
}

func IsDup(err error) bool {
    var e mongo.WriteException
    if errors.As(err, &e) {
        for _, we := range e.WriteErrors {
            if we.Code == 11000 {
                return true
            }
        }
    }
    return false
}

func (db MongoDB) ReserveWindow(cost int) bool {
	MAX_PER_DAY := 1_000_000_000
	MAX_USER_PER_SECOND := 150  // Gmail has a max of 250 for this

	now := time.Now()
	today := now.Truncate(24*time.Hour)
	this_second := now.Truncate(1*time.Second)

	database := db.MongoClient.Database("gmail_deleter")
	windowsCollection := database.Collection("windows")

	upsert := true
	after := options.After
	opt := options.FindOneAndUpdateOptions{
		ReturnDocument: &after,
		Upsert:         &upsert,
	}

	result := windowsCollection.FindOneAndUpdate(
		context.Background(),
		bson.M{
			"window_name": "GOOGLE",
			"ts": today,
			"count": bson.M{"$lt": MAX_PER_DAY},
		},
		bson.D{
			{"$inc", bson.M{
				"count": cost,
			}},
		},
		&opt,
	)
	if (result.Err() != nil) {
		return false
	}

	result = windowsCollection.FindOneAndUpdate(
		context.Background(),
		bson.M{
			"window_name": "USER",
			"ts": this_second,
			"count": bson.M{"$lt": MAX_USER_PER_SECOND},
		},
		bson.D{
			{"$inc", bson.M{
				"count": cost,
			}},
		},
		&opt,
	)
	if (result.Err() != nil) {
		return false
	}

	return true
}

func (db MongoDB) Summarize() []models.Report {
	summaryLimit := 100

	database := db.MongoClient.Database("gmail_deleter")
	threadsCollection := database.Collection("threads")

	matchStage := bson.D{{"$match", bson.D{{"status", "FETCHED"}}}}
	groupStage := bson.D{
		{
			"$group", bson.D{
				{"_id", "$from"},
				{"count", bson.D{{"$sum", 1}}},
			},
		},
	}
	sort := bson.D{
		{
			"$sort", bson.D{
				{"count", -1},
				{"_id", 1},
			},
		},
	}
	limit := bson.D{{"$limit", summaryLimit}}
	report := make([]models.Report, summaryLimit)

	summaryCursor, err := threadsCollection.Aggregate(
		context.TODO(),
		mongo.Pipeline{matchStage, groupStage, sort, limit},
	)

	if err != nil {
		log.Fatal("Could not generate report", err)
	}

	if err = summaryCursor.All(context.TODO(), &report); err != nil {
		panic(err)
	}
	return report
}

func (db MongoDB) Create(thread models.Thread) (error) {
	database := db.MongoClient.Database("gmail_deleter")
	threadsCollection := database.Collection("threads")
	_, err := threadsCollection.InsertOne(context.TODO(), thread)

	log.Println(thread, err)
	return err
}

func (db MongoDB) Populate(thread models.Thread) (error) {
	database := db.MongoClient.Database("gmail_deleter")
	threadsCollection := database.Collection("threads")
	upsert := false
	after := options.After
	opt := options.FindOneAndUpdateOptions{
		ReturnDocument: &after,
		Upsert:         &upsert,
	}

	result := threadsCollection.FindOneAndUpdate(
		context.Background(),
		bson.M{"id": thread.Id},
		bson.D{
			{"$set", bson.M{
				"status": thread.Status,
				"from": thread.From,
				"to": thread.To,
				"created": thread.Created,
			}},
		},
		&opt,
	)

	if result.Err() == mongo.ErrNoDocuments {
		log.Println("Unable to update thread", thread.Id)
		return nil
	} else if (result.Err() != nil) {
		log.Fatal("Unable to populate thread", result.Err())
	}

	return result.Err()
}


func (db MongoDB) DeleteOne(tid string) {
	database := db.MongoClient.Database("gmail_deleter")
	threadsCollection := database.Collection("threads")
	_, err := threadsCollection.DeleteOne(
		context.Background(),
		bson.M{"id": tid},
	)
	if (err != nil) {
		log.Fatal("Could not delete from mongo", tid)
	}
}

func (db MongoDB) FindOne(criteria bson.M, newStatus string) (thread models.Thread) {
	database := db.MongoClient.Database("gmail_deleter")
	threadsCollection := database.Collection("threads")

	upsert := false
	after := options.After
	opt := options.FindOneAndUpdateOptions{
		ReturnDocument: &after,
		Upsert:         &upsert,
	}
	result := threadsCollection.FindOneAndUpdate(
		context.Background(),
		criteria,
		bson.D{
			{"$set", bson.M{
				"status": newStatus,
			}},
		},
		&opt,
	)

	thread = models.Thread{}
	if result.Err() == mongo.ErrNoDocuments {
		return
	}

	err := result.Decode(&thread)
	if err != nil {
		log.Fatal("Could not decode object from mongo", err)
	}

	return
}