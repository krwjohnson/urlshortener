package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"html/template"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)


func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal(err)
	}
	collection := client.Database("urlshortener").Collection("urls")
	userCollection := client.Database("urlshortener").Collection("users")

	fs := http.FileServer(http.Dir("./static"))

	r := mux.NewRouter()
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))
	r.HandleFunc("/home", HomeHandler(collection)).Methods("GET", "POST")
	r.HandleFunc("/create", CreateHandler(collection, userCollection, store)).Methods("POST")
	r.HandleFunc("/api/register", RegisterHandler(userCollection, store)).Methods("GET", "POST")
	r.HandleFunc("/api/login", LoginHandler(userCollection, store)).Methods("GET", "POST")
	r.HandleFunc("/api/protected-endpoint", ProtectedEndpointHandler).Methods("GET")
	r.HandleFunc("/api/logout", LogoutHandler).Methods("POST")
	r.HandleFunc("/dashboard", DashboardHandler(userCollection, collection, store)).Methods("GET")
	r.HandleFunc("/{id:[a-zA-Z0-9]+}", RedirectHandler(collection)).Methods("GET")

	// Add redirect from "/" to "/home"
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/home", http.StatusSeeOther)
	})

	log.Fatal(http.ListenAndServe(":8080", r))
}

func HomeHandler(collection *mongo.Collection) func(w http.ResponseWriter, r *http.Request) {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "urlshortener")
        authenticated := false
        email := ""
        if auth, ok := session.Values["authenticated"].(bool); ok && auth {
            authenticated = true
            email = session.Values["email"].(string)
        }

        tmpl := template.Must(template.ParseFiles("./templates/index.html"))
        data := struct {
            Email         string
            Authenticated bool
            ShortURL      string
        }{
            Email:         email,
            Authenticated: authenticated,
            ShortURL:      "",
        }

        err := tmpl.Execute(w, data)
        if err != nil {
            http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
            return
        }
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

        err := collection.FindOne(ctx, bson.M{"shortID": id}).Decode(&result)
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

func CreateHandler(collection *mongo.Collection, userCollection *mongo.Collection, store *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
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

        var id string
        if customURL != "" {
            err := collection.FindOne(ctx, bson.M{"id": customURL}).Decode(&result)
            if err != mongo.ErrNoDocuments {
                http.Error(w, "Custom URL is already in use", http.StatusBadRequest)
                return
            }
            id = customURL
        } else {
            for {
                id, _ = generateID()
                err := collection.FindOne(ctx, bson.M{"id": id}).Decode(&result)
                if err == mongo.ErrNoDocuments {
                    break
                }
            }
        }

        newURL := &URL{
            ID:        primitive.NewObjectID(),
            ShortID:   id,
            CreatedAt: time.Now(),
            Dest:      url,
        }

        _, err := collection.InsertOne(ctx, newURL)
        if err != nil {
            http.Error(w, "Error inserting URL into database", http.StatusInternalServerError)
            return
        }

        // Add the new URL to the user's history
        session, _ := store.Get(r, "urlshortener")
        if auth, ok := session.Values["authenticated"].(bool); ok && auth {
            email := session.Values["email"].(string)
            var user User
            err := userCollection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
            if err != nil {
                http.Error(w, "Error finding user in database", http.StatusInternalServerError)
                return
            }
            user.History = append(user.History, newURL.ID)
            _, err = userCollection.UpdateOne(ctx, bson.M{"email": email}, bson.M{"$set": bson.M{"history": user.History}})
            if err != nil {
                http.Error(w, "Error updating user history in database", http.StatusInternalServerError)
                return
            }
        }
        
        email := ""
        authenticated := false
        if auth, ok := session.Values["authenticated"].(bool); ok && auth {
            email = session.Values["email"].(string)
            authenticated = true
        }

        data := struct {
            ShortURL      string
            Email         string
            Authenticated bool
        }{
            ShortURL:      "http://localhost:8080/" + newURL.ShortID,
            Email:         email,
            Authenticated: authenticated,
        }

        err = tmpl.Execute(w, data)
        if err != nil {
            http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
            return
        }
    }
}
