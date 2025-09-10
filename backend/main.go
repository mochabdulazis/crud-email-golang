package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
)

var db *sql.DB

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func main() {
	// koneksi ke Postgres (samain dengan docker-compose)
	dsn := "postgres://admin:admin@db:5432/tugasdb?sslmode=disable"

	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}

	if err := db.Ping(); err != nil {
		log.Fatal("Gagal connect ke PostgreSQL:", err)
	}
	fmt.Println("✅ Connect PostgreSQL OK")

	// Routes API
	http.HandleFunc("/users", usersHandler)       // GET all, POST new
	http.HandleFunc("/users/", userDetailHandler) // GET/PUT/DELETE by id

	// Bungkus CORS biar frontend (file:// atau domain lain) bisa akses
	handler := withCORS(http.DefaultServeMux)

	fmt.Println("🚀 API jalan di http://localhost:8081")
	log.Fatal(http.ListenAndServe(":8081", handler))
}

// CORS middleware simple
func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// /users
func usersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		rows, err := db.Query(`SELECT id, name, email FROM users ORDER BY id`)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var users []User
		for rows.Next() {
			var u User
			if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			users = append(users, u)
		}
		json.NewEncoder(w).Encode(users)

	case http.MethodPost:
		var u User
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		err := db.QueryRow(
			`INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id`,
			u.Name, u.Email,
		).Scan(&u.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict) // bisa conflict kalau email duplikat
			return
		}
		json.NewEncoder(w).Encode(u)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// /users/{id}
func userDetailHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 {
		http.Error(w, "ID tidak ditemukan", http.StatusBadRequest)
		return
	}
	id, err := strconv.Atoi(parts[1])
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		var u User
		err := db.QueryRow(`SELECT id, name, email FROM users WHERE id=$1`, id).
			Scan(&u.ID, &u.Name, &u.Email)
		if err == sql.ErrNoRows {
			http.Error(w, "User tidak ditemukan", http.StatusNotFound)
			return
		} else if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(u)

	case http.MethodPut:
		var u User
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_, err := db.Exec(`UPDATE users SET name=$1, email=$2 WHERE id=$3`, u.Name, u.Email, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		u.ID = id
		json.NewEncoder(w).Encode(u)

	case http.MethodDelete:
		_, err := db.Exec(`DELETE FROM users WHERE id=$1`, id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"message": "User dihapus"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
