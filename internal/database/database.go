package database

import (
	"go.mongodb.org/mongo-driver/bson"
	"gmail-deleter/internal/models"
)

type Database interface {
	Init()
	Close()
	ReserveWindow(int) bool
	Summarize() []models.Report 
	Create(models.Thread) error
	Populate(models.Thread) error
	DeleteOne(string)
	FindOne(bson.M, string) models.Thread
}