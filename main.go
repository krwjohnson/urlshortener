package main

import (
	"fmt"
	"log"
	"time"
	"net/http"
	"strings"
	"math/big"
	"github.com/gorilla/mux"
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"crypto/rand"
	"html/template"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, _ := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	collection := client.Database("urlshortener").Collection("urls")

	fs := http.FileServer(http.Dir("./static"))

	r := mux.NewRouter()
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))
	r.HandleFunc("/create", CreateHandler(collection)).Methods("POST")
	r.HandleFunc("/home", HomeHandler(collection)).Methods("GET", "POST")
	r.HandleFunc("/{id:[a-zA-Z0-9]+}", RedirectHandler(collection)).Methods("GET")

	// Add redirect from "/" to "/home"
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request){
		http.Redirect(w, r, "/home", http.StatusSeeOther)
	})

	http.Handle("/", r)

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func HomeHandler(collection *mongo.Collection) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles("./templates/index.html"))
		tmpl.Execute(w, nil)
	}
}

func generateID() (string, error) {
    const charset = "abcdefghijklmnopqrstuvwxyz" + 
                    "ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
                    "0123456789"
    const idLength = 4

    b := make([]byte, idLength)
    for i := range b {
        num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
        if err != nil {
            return "", err
        }
        b[i] = charset[num.Int64()]
    }
    return string(b), nil
}


func RedirectHandler(collection *mongo.Collection) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		id := vars["id"]

		var result URL
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := collection.FindOne(ctx, bson.M{"_id": id}).Decode(&result)
		if err == mongo.ErrNoDocuments {
			fmt.Printf("Error: no document found for id: %s\n", id) // debug print
			http.Error(w, "No short URL found for given ID", http.StatusNotFound)
			return
		}
		if err != nil {
			fmt.Printf("Error: %v\n", err) // debug print
			http.Error(w, "Error finding short URL", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, result.Dest, http.StatusSeeOther)
	}
}


func CreateHandler(collection *mongo.Collection) func(w http.ResponseWriter, r *http.Request) {
    return func(w http.ResponseWriter, r *http.Request) {
        tmpl := template.Must(template.ParseFiles("templates/index.html"))
        r.ParseForm()
        url := r.Form.Get("url")
        customURL := r.Form.Get("customurl")

        if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
            url = "http://" + url
        }

        var result URL
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        err := collection.FindOne(ctx, bson.M{"dest": url}).Decode(&result)

        var id string
        if err == mongo.ErrNoDocuments {
            if customURL != "" {
				// Custom URL provided, check if it's already in use
				err = collection.FindOne(ctx, bson.M{"id": customURL}).Decode(&result)
				if err != nil && err != mongo.ErrNoDocuments {
					// A real error occurred while searching in the database
					http.Error(w, "Error searching in database: "+err.Error(), http.StatusInternalServerError)
					return
				}
				
				if err != mongo.ErrNoDocuments {
					// Custom URL is already in use, return an error
					http.Error(w, "Custom URL is already in use", http.StatusBadRequest)
					return
				}

				id = customURL
			} else {
                // Generate a new short ID
                for {
                    id, err = generateID()
                    if err != nil {
                        http.Error(w, "Error generating ID", http.StatusInternalServerError)
                        return
                    }

                    var existingURL URL
                    err = collection.FindOne(ctx, bson.M{"id": id}).Decode(&existingURL)

                    if err == mongo.ErrNoDocuments {
                        // This ID does not exist in the database, so we can use it.
                        break
                    }

                    if err != nil {
                        http.Error(w, "Error searching in database (2)", http.StatusInternalServerError)
                        return
                    }
                }
            }

            newURL := &URL{
                ID:        id,
                CreatedAt: time.Now(),
                Dest:      url,
            }

            _, err = collection.InsertOne(ctx, newURL)
            if err != nil {
                http.Error(w, "Error inserting URL into database", http.StatusInternalServerError)
                return
            }
        } else if err != nil {
            http.Error(w, "Error searching in database (3)", http.StatusInternalServerError)
            return
        } else {
            // URL already exists in the database, so we use its existing short ID.
            id = result.ID
        }

        data := struct {
            ShortURL string
        }{
            ShortURL: "http://localhost:8080/" + id,
        }

        tmpl.Execute(w, data)
    }
}
