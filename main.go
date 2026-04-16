package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	_ "github.com/go-sql-driver/mysql"
)

// from sql database
var db *sql.DB

// secret key
var jwtKey = []byte("secret_key")

// mutex
var bookingMutex sync.Mutex
var rateLimiterMutex sync.Mutex

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

type Event struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Quota       int    `json:"quota"`
}

type Booking struct {
	EventID int `json:"event_id"`
}

// WORKER POOL
type Job struct {
	Username string
	EventID int
}

var jobQueue = make(chan Job, 100)

func worker(id int) {
	for job := range jobQueue {
		log.Printf("Worker %d processing booking: user=%s event=%d\n", id, job.Username, job.EventID)
	}
}

// Rate Limiter
var rateLimiter = make(map[string]int)

func rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := strings.Split(r.RemoteAddr, ":")[0]

		rateLimiterMutex.Lock()
		rateLimiter[ip]++
		count := rateLimiter[ip]
		rateLimiterMutex.Unlock()

		if rateLimiter[ip] > 5 {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next(w, r)
	}
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

func eventsHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)

	if r.Method != http.MethodGet {
		http.Error(w, "Invalid Method!", http.StatusMethodNotAllowed)
		return
	}

	rows, err := db.Query("SELECT id, name, quota FROM events")

	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Close the rows
	defer rows.Close()
	
	var events []Event

	for rows.Next() {
		var e Event
		rows.Scan(&e.ID, &e.Name, &e.Quota)
		events = append(events, e)
	}

	json.NewEncoder(w).Encode(events)
}

func bookingHandler (w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid Method!", http.StatusMethodNotAllowed)
		return
	}

	username := r.Header.Get("username")

	var booking Booking
	err := json.NewDecoder(r.Body).Decode(&booking)
	if err != nil {
		http.Error(w, "Invalid JSON!", http.StatusBadRequest)
		return
	}

	bookingMutex.Lock()
	defer bookingMutex.Unlock()

	if booking.EventID == 0 {
		http.Error(w, "Event ID required", http.StatusBadRequest)
		return
	}

	// Check duplicate booking
	var exists int
	err = db.QueryRow("SELECT COUNT(*) FROM bookings WHERE username = ? AND event_id = ?", username, booking.EventID).Scan(&exists)

	if exists > 0 {
		http.Error(w, "Duplicate booking", http.StatusBadRequest)
		return
	}

	// Check quota
	var quota int
	err = db.QueryRow("SELECT quota FROM events WHERE id = ?", booking.EventID).Scan(&quota)

	if err != nil {
		http.Error(w, "Event not found", http.StatusBadRequest)
		return
	}

	var booked int
	err = db.QueryRow("SELECT COUNT(*) FROM bookings WHERE event_id = ?", booking.EventID).Scan(&booked)

	if booked >= quota {
		http.Error(w, "Quota Full", http.StatusBadRequest)
		return
	}

	// Insert booking
	_, err = db.Exec("INSERT INTO bookings (username, event_id) VALUES (?, ?)", username, booking.EventID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	jobQueue <- Job{Username: username, EventID: booking.EventID}

	json.NewEncoder(w).Encode(map[string]string{
		"message": "Booking successful",
	})
}

func historyHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(&w)

	if r.Method != http.MethodGet {
		http.Error(w, "Invalid Method!", http.StatusMethodNotAllowed)
		return
	}

	username := r.Header.Get("username")

	rows, err := db.Query(`SELECT e.id, e.name FROM bookings b JOIN events e ON b.event_id = e.id WHERE b.username = ?`, username)

	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Close the rows
	defer rows.Close()
	
	var events []Event

	for rows.Next() {
		var e Event
		rows.Scan(&e.ID, &e.Name)
		events = append(events, e)
	}

	json.NewEncoder(w).Encode(events)
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

		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Store username in header
		r.Header.Set("username", claims["username"].(string))

		next(w, r)
	}
}

func enableCORS(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func main() {

	for i := 1; i <= 3; i++ {
		go worker(i)
	}

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
	http.HandleFunc("/login", rateLimitMiddleware(loginHandler))
	http.HandleFunc("/success", authMiddleware(successHandler))
	http.HandleFunc("/events", eventsHandler)
	http.HandleFunc("/booking", rateLimitMiddleware(authMiddleware(bookingHandler)))
	http.HandleFunc("/history", authMiddleware(historyHandler))

	fmt.Println("Server running on :8080")
	http.ListenAndServe(":8080", nil)
}