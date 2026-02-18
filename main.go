package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

// КОНФИГУРАЦИЯ БД
const (
	DBHost = "127.0.0.1"
	DBPort = "3306"
	DBUser = "diiev"
	DBPass = "331995577Qs"
	DBName = "carwash_db"
)

var db *sql.DB

// --- МОДЕЛИ ДАННЫХ ---
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	Role     string `json:"role"`
	Name     string `json:"name"`
	IsActive int    `json:"is_active"`
}

type Service struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	IsActive int     `json:"is_active"`
}

type Wash struct {
	ID          int     `json:"id"`
	UserID      int     `json:"user_id"`
	CarModel    string  `json:"car_model"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	DateOnly    string  `json:"date_only"`
	WorkerName  string  `json:"worker_name,omitempty"`
}

type Expense struct {
	ID       int     `json:"id"`
	Title    string  `json:"title"`
	Amount   float64 `json:"amount"`
	DateOnly string  `json:"date_only"`
}

// Структура для приема данных от Vue
type Payload struct {
	Action string `json:"-"` // Из URL
	ID     int    `json:"id"`

	// User
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
	Name     string `json:"name"`
	IsActive *int   `json:"is_active"`

	// Service (специальные поля, чтобы не путаться)
	ServiceName  string  `json:"service_name"`
	ServicePrice float64 `json:"service_price"`

	// Wash / Expense
	UserID      int     `json:"user_id"`
	CarModel    string  `json:"car_model"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Title       string  `json:"title"`
	Amount      float64 `json:"amount"`
	Date        string  `json:"date"`

	// Filters
	Start    string `json:"start"`
	End      string `json:"end"`
	FilterID int    `json:"filter_id"` // ID работника для фильтра
}

func main() {
	// Логирование в файл (опционально)
	f, err := os.OpenFile("server.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(f)
	}

	// Подключение к БД
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4", DBUser, DBPass, DBHost, DBPort, DBName)
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Error creating DB pool: ", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatal("Error connecting to DB: ", err)
	}
	fmt.Println("Connected to MariaDB!")

	// 1. API Handlers
	http.HandleFunc("/api", corsMiddleware(apiHandler))

	// 2. Static Files (Раздаем index.html)
	// Если запрашивают корень "/", отдаем index.html
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})

	// Запуск на 80 порту (стандартный веб)
	fmt.Println("Server running on port 80...")
	// На Linux для 80 порта нужны права root (sudo)
	log.Fatal(http.ListenAndServe(":80", nil))
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "OPTIONS" {
			return
		}
		next(w, r)
	}
}

func apiHandler(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")
	var p Payload
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&p)
	}

	switch action {
	// Auth & Users
	case "login":
		var u User
		err := db.QueryRow("SELECT id, username, password, role, name, is_active FROM users WHERE username=? AND is_active=1", p.Username).Scan(&u.ID, &u.Username, &u.Password, &u.Role, &u.Name, &u.IsActive)
		if err != nil || u.Password != p.Password {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false})
			return
		}
		u.Password = ""
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "user": u})

	case "get_users":
		rows, _ := db.Query("SELECT id, username, role, name, is_active FROM users ORDER BY is_active DESC, name ASC")
		defer rows.Close()
		var res []User
		for rows.Next() {
			var u User
			rows.Scan(&u.ID, &u.Username, &u.Role, &u.Name, &u.IsActive)
			res = append(res, u)
		}
		if res == nil {
			res = []User{}
		}
		json.NewEncoder(w).Encode(res)

	case "save_user":
		var err error
		act := 1
		if p.IsActive != nil {
			act = *p.IsActive
		}
		if p.ID > 0 {
			if p.Password != "" {
				_, err = db.Exec("UPDATE users SET username=?, role=?, name=?, is_active=?, password=? WHERE id=?", p.Username, p.Role, p.Name, act, p.Password, p.ID)
			} else {
				_, err = db.Exec("UPDATE users SET username=?, role=?, name=?, is_active=? WHERE id=?", p.Username, p.Role, p.Name, act, p.ID)
			}
		} else {
			_, err = db.Exec("INSERT INTO users (username, password, role, name) VALUES (?, ?, ?, ?)", p.Username, p.Password, p.Role, p.Name)
		}
		send(w, err)

	case "delete_user":
		_, err := db.Exec("UPDATE users SET is_active = 0 WHERE id = ?", p.ID)
		send(w, err)

	// Services
	case "get_services":
		rows, _ := db.Query("SELECT id, name, price, is_active FROM services WHERE is_active = 1 ORDER BY price ASC")
		defer rows.Close()
		var res []Service
		for rows.Next() {
			var s Service
			rows.Scan(&s.ID, &s.Name, &s.Price, &s.IsActive)
			res = append(res, s)
		}
		if res == nil {
			res = []Service{}
		}
		json.NewEncoder(w).Encode(res)

	case "save_service":
		var err error
		if p.ID > 0 {
			_, err = db.Exec("UPDATE services SET name=?, price=? WHERE id=?", p.ServiceName, p.ServicePrice, p.ID)
		} else {
			_, err = db.Exec("INSERT INTO services (name, price) VALUES (?, ?)", p.ServiceName, p.ServicePrice)
		}
		send(w, err)

	case "delete_service":
		_, err := db.Exec("UPDATE services SET is_active = 0 WHERE id = ?", p.ID)
		send(w, err)

	// Washes
	case "add_wash":
		_, err := db.Exec("INSERT INTO washes (user_id, car_model, description, price, date_only) VALUES (?, ?, ?, ?, ?)", p.UserID, p.CarModel, p.Description, p.Price, p.Date)
		send(w, err)

	case "edit_wash":
		_, err := db.Exec("UPDATE washes SET car_model=?, description=?, price=?, date_only=?, user_id=? WHERE id=?", p.CarModel, p.Description, p.Price, p.Date, p.UserID, p.ID)
		send(w, err)

	case "delete_wash":
		_, err := db.Exec("DELETE FROM washes WHERE id=?", p.ID)
		send(w, err)

	// Expenses
	case "add_expense":
		_, err := db.Exec("INSERT INTO expenses (title, amount, date_only) VALUES (?, ?, ?)", p.Title, p.Amount, p.Date)
		send(w, err)

	// Reports
	case "get_report":
		res := make(map[string]interface{})
		var washes []Wash
		var expenses []Expense

		// Логика фильтрации моек
		query := ""
		var args []interface{}

		if p.Role == "admin" {
			if p.FilterID > 0 {
				query = "SELECT w.id, w.user_id, w.car_model, w.description, w.price, w.date_only, u.name FROM washes w LEFT JOIN users u ON w.user_id=u.id WHERE w.date_only BETWEEN ? AND ? AND w.user_id=? ORDER BY w.date_only DESC, w.id DESC"
				args = []interface{}{p.Start, p.End, p.FilterID}
			} else {
				query = "SELECT w.id, w.user_id, w.car_model, w.description, w.price, w.date_only, u.name FROM washes w LEFT JOIN users u ON w.user_id=u.id WHERE w.date_only BETWEEN ? AND ? ORDER BY w.date_only DESC, w.id DESC"
				args = []interface{}{p.Start, p.End}
			}
		} else {
			query = "SELECT id, user_id, car_model, description, price, date_only FROM washes WHERE user_id=? AND date_only BETWEEN ? AND ? ORDER BY id DESC"
			args = []interface{}{p.UserID, p.Start, p.End}
		}

		rows, err := db.Query(query, args...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var w Wash
				if p.Role == "admin" {
					rows.Scan(&w.ID, &w.UserID, &w.CarModel, &w.Description, &w.Price, &w.DateOnly, &w.WorkerName)
				} else {
					rows.Scan(&w.ID, &w.UserID, &w.CarModel, &w.Description, &w.Price, &w.DateOnly)
				}
				washes = append(washes, w)
			}
		}
		if washes == nil {
			washes = []Wash{}
		}

		// Расходы (только админ)
		if p.Role == "admin" {
			rExp, _ := db.Query("SELECT id, title, amount, date_only FROM expenses WHERE date_only BETWEEN ? AND ?", p.Start, p.End)
			if rExp != nil {
				defer rExp.Close()
				for rExp.Next() {
					var e Expense
					rExp.Scan(&e.ID, &e.Title, &e.Amount, &e.DateOnly)
					expenses = append(expenses, e)
				}
			}
		}
		if expenses == nil {
			expenses = []Expense{}
		}

		res["washes"] = washes
		res["expenses"] = expenses
		json.NewEncoder(w).Encode(res)

	default:
		json.NewEncoder(w).Encode(map[string]string{"error": "Unknown"})
	}
}

func send(w http.ResponseWriter, err error) {
	if err != nil {
		fmt.Println("Err:", err)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}
}
