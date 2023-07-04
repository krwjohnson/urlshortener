package main

import (
	"context"
	"html/template"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID       primitive.ObjectID `bson:"_id,omitempty"`
	Password string             `bson:"password"`
	Email    string             `bson:"email"`
	History  []primitive.ObjectID  `bson:"history"` // new field for history
}

func RegisterHandler(collection *mongo.Collection, store *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodGet {
            // Render the register.html template
            tmpl := template.Must(template.ParseFiles("./templates/register.html"))
            err := tmpl.Execute(w, nil)
            if err != nil {
                http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
                return
            }
            return
        }

        // Parse form data
        err := r.ParseForm()
        if err != nil {
            http.Error(w, "Error parsing form data", http.StatusInternalServerError)
            return
        }
        email := r.Form.Get("email")
        password := r.Form.Get("password")

        // Check if the email address is already registered
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        var result User
        err = collection.FindOne(ctx, bson.M{"email": email}).Decode(&result)
        if err == nil {
            http.Error(w, "Email address is already registered", http.StatusBadRequest)
            return
        } else if err != mongo.ErrNoDocuments {
            http.Error(w, "Error searching in database", http.StatusInternalServerError)
            return
        }

        // Hash the password
        hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
        if err != nil {
            http.Error(w, "Error hashing password", http.StatusInternalServerError)
            return
        }

        // Create a new user document
        newUser := &User{
            Password: string(hashedPassword),
            Email:    email,
        }

        // Insert the new user into the database
        _, err = collection.InsertOne(ctx, newUser)
        if err != nil {
            http.Error(w, "Error inserting user into database", http.StatusInternalServerError)
            return
        }

        // Set session values
        session, _ := store.Get(r, "urlshortener")
        session.Values["authenticated"] = true
        session.Values["email"] = email
        session.Save(r, w)

        // Redirect to the home page or another relevant page
        http.Redirect(w, r, "/home", http.StatusSeeOther)
    }
}


func DashboardHandler(userCollection *mongo.Collection, collection *mongo.Collection, store *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
    return func(w http.ResponseWriter, r *http.Request) {
        session, _ := store.Get(r, "urlshortener")
        if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
            http.Error(w, "Not authenticated", http.StatusUnauthorized)
            return
        }

        email := session.Values["email"].(string)
        var user User
        err := userCollection.FindOne(context.Background(), bson.M{"email": email}).Decode(&user)
        if err != nil {
            http.Error(w, "User not found", http.StatusNotFound)
            return
        }

        var urls []URL
        cursor, err := collection.Find(context.Background(), bson.M{"_id": bson.M{"$in": user.History}})
        if err != nil {
            http.Error(w, "Error retrieving URLs", http.StatusInternalServerError)
            return
        }
        if err = cursor.All(context.Background(), &urls); err != nil {
            http.Error(w, "Error decoding URLs", http.StatusInternalServerError)
            return
        }

        tmpl := template.Must(template.ParseFiles("./templates/dashboard.html"))
        data := struct {
            Email     string
            URLs         []URL
            Authenticated bool
        }{
            Email:     email,
            URLs:         urls,
            Authenticated: true,
        }
        tmpl.Execute(w, data)
    }
}


