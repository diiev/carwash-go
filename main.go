package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
)

const (
	DBHost = "127.0.0.1"
	DBPort = "5432"
	DBUser = "diiev"
	DBPass = "331995577Qs"
	DBName = "carwash_db"
)

var db *sql.DB

// --- МОДЕЛИ ---
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
	ID           int     `json:"id"`
	UserID       int     `json:"user_id"`
	CoWorkerID   *int    `json:"co_worker_id"`
	CarModel     string  `json:"car_model"`
	Description  string  `json:"description"`
	Price        float64 `json:"price"`
	DateOnly     string  `json:"date_only"`
	IsFree       int     `json:"is_free"`
	WorkerName   string  `json:"worker_name"`
	CoWorkerName *string `json:"co_worker_name"`
}
type Expense struct {
	ID       int     `json:"id"`
	Title    string  `json:"title"`
	Amount   float64 `json:"amount"`
	DateOnly string  `json:"date_only"`
}

// НОВАЯ СТРУКТУРА ДЛЯ ОКРУГЛЕНИЙ
type Bonus struct {
	ID          int     `json:"id"`
	UserID      int     `json:"user_id"`
	UserName    string  `json:"user_name"`
	Amount      float64 `json:"amount"`
	DateOnly    string  `json:"date_only"`
	Description string  `json:"description"`
}

type Payload struct {
	Action string `json:"-"`
	ID     int    `json:"id"`
	// User
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
	Name     string `json:"name"`
	IsActive *int   `json:"is_active"`
	// Service
	ServiceName  string  `json:"service_name"`
	ServicePrice float64 `json:"service_price"`
	// Wash & Expense & Bonus
	UserID      int     `json:"user_id"`
	CoWorkerID  *int    `json:"co_worker_id"`
	CarModel    string  `json:"car_model"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	IsFree      int     `json:"is_free"`
	Title       string  `json:"title"`
	Amount      float64 `json:"amount"`
	Date        string  `json:"date"`
	Start       string  `json:"start"`
	End         string  `json:"end"`
	FilterID    int     `json:"filter_id"`
}

func main() {
	f, err := os.OpenFile("server.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(f)
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", DBHost, DBPort, DBUser, DBPass, DBName)
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to PostgreSQL DB!")

	http.HandleFunc("/api", corsMiddleware(apiHandler))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "index.html") })

	fmt.Println("Server running on port 80...")
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
	case "login":
		var u User
		err := db.QueryRow("SELECT id, username, password, role, name, is_active FROM users WHERE username=$1 AND is_active=1", p.Username).Scan(&u.ID, &u.Username, &u.Password, &u.Role, &u.Name, &u.IsActive)
		if err != nil || u.Password != p.Password {
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false})
			return
		}
		u.Password = ""
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "user": u})

	case "update_profile":
		var err error
		if p.Password != "" {
			_, err = db.Exec("UPDATE users SET username=$1, password=$2 WHERE id=$3", p.Username, p.Password, p.ID)
		} else {
			_, err = db.Exec("UPDATE users SET username=$1 WHERE id=$2", p.Username, p.ID)
		}
		send(w, err)

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
				_, err = db.Exec("UPDATE users SET username=$1, role=$2, name=$3, is_active=$4, password=$5 WHERE id=$6", p.Username, p.Role, p.Name, act, p.Password, p.ID)
			} else {
				_, err = db.Exec("UPDATE users SET username=$1, role=$2, name=$3, is_active=$4 WHERE id=$5", p.Username, p.Role, p.Name, act, p.ID)
			}
		} else {
			_, err = db.Exec("INSERT INTO users (username, password, role, name) VALUES ($1, $2, $3, $4)", p.Username, p.Password, p.Role, p.Name)
		}
		send(w, err)
	case "delete_user":
		_, err := db.Exec("UPDATE users SET is_active = 0 WHERE id = $1", p.ID)
		send(w, err)

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
			_, err = db.Exec("UPDATE services SET name=$1, price=$2 WHERE id=$3", p.ServiceName, p.ServicePrice, p.ID)
		} else {
			_, err = db.Exec("INSERT INTO services (name, price) VALUES ($1, $2)", p.ServiceName, p.ServicePrice)
		}
		send(w, err)
	case "delete_service":
		_, err := db.Exec("UPDATE services SET is_active = 0 WHERE id = $1", p.ID)
		send(w, err)

	case "add_wash":
		_, err := db.Exec("INSERT INTO washes (user_id, co_worker_id, car_model, description, price, date_only, is_free) VALUES ($1, $2, $3, $4, $5, $6, $7)", p.UserID, p.CoWorkerID, p.CarModel, p.Description, p.Price, p.Date, p.IsFree)
		send(w, err)
	case "edit_wash":
		_, err := db.Exec("UPDATE washes SET car_model=$1, description=$2, price=$3, date_only=$4, user_id=$5, co_worker_id=$6, is_free=$7 WHERE id=$8", p.CarModel, p.Description, p.Price, p.Date, p.UserID, p.CoWorkerID, p.IsFree, p.ID)
		send(w, err)
	case "delete_wash":
		_, err := db.Exec("DELETE FROM washes WHERE id=$1", p.ID)
		send(w, err)

	case "add_expense":
		_, err := db.Exec("INSERT INTO expenses (title, amount, date_only) VALUES ($1, $2, $3)", p.Title, p.Amount, p.Date)
		send(w, err)
	case "edit_expense":
		_, err := db.Exec("UPDATE expenses SET title=$1, amount=$2, date_only=$3 WHERE id=$4", p.Title, p.Amount, p.Date, p.ID)
		send(w, err)
	case "delete_expense":
		_, err := db.Exec("DELETE FROM expenses WHERE id=$1", p.ID)
		send(w, err)

	// НОВЫЕ МЕТОДЫ ДЛЯ ОКРУГЛЕНИЙ (БОНУСОВ)
	case "add_bonus":
		_, err := db.Exec("INSERT INTO bonuses (user_id, amount, date_only, description) VALUES ($1, $2, $3, $4)", p.UserID, p.Amount, p.Date, p.Description)
		send(w, err)
	case "delete_bonus":
		_, err := db.Exec("DELETE FROM bonuses WHERE id=$1", p.ID)
		send(w, err)

	case "get_report":
		res := make(map[string]interface{})
		var washes []Wash
		var args []interface{}

		query := `SELECT w.id, w.user_id, w.co_worker_id, w.car_model, w.description, w.price, w.date_only, w.is_free, u.name, cu.name 
				  FROM washes w 
				  LEFT JOIN users u ON w.user_id = u.id 
				  LEFT JOIN users cu ON w.co_worker_id = cu.id 
				  WHERE w.date_only BETWEEN $1 AND $2 `

		if p.Role == "admin" && p.FilterID > 0 {
			query += "AND (w.user_id = $3 OR w.co_worker_id = $4) "
			args = []interface{}{p.Start, p.End, p.FilterID, p.FilterID}
		} else if p.Role == "worker" {
			query += "AND (w.user_id = $3 OR w.co_worker_id = $4) "
			args = []interface{}{p.Start, p.End, p.UserID, p.UserID}
		} else {
			args = []interface{}{p.Start, p.End}
		}
		query += "ORDER BY w.date_only DESC, w.id DESC"

		rows, err := db.Query(query, args...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var w Wash
				var cwId sql.NullInt64
				var wName sql.NullString
				var cwName sql.NullString
				var isFree sql.NullInt64

				rows.Scan(&w.ID, &w.UserID, &cwId, &w.CarModel, &w.Description, &w.Price, &w.DateOnly, &isFree, &wName, &cwName)
				if cwId.Valid {
					id := int(cwId.Int64)
					w.CoWorkerID = &id
				}
				if wName.Valid {
					w.WorkerName = wName.String
				}
				if cwName.Valid {
					w.CoWorkerName = &cwName.String
				}
				if isFree.Valid {
					w.IsFree = int(isFree.Int64)
				}
				washes = append(washes, w)
			}
		}
		if washes == nil {
			washes = []Wash{}
		}

		var expenses []Expense
		var bonuses []Bonus // Добавляем массив для бонусов

		if p.Role == "admin" {
			// Расходы
			rExp, _ := db.Query("SELECT id, title, amount, date_only FROM expenses WHERE date_only BETWEEN $1 AND $2 ORDER BY date_only DESC", p.Start, p.End)
			if rExp != nil {
				defer rExp.Close()
				for rExp.Next() {
					var e Expense
					rExp.Scan(&e.ID, &e.Title, &e.Amount, &e.DateOnly)
					expenses = append(expenses, e)
				}
			}

			// Округления (Бонусы)
			bQuery := `SELECT b.id, b.user_id, b.amount, b.date_only, b.description, u.name 
					   FROM bonuses b JOIN users u ON b.user_id = u.id 
					   WHERE b.date_only BETWEEN $1 AND $2 `
			var bArgs []interface{}
			if p.FilterID > 0 {
				bQuery += "AND b.user_id = $3 "
				bArgs = []interface{}{p.Start, p.End, p.FilterID}
			} else {
				bArgs = []interface{}{p.Start, p.End}
			}
			bQuery += "ORDER BY b.date_only DESC"

			rBonuses, _ := db.Query(bQuery, bArgs...)
			if rBonuses != nil {
				defer rBonuses.Close()
				for rBonuses.Next() {
					var b Bonus
					rBonuses.Scan(&b.ID, &b.UserID, &b.Amount, &b.DateOnly, &b.Description, &b.UserName)
					bonuses = append(bonuses, b)
				}
			}
		} else if p.Role == "worker" {
			// Работник тоже должен видеть свои округления
			rBonuses, _ := db.Query("SELECT b.id, b.user_id, b.amount, b.date_only, b.description, u.name FROM bonuses b JOIN users u ON b.user_id = u.id WHERE b.date_only BETWEEN $1 AND $2 AND b.user_id = $3 ORDER BY b.date_only DESC", p.Start, p.End, p.UserID)
			if rBonuses != nil {
				defer rBonuses.Close()
				for rBonuses.Next() {
					var b Bonus
					rBonuses.Scan(&b.ID, &b.UserID, &b.Amount, &b.DateOnly, &b.Description, &b.UserName)
					bonuses = append(bonuses, b)
				}
			}
		}

		if expenses == nil {
			expenses = []Expense{}
		}
		if bonuses == nil {
			bonuses = []Bonus{}
		}

		res["washes"] = washes
		res["expenses"] = expenses
		res["bonuses"] = bonuses // Отправляем на фронт
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
