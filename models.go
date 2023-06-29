package main

import (
	"time"
)

// URL represents url document in MongoDB
type URL struct {
	ID        string    `bson:"_id,omitempty"`
	CreatedAt time.Time `bson:"created_at,omitempty"`
	Dest      string    `bson:"dest,omitempty"`
}
