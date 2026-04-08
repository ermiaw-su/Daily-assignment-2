package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	_ "github.com/go-sql-driver/mysql"
)

// from sql database
var db *sql.DB

// secret key
var jwtKey = []byte("secret_key")

// Model

type Contact struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	OrderNumber string `json:"order_number"`
	Message     string `json:"message"`
}

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Authentication Handlers

func registerHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid Method!", http.StatusMethodNotAllowed)
		return
	}

	var user User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, "Invalid JSON!", http.StatusBadRequest)
		return
	}

	// Validation
	if user.Username == "" || user.Password == "" {
		http.Error(w, "Username & Password required", http.StatusBadRequest)
		return
	}


	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Error hashing password", http.StatusInternalServerError)
		return
	}

	// Insert the user into the database
	query := "INSERT INTO users (username, password) VALUES (?, ?)"
	_, err = db.Exec(query, user.Username, hashedPassword)
	if err != nil {
		http.Error(w, "User already exists / DB error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"message": "User registered successfully",
	})
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid Method!", http.StatusMethodNotAllowed)
		return
	}

	var user User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, "Invalid JSON!", http.StatusBadRequest)
		return
	}

	var storedPassword string
	query := "SELECT password FROM users WHERE username = ?"
	err = db.QueryRow(query, user.Username).Scan(&storedPassword)

	if err != nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	// Compare the password
	err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(user.Password))
	if err != nil {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	// Generate JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": user.Username,
	})

	// Sign the token
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	// Send the token
	json.NewEncoder(w).Encode(map[string]string{
		"token": tokenString,
	})
}


// Success Handler
func successHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]string{
		"message": "Congrats, you successfully logged in!",
	})
}

// Middleware

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		enableCORS(&w)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		authHeader := r.Header.Get("Authorization")

		if authHeader == "" {
			http.Error(w, "Missing token", http.StatusUnauthorized)
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

func enableCORS(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func main() {

	var err error

	dsn := "root:@tcp(127.0.0.1:3306)/midterm"
	db, err = sql.Open("mysql", dsn)

	if err != nil {
		log.Fatal("Database connection failed:", err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal("Database not responding:", err)
	}

	fmt.Println("Database connected!")

	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/success", authMiddleware(successHandler))

	fmt.Println("Server running on :8080")
	http.ListenAndServe(":8080", nil)
}