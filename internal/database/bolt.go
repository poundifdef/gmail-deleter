package database

import (
	"fmt"
	"log"
	"time"
	"encoding/binary"
	"errors"
	"sort"
	"go.mongodb.org/mongo-driver/bson"
	"gmail-deleter/internal/models"
	bolt "go.etcd.io/bbolt"
)

type BoltDB struct {
	Filename string
	Client *bolt.DB
}

func (db *BoltDB) Init() {
	client, err := bolt.Open(db.Filename, 0666, nil)
	if err != nil {
		panic(err)
	}
	db.Client = client

	err = client.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("NEW"))
		if err != nil {
			panic(err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte("FETCHING_THREAD"))
		if err != nil {
			panic(err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte("FETCHED"))
		if err != nil {
			panic(err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte("DELETING"))
		if err != nil {
			panic(err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte("COUNTERS"))
		if err != nil {
			panic(err)
		}

		return nil
	})
}

func (db *BoltDB) Close() {
	db.Client.Close()
}

func (db BoltDB) ReserveWindow(cost int) bool {
	MAX_PER_DAY := uint64(1_000_000_000)
	MAX_USER_PER_SECOND := uint64(150)  // Gmail has a max of 250 for this
	
	now := time.Now()
	today := []byte(now.Truncate(24*time.Hour).String())
	this_second := []byte(now.Truncate(1*time.Second).String())

	google_key := append([]byte("GOOGLE"), today...)
	user_key := append([]byte("USER"), this_second...)

	err := db.Client.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("COUNTERS"))

		google_quota_bytes := b.Get(google_key)
		google_quota := uint64(0)
		if (google_quota_bytes != nil) {
			google_quota = uint64(binary.LittleEndian.Uint64(google_quota_bytes))
		}
		google_quota += uint64(cost)
		// log.Println("google", google_quota)
		if (google_quota > MAX_PER_DAY) {
			return errors.New("Reached max google per day")
		} else {
			bin := make([]byte, 8)
			binary.LittleEndian.PutUint64(bin, uint64(google_quota))
			b.Put(google_key, bin)
		}

		user_quota_bytes := b.Get(user_key)
		user_quota := uint64(0)
		if (user_quota_bytes != nil) {
			user_quota = uint64(binary.LittleEndian.Uint64(user_quota_bytes))
		}
		user_quota += uint64(cost)
		// log.Println("user", user_quota)
		if (user_quota > MAX_USER_PER_SECOND) {
			return errors.New("Reached max user per second")
		} else {
			bin := make([]byte, 8)
			binary.LittleEndian.PutUint64(bin, uint64(user_quota))
			b.Put(user_key, bin)
		}

		return nil
	})

	return err == nil //under_global_quota && under_user_quota
}

func (db BoltDB) Summarize() []models.Report {
	m := make(map[string]int)

	db.Client.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("FETCHED"))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			//fmt.Printf("key=%s, value=%s\n", k, v)
			var thread models.Thread
			thread.FromBytes(v)
			m[thread.From] += 1
		}

		return nil
	})

	keys := make([]string, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}

	sort.Slice(keys, func(i, j int) bool {
		return m[keys[i]] > m[keys[j]]
	})

	numRecords := len(keys)
	if numRecords > 100 {
		numRecords = 100
	}
	report := make([]models.Report, numRecords)

	for i=0; i < numRecords; i++ {
		var r models.Report
		r.From = keys[i]
		r.Count = m[r.From]
		report[i] = r
	}

	return report
}

func (db BoltDB) Create(thread models.Thread) (error) {
	err := db.Client.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("NEW"))
		key := []byte(thread.Id)
		value := thread.ToBytes()
		err := b.Put(key, value)
		return err
	})

	return err
}

func (db BoltDB) Populate(thread models.Thread) (error) {
	db.Client.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("FETCHING_THREAD"))
		key := []byte(thread.Id)
		itemBytes := b.Get(key)

		if (itemBytes != nil) {
			newStatus := thread.Status
			newBucket := tx.Bucket([]byte(newStatus))
			newBucket.Put(key, thread.ToBytes())
			b.Delete(key)
			//log.Println(thread)
		}

		return nil
	})
	return nil
}

func (db BoltDB) DeleteOne(tid string) {
	err := db.Client.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("FETCHED"))
		return b.Delete([]byte(tid))
	})
	if (err != nil) {
		log.Fatal(err)
	}
}

func (db BoltDB) FindOne(criteria bson.M, newStatus string) (thread models.Thread) {
	bucket := fmt.Sprintf("%v", criteria["status"])
	from := fmt.Sprintf("%v", criteria["from"])
	db.Client.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		c := b.Cursor()

		if (from == "<nil>") {
			k, v := c.First()
	
			if (k != nil) {
				thread.FromBytes(v)
	
				thread.Status = newStatus
				newBucket := tx.Bucket([]byte(newStatus))
				newBucket.Put(k, thread.ToBytes())
	
				b.Delete(k)
			}
		} else {
			for k, v := c.First(); k != nil; k, v = c.Next() {
				thread.FromBytes(v)
	
				if thread.From == from {
					//log.Println("match", from)
					thread.Status = newStatus
					newBucket := tx.Bucket([]byte(newStatus))
					newBucket.Put(k, thread.ToBytes())
					b.Delete(k)
					break
				} 

				thread = models.Thread{}
			}
		}

		return nil
	})

	return thread
}