package main

import (
	"fmt"
	"log"
	"time"
	"net/http"
	"strings"
	"github.com/gorilla/mux"
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"crypto/rand"
	"encoding/hex"
	"html/template"
)

func main() {
	collection, err := connectDB()
	if err != nil {
		log.Fatal(err)
	}

	r := mux.NewRouter()

	r.HandleFunc("/", HomeHandler(collection)).Methods("GET", "POST")
	r.HandleFunc("/{id}", RedirectHandler(collection))

	http.Handle("/", r)
	fmt.Println("Server is listening...")
	http.ListenAndServe(":8080", r)
}

func HomeHandler(collection *mongo.Collection) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles("index.html"))
		if r.Method == "POST" {
			r.ParseForm()
			url := r.Form.Get("url")

			if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
				url = "http://" + url
			}

			id, err := generateID()
			if err != nil {
				http.Error(w, "Error generating ID", http.StatusInternalServerError)
				return
			}

			newURL := &URL{
				ID:        id,
				CreatedAt: time.Now(),
				Dest:      url,
			}

			_, err = collection.InsertOne(context.Background(), newURL)
			if err != nil {
				http.Error(w, "Error inserting URL into database", http.StatusInternalServerError)
				return
			}

			shortURL := "http://localhost:8080/" + id
			tmpl.Execute(w, struct{ ShortURL string }{shortURL})

		} else {
			tmpl.Execute(w, nil)
		}
	}
}

func connectDB() (*mongo.Collection, error) {
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")

	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		return nil, err
	}

	err = client.Ping(context.TODO(), nil)
	if err != nil {
		return nil, err
	}

	collection := client.Database("urlshortener").Collection("urls")

	return collection, nil
}

func generateID() (string, error) {
	b := make([]byte, 4) // Change this to increase/decrease ID length
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func RedirectHandler(collection *mongo.Collection) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]

		var url URL
		err := collection.FindOne(context.TODO(), bson.M{"_id": id}).Decode(&url)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				http.Error(w, "No short URL found for given ID", http.StatusNotFound)
			} else {
				http.Error(w, "Error finding short URL in database", http.StatusInternalServerError)
			}
			return
		}

		http.Redirect(w, r, url.Dest, http.StatusSeeOther)
	}
}