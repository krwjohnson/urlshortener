package main

import (
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
	collection := client.Database("test").Collection("urls")

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", HomeHandler(collection))
	http.HandleFunc("/create", CreateHandler(collection))

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func HomeHandler(collection *mongo.Collection) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles("./templates/index.html"))
		tmpl.Execute(w, nil)
	}
}


// func connectDB() (*mongo.Collection, error) {
// 	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")

// 	client, err := mongo.Connect(context.TODO(), clientOptions)
// 	if err != nil {
// 		return nil, err
// 	}

// 	err = client.Ping(context.TODO(), nil)
// 	if err != nil {
// 		return nil, err
// 	}

// 	collection := client.Database("urlshortener").Collection("urls")

// 	return collection, nil
// }

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

func CreateHandler(collection *mongo.Collection) func(w http.ResponseWriter, r *http.Request) {
    return func(w http.ResponseWriter, r *http.Request) {
        tmpl := template.Must(template.ParseFiles("templates/index.html"))
        r.ParseForm()
        url := r.Form.Get("url")

        if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
            url = "http://" + url
        }

        var result URL
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        err := collection.FindOne(ctx, bson.M{"dest": url}).Decode(&result)

        var id string
        if err == mongo.ErrNoDocuments {
            // URL does not exist in the database, so we generate a new short ID and create it.
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
                    http.Error(w, "Error searching in database", http.StatusInternalServerError)
                    return
                }
                // This ID exists in the database, so we continue the loop to generate a new one.
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
            http.Error(w, "Error searching in database", http.StatusInternalServerError)
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


