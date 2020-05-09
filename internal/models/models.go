package models

import "time"

type Thread struct {
	Id string `bson:id`
	Status string `bson:status`
	From string `bson:from`
	To string `bson:to`
	Created time.Time `bson:created`
}

type Report struct {
	From string `bson:"_id"`
	Count int `bson:"count"`
}