package main

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type URL struct {
    ID        primitive.ObjectID `bson:"_id,omitempty"`
    ShortID   string             `bson:"shortID"`
    CreatedAt time.Time          `bson:"createdAt"`
    Dest      string             `bson:"dest"`
}