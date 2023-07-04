package main

import (
	"context"
	"html/template"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

var (
    key = []byte("super-secret-key")
    store = sessions.NewCookieStore(key)
)

func ProtectedEndpointHandler(w http.ResponseWriter, r *http.Request) {
    session, _ := store.Get(r, "urlshortener")
    // Check if user is authenticated
    if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }
    // Do something for authenticated users
}

func LoginHandler(collection *mongo.Collection, store *sessions.CookieStore) func(w http.ResponseWriter, r *http.Request) {
    return func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodGet {
            // Render the login form
            tmpl := template.Must(template.ParseFiles("./templates/login.html"))
            err := tmpl.Execute(w, nil)
            if err != nil {
                http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
                return
            }
        } else if r.Method == http.MethodPost {
            // Parse form data
            r.ParseForm()
            email := r.Form.Get("email")
            password := r.Form.Get("password")

            // Check if the email address exists in the database
            ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
            defer cancel()
            var result User
            err := collection.FindOne(ctx, bson.M{"email": email}).Decode(&result)
            if err != nil {
                http.Error(w, "Incorrect email or password", http.StatusBadRequest)
                return
            }

            // Compare the provided password with the stored hashed password
            err = bcrypt.CompareHashAndPassword([]byte(result.Password), []byte(password))
            if err == bcrypt.ErrMismatchedHashAndPassword {
                http.Error(w, "Incorrect email or password", http.StatusBadRequest)
                return
            } else if err != nil {
                http.Error(w, "Error comparing passwords", http.StatusInternalServerError)
                return
            }

            // Set session values
            session, _ := store.Get(r, "urlshortener")
            session.Values["authenticated"] = true
            session.Values["email"] = email
            session.Save(r, w)

            // Redirect to the home page
            http.Redirect(w, r, "/home", http.StatusSeeOther)
        }
    }
}



func LogoutHandler(w http.ResponseWriter, r *http.Request) {
    // Clear the session
    session, _ := store.Get(r, "urlshortener")
    session.Values["authenticated"] = false
    session.Values["email"] = ""
    session.Save(r, w)

    // Redirect to home page
    http.Redirect(w, r, "/home", http.StatusSeeOther)
}