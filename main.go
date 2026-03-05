package main

import (
	"fmt"
	"encoding/json"
	"log"
	"net/http"
	"database/sql"
	"strings"
	"regexp"

	_ "github.com/go-sql-driver/mysql"
)

type Contact struct {
	Name string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	OrderNumber string `json:"order_number"`
	Message string `json:"message"`
}

var db *sql.DB

func contactHandler(w http.ResponseWriter, r *http.Request){

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid Method!", http.StatusMethodNotAllowed)
		return
	}

	var contact Contact

	err := json.NewDecoder(r.Body).Decode(&contact)

	if err != nil {
		http.Error(w, "Invalid JSON!", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(contact.Name) == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	if len(name) < 3 {
		http.Error(w, "Name must be at least 3 characters", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(contact.Email) == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(contact.Message) == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	emailValidation := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)

	if !emailValidation.MatchString(contact.Email) {
		http.Error(w, "Invalid email format", http.StatusBadRequest)
		return
	}

	if contact.Phone != "" {

		phoneValidation := regexp.MustCompile(`^[0-9]+$`)

		if !phoneValidation.MatchString(contact.Phone) {
			http.Error(w, "Phone must contain numbers only", http.StatusBadRequest)
			return
		}
	}

	query := "INSERT INTO contacts (name, email, phone, order_number, message) VALUES (?, ?, ?, ?, ?)"

	_, err = db.Exec(query, contact.Name, contact.Email, contact.Phone, contact.OrderNumber, contact.Message)

	if err != nil {
		http.Error(w, "Failed to save contact", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]string{
		"message": "Contact saved successfully",
	})
}

func main(){

	var err error

	// Database connection
	dsn := "root:@tcp(127.0.0.1:3306)/midterm"

	db, err = sql.Open("mysql", dsn)

	if err != nil {
		log.Fatal("Database connection failed:", err)
	}

	err = db.Ping()

	if err != nil {
		log.Fatal("Database not responding:", err)
	}

	fmt.Println("Database connection successful")

	http.HandleFunc("/contact", contactHandler)

	http.ListenAndServe(":8080", nil)
}