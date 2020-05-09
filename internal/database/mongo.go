package database

import (
	"context"
	"errors"
	"log"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"

	"gmail-deleter/internal/models"
)

type Db struct {
	MongoClient *mongo.Client
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

func (db Db) Summarize() []models.Report {
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
	//report := []models.Report

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

func (db Db) Create(thread models.Thread) (error) {
	database := db.MongoClient.Database("gmail_deleter")
	threadsCollection := database.Collection("threads")
	_, err := threadsCollection.InsertOne(context.TODO(), thread)
	return err
}

func (db Db) Populate(thread models.Thread) (error) {
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


func (db Db) DeleteOne(tid string) {
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

func (db Db) FindOne(criteria bson.M, newStatus string) (thread models.Thread) {
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