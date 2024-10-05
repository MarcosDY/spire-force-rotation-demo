package main

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	_ "github.com/lib/pq"
)

type ListResponse struct {
	Customers []*Customer `json:"customers"`
}

type Customer struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type Handler struct {
	connStr   string
	agentSock string
	log       *slog.Logger
}

func NewHandler(connStr string, agentSock string, log *slog.Logger) *Handler {
	return &Handler{
		connStr:   connStr,
		agentSock: agentSock,
		log:       log,
	}
}

func (h *Handler) CustomersList(w http.ResponseWriter, r *http.Request) {
	h.log.Info("List customers called...")
	if r.Method != http.MethodGet {
		h.log.Error("Invalid http method", "method", r.Method)
		http.Error(w, "unexpected http method", http.StatusInternalServerError)
		return
	}

	// open database
	db, err := sql.Open("postgres", h.connStr)
	if err != nil {
		h.log.Error("Failed to connect database", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// close database
	defer db.Close()

	rows, err := db.Query(`SELECT "name", "address" FROM "customers"`)
	if err != nil {
		h.log.Error("Error executing query", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	listResp := new(ListResponse)
	for rows.Next() {
		customer := &Customer{}
		if err := rows.Scan(&customer.Name, &customer.Address); err != nil {
			h.log.Error("Error retrieving customer", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		listResp.Customers = append(listResp.Customers, customer)
	}

	if err := rows.Err(); err != nil {
		h.log.Error("Error iterating over rows", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(listResp); err != nil {
		h.log.Error("Error processing payload", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) CustomerInsert(w http.ResponseWriter, r *http.Request) {
	h.log.Info("Insert customers called...")
	if r.Method != http.MethodPost {
		h.log.Error("Invalid http method", "method", r.Method)
		http.Error(w, "unexpected http method", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	var customer Customer
	if err := json.NewDecoder(r.Body).Decode(&customer); err != nil {
		h.log.Error("Failed to decode request", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// open database
	db, err := sql.Open("postgres", h.connStr)
	if err != nil {
		h.log.Error("Failed to connect database", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// close database
	defer db.Close()

	insert := `INSERT INTO "customers"("name", "address") VALUES($1, $2)`
	if _, err := db.Exec(insert, customer.Name, customer.Address); err != nil {
		h.log.Error("Failed to insert customer", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}
