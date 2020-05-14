package models

import (
	"time"
	"encoding/gob"
	"bytes"
	"log"
)

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

func (t Thread) ToBytes() []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(t)
	if err != nil {
		log.Fatal("decode:", err)
	}

	return buf.Bytes()
}

func (t *Thread) FromBytes(b []byte) {
	network := bytes.NewBuffer(b)
	dec := gob.NewDecoder(network)
	err := dec.Decode(t)
	if err != nil {
		log.Fatal("encode:", err)
	}
}