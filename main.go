package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	_ "github.com/go-sql-driver/mysql"
)

// Настройки БД
const (
	DBHost = "127.0.0.1"
	DBPort = "3306"
	DBUser = "root"
	DBPass = "root"
	DBName = "carwash_db"
)

var db *sql.DB

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	Role     string `json:"role"`
	Name     string `json:"name"`
	IsActive int    `json:"is_active"`
}

type Wash struct {
	ID          int     `json:"id"`
	UserID      int     `json:"user_id"`
	CarModel    string  `json:"car_model"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	CreatedAt   string  `json:"created_at"`
	DateOnly    string  `json:"date_only"`
	WorkerName  string  `json:"worker_name,omitempty"`
}

type Expense struct {
	ID       int     `json:"id"`
	Title    string  `json:"title"`
	Amount   float64 `json:"amount"`
	DateOnly string  `json:"date_only"`
}

type Payload struct {
	Action string `json:"-"`

	// User fields
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
	Name     string `json:"name"`
	IsActive *int   `json:"is_active"`

	// Common ID (User ID or Wash ID or Expense ID)
	ID int `json:"id"`

	// Wash / Expense fields
	UserID      int     `json:"user_id"`
	CarModel    string  `json:"car_model"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Title       string  `json:"title"`
	Amount      float64 `json:"amount"`
	Date        string  `json:"date"`

	// Report filters
	Start string `json:"start"`
	End   string `json:"end"`
}

func main() {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4", DBUser, DBPass, DBHost, DBPort, DBName)
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatal("Ошибка БД:", err)
	}

	http.HandleFunc("/api", apiHandler)
	fmt.Println("Server running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func apiHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")

	if r.Method == "OPTIONS" {
		return
	}

	action := r.URL.Query().Get("action")
	var p Payload
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&p)
	}

	switch action {
	case "login":
		handleLogin(w, p)
	case "get_users":
		handleGetUsers(w)
	case "save_user":
		handleSaveUser(w, p)
	case "delete_user":
		handleDeleteUser(w, p)

	case "add_wash":
		handleAddWash(w, p)
	case "edit_wash":
		handleEditWash(w, p) // НОВОЕ
	case "delete_wash":
		handleDeleteWash(w, p) // НОВОЕ

	case "add_expense":
		handleAddExpense(w, p)
	case "get_report":
		handleGetReport(w, p)
	default:
		json.NewEncoder(w).Encode(map[string]string{"error": "Unknown action"})
	}
}

// --- Handlers ---

func handleLogin(w http.ResponseWriter, p Payload) {
	var u User
	err := db.QueryRow("SELECT id, username, password, role, name, is_active FROM users WHERE username = ? AND is_active = 1", p.Username).Scan(&u.ID, &u.Username, &u.Password, &u.Role, &u.Name, &u.IsActive)
	if err != nil || u.Password != p.Password {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Неверно"})
		return
	}
	u.Password = ""
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "user": u})
}

func handleGetUsers(w http.ResponseWriter) {
	rows, _ := db.Query("SELECT id, username, role, name, is_active FROM users ORDER BY is_active DESC, name ASC")
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		rows.Scan(&u.ID, &u.Username, &u.Role, &u.Name, &u.IsActive)
		users = append(users, u)
	}
	if users == nil {
		users = []User{}
	}
	json.NewEncoder(w).Encode(users)
}

func handleSaveUser(w http.ResponseWriter, p Payload) {
	active := 1
	if p.IsActive != nil {
		active = *p.IsActive
	}
	var err error
	if p.ID > 0 {
		if p.Password != "" {
			_, err = db.Exec("UPDATE users SET username=?, role=?, name=?, is_active=?, password=? WHERE id=?", p.Username, p.Role, p.Name, active, p.Password, p.ID)
		} else {
			_, err = db.Exec("UPDATE users SET username=?, role=?, name=?, is_active=? WHERE id=?", p.Username, p.Role, p.Name, active, p.ID)
		}
	} else {
		_, err = db.Exec("INSERT INTO users (username, password, role, name) VALUES (?, ?, ?, ?)", p.Username, p.Password, p.Role, p.Name)
	}
	sendResult(w, err)
}

func handleDeleteUser(w http.ResponseWriter, p Payload) {
	_, err := db.Exec("UPDATE users SET is_active = 0 WHERE id = ?", p.ID)
	sendResult(w, err)
}

func handleAddWash(w http.ResponseWriter, p Payload) {
	_, err := db.Exec("INSERT INTO washes (user_id, car_model, description, price, date_only) VALUES (?, ?, ?, ?, ?)", p.UserID, p.CarModel, p.Description, p.Price, p.Date)
	sendResult(w, err)
}

// НОВОЕ: Редактирование машины
func handleEditWash(w http.ResponseWriter, p Payload) {
	// Обновляем запись. Если админ меняет работника (user_id), это тоже учтено.
	_, err := db.Exec("UPDATE washes SET car_model=?, description=?, price=?, date_only=?, user_id=? WHERE id=?",
		p.CarModel, p.Description, p.Price, p.Date, p.UserID, p.ID)
	sendResult(w, err)
}

// НОВОЕ: Удаление машины
func handleDeleteWash(w http.ResponseWriter, p Payload) {
	// Удаляем запись физически
	_, err := db.Exec("DELETE FROM washes WHERE id=?", p.ID)
	sendResult(w, err)
}

func handleAddExpense(w http.ResponseWriter, p Payload) {
	_, err := db.Exec("INSERT INTO expenses (title, amount, date_only) VALUES (?, ?, ?)", p.Title, p.Amount, p.Date)
	sendResult(w, err)
}

func handleGetReport(w http.ResponseWriter, p Payload) {
	response := make(map[string]interface{})
	var washes []Wash
	var expenses []Expense

	var rows *sql.Rows
	var err error

	if p.Role == "admin" {
		rows, err = db.Query(`
			SELECT w.id, w.user_id, w.car_model, w.description, w.price, w.created_at, w.date_only, u.name 
			FROM washes w 
			LEFT JOIN users u ON w.user_id = u.id 
			WHERE w.date_only BETWEEN ? AND ? 
			ORDER BY w.date_only DESC, w.id DESC`, p.Start, p.End)
	} else {
		rows, err = db.Query(`
			SELECT id, user_id, car_model, description, price, created_at, date_only 
			FROM washes 
			WHERE user_id = ? AND date_only BETWEEN ? AND ? 
			ORDER BY id DESC`, p.UserID, p.Start, p.End)
	}

	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var w Wash
			var createdStr string
			if p.Role == "admin" {
				rows.Scan(&w.ID, &w.UserID, &w.CarModel, &w.Description, &w.Price, &createdStr, &w.DateOnly, &w.WorkerName)
			} else {
				rows.Scan(&w.ID, &w.UserID, &w.CarModel, &w.Description, &w.Price, &createdStr, &w.DateOnly)
			}
			w.CreatedAt = createdStr
			washes = append(washes, w)
		}
	}
	if washes == nil {
		washes = []Wash{}
	}
	response["washes"] = washes

	if p.Role == "admin" {
		rowsExp, err := db.Query("SELECT id, title, amount, date_only FROM expenses WHERE date_only BETWEEN ? AND ? ORDER BY date_only DESC", p.Start, p.End)
		if err == nil {
			defer rowsExp.Close()
			for rowsExp.Next() {
				var e Expense
				rowsExp.Scan(&e.ID, &e.Title, &e.Amount, &e.DateOnly)
				expenses = append(expenses, e)
			}
		}
	}
	if expenses == nil {
		expenses = []Expense{}
	}
	response["expenses"] = expenses

	json.NewEncoder(w).Encode(response)
}

func sendResult(w http.ResponseWriter, err error) {
	if err != nil {
		fmt.Println("Error:", err)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}
}
