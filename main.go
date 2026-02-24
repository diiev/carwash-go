package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

const (
	DBHost = "127.0.0.1"
	DBPort = "5432"
	DBUser = "diiev"
	DBPass = "331995577Qs"
	DBName = "carwash_db"

	// Секретный ключ для токенов
	JWTSecret = "gums-super-secret-key-2026-carwash"
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

type Bonus struct {
	ID          int     `json:"id"`
	UserID      int     `json:"user_id"`
	UserName    string  `json:"user_name"`
	Amount      float64 `json:"amount"`
	DateOnly    string  `json:"date_only"`
	Description string  `json:"description"`
}

type Payload struct {
	Action       string  `json:"-"`
	ID           int     `json:"id"`
	Username     string  `json:"username"`
	Password     string  `json:"password"`
	Role         string  `json:"role"`
	Name         string  `json:"name"`
	IsActive     *int    `json:"is_active"`
	ServiceName  string  `json:"service_name"`
	ServicePrice float64 `json:"service_price"`
	UserID       int     `json:"user_id"`
	CoWorkerID   *int    `json:"co_worker_id"`
	CarModel     string  `json:"car_model"`
	Description  string  `json:"description"`
	Price        float64 `json:"price"`
	IsFree       int     `json:"is_free"`
	Title        string  `json:"title"`
	Amount       float64 `json:"amount"`
	Date         string  `json:"date"`
	Start        string  `json:"start"`
	End          string  `json:"end"`
	FilterID     int     `json:"filter_id"`

	// Данные авторизации из токена
	AuthUserID int    `json:"-"`
	AuthRole   string `json:"-"`
}

type Claims struct {
	UserID int    `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

type APIHandlerFunc func(p Payload, w http.ResponseWriter, r *http.Request)

var routes = map[string]APIHandlerFunc{
	"update_profile": handleUpdateProfile,
	"get_users":      handleGetUsers,
	"save_user":      handleSaveUser,
	"delete_user":    handleDeleteUser,
	"get_services":   handleGetServices,
	"save_service":   handleSaveService,
	"delete_service": handleDeleteService,
	"add_wash":       handleAddWash,
	"edit_wash":      handleEditWash,
	"delete_wash":    handleDeleteWash,
	"add_expense":    handleAddExpense,
	"edit_expense":   handleEditExpense,
	"delete_expense": handleDeleteExpense,
	"add_bonus":      handleAddBonus,
	"delete_bonus":   handleDeleteBonus,
	"get_report":     handleGetReport,
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

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to PostgreSQL DB successfully!")

	http.HandleFunc("/api", corsMiddleware(apiRouter))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "index.html") })

	fmt.Println("Server running on port 80...")
	log.Fatal(http.ListenAndServe(":80", nil))
}

// --- БЕЗОПАСНОСТЬ ---
func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	return string(bytes), err
}

func checkPasswordHash(password, hash string) bool {
	// 1. Пытаемся проверить как секьюрный зашифрованный пароль
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err == nil {
		return true
	}

	// 2. Если не получилось, сверяем как обычный текст (для обратной совместимости)
	return password == hash
}

// --- МИДЛВАРЫ И РОУТЕР ---
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func apiRouter(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")
	var p Payload
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&p)
	}

	if action == "login" {
		handleLogin(p, w, r)
		return
	}

	// Проверка токена
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Missing token"})
		return
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(JWTSecret), nil
	})

	if err != nil || !token.Valid {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid token"})
		return
	}

	p.AuthUserID = claims.UserID
	p.AuthRole = claims.Role

	// Проверка прав
	switch action {
	case "save_user", "delete_user", "save_service", "delete_service", "add_expense", "edit_expense", "delete_expense", "delete_wash", "add_bonus", "delete_bonus":
		if p.AuthRole != "admin" {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "Access denied"})
			return
		}
	}

	if handler, exists := routes[action]; exists {
		handler(p, w, r)
	} else {
		json.NewEncoder(w).Encode(map[string]string{"error": "Unknown action"})
	}
}

// --- ХЕНДЛЕРЫ ---
func handleLogin(p Payload, w http.ResponseWriter, r *http.Request) {
	var u User
	var dbHash string
	err := db.QueryRow("SELECT id, username, password, role, name, is_active FROM users WHERE username=$1 AND is_active=1", p.Username).
		Scan(&u.ID, &u.Username, &dbHash, &u.Role, &u.Name, &u.IsActive)

	if err != nil || !checkPasswordHash(p.Password, dbHash) {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Неверный логин или пароль"})
		return
	}

	expirationTime := time.Now().Add(30 * 24 * time.Hour)
	claims := &Claims{
		UserID: u.ID,
		Role:   u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(JWTSecret))

	u.Password = ""
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"user":    u,
		"token":   tokenString,
	})
}

func handleUpdateProfile(p Payload, w http.ResponseWriter, r *http.Request) {
	if p.AuthRole != "admin" && p.AuthUserID != p.ID {
		sendResponse(w, fmt.Errorf("Access denied"))
		return
	}
	var err error
	if p.Password != "" {
		hash, _ := hashPassword(p.Password)
		_, err = db.Exec("UPDATE users SET username=$1, password=$2 WHERE id=$3", p.Username, hash, p.ID)
	} else {
		_, err = db.Exec("UPDATE users SET username=$1 WHERE id=$2", p.Username, p.ID)
	}
	sendResponse(w, err)
}

func handleGetUsers(p Payload, w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, username, role, name, is_active FROM users ORDER BY is_active DESC, name ASC")
	if err != nil {
		sendResponse(w, err)
		return
	}
	defer rows.Close()

	res := make([]User, 0)
	for rows.Next() {
		var u User
		rows.Scan(&u.ID, &u.Username, &u.Role, &u.Name, &u.IsActive)
		res = append(res, u)
	}
	json.NewEncoder(w).Encode(res)
}

func handleSaveUser(p Payload, w http.ResponseWriter, r *http.Request) {
	var err error
	act := 1
	if p.IsActive != nil {
		act = *p.IsActive
	}

	if p.ID > 0 {
		if p.Password != "" {
			hash, _ := hashPassword(p.Password)
			_, err = db.Exec("UPDATE users SET username=$1, role=$2, name=$3, is_active=$4, password=$5 WHERE id=$6", p.Username, p.Role, p.Name, act, hash, p.ID)
		} else {
			_, err = db.Exec("UPDATE users SET username=$1, role=$2, name=$3, is_active=$4 WHERE id=$5", p.Username, p.Role, p.Name, act, p.ID)
		}
	} else {
		hash, _ := hashPassword(p.Password)
		_, err = db.Exec("INSERT INTO users (username, password, role, name) VALUES ($1, $2, $3, $4)", p.Username, hash, p.Role, p.Name)
	}
	sendResponse(w, err)
}

func handleDeleteUser(p Payload, w http.ResponseWriter, r *http.Request) {
	_, err := db.Exec("UPDATE users SET is_active = 0 WHERE id = $1", p.ID)
	sendResponse(w, err)
}

func handleGetServices(p Payload, w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, name, price, is_active FROM services WHERE is_active = 1 ORDER BY price ASC")
	if err != nil {
		sendResponse(w, err)
		return
	}
	defer rows.Close()

	res := make([]Service, 0)
	for rows.Next() {
		var s Service
		rows.Scan(&s.ID, &s.Name, &s.Price, &s.IsActive)
		res = append(res, s)
	}
	json.NewEncoder(w).Encode(res)
}

func handleSaveService(p Payload, w http.ResponseWriter, r *http.Request) {
	var err error
	if p.ID > 0 {
		_, err = db.Exec("UPDATE services SET name=$1, price=$2 WHERE id=$3", p.ServiceName, p.ServicePrice, p.ID)
	} else {
		_, err = db.Exec("INSERT INTO services (name, price) VALUES ($1, $2)", p.ServiceName, p.ServicePrice)
	}
	sendResponse(w, err)
}

func handleDeleteService(p Payload, w http.ResponseWriter, r *http.Request) {
	_, err := db.Exec("UPDATE services SET is_active = 0 WHERE id = $1", p.ID)
	sendResponse(w, err)
}

func handleAddWash(p Payload, w http.ResponseWriter, r *http.Request) {
	targetUserID := p.UserID
	if p.AuthRole == "worker" {
		targetUserID = p.AuthUserID
	}
	_, err := db.Exec("INSERT INTO washes (user_id, co_worker_id, car_model, description, price, date_only, is_free) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		targetUserID, p.CoWorkerID, p.CarModel, p.Description, p.Price, p.Date, p.IsFree)
	sendResponse(w, err)
}

func handleEditWash(p Payload, w http.ResponseWriter, r *http.Request) {
	targetUserID := p.UserID
	if p.AuthRole == "worker" {
		targetUserID = p.AuthUserID
	}
	_, err := db.Exec("UPDATE washes SET car_model=$1, description=$2, price=$3, date_only=$4, user_id=$5, co_worker_id=$6, is_free=$7 WHERE id=$8",
		p.CarModel, p.Description, p.Price, p.Date, targetUserID, p.CoWorkerID, p.IsFree, p.ID)
	sendResponse(w, err)
}

func handleDeleteWash(p Payload, w http.ResponseWriter, r *http.Request) {
	_, err := db.Exec("DELETE FROM washes WHERE id=$1", p.ID)
	sendResponse(w, err)
}

func handleAddExpense(p Payload, w http.ResponseWriter, r *http.Request) {
	_, err := db.Exec("INSERT INTO expenses (title, amount, date_only) VALUES ($1, $2, $3)", p.Title, p.Amount, p.Date)
	sendResponse(w, err)
}

func handleEditExpense(p Payload, w http.ResponseWriter, r *http.Request) {
	_, err := db.Exec("UPDATE expenses SET title=$1, amount=$2, date_only=$3 WHERE id=$4", p.Title, p.Amount, p.Date, p.ID)
	sendResponse(w, err)
}

func handleDeleteExpense(p Payload, w http.ResponseWriter, r *http.Request) {
	_, err := db.Exec("DELETE FROM expenses WHERE id=$1", p.ID)
	sendResponse(w, err)
}

func handleAddBonus(p Payload, w http.ResponseWriter, r *http.Request) {
	_, err := db.Exec("INSERT INTO bonuses (user_id, amount, date_only, description) VALUES ($1, $2, $3, $4)", p.UserID, p.Amount, p.Date, p.Description)
	sendResponse(w, err)
}

func handleDeleteBonus(p Payload, w http.ResponseWriter, r *http.Request) {
	_, err := db.Exec("DELETE FROM bonuses WHERE id=$1", p.ID)
	sendResponse(w, err)
}

func handleGetReport(p Payload, w http.ResponseWriter, r *http.Request) {
	res := make(map[string]interface{})

	role := p.AuthRole
	userID := p.AuthUserID
	if role == "admin" {
		userID = p.FilterID
	}

	washes := fetchWashesReport(p.Start, p.End, role, userID)
	res["washes"] = washes

	expenses := make([]Expense, 0)
	bonuses := make([]Bonus, 0)

	if role == "admin" {
		expenses = fetchExpensesReport(p.Start, p.End)
		bonuses = fetchBonusesReportAdmin(p.Start, p.End, userID)
	} else if role == "worker" {
		bonuses = fetchBonusesReportWorker(p.Start, p.End, userID)
	}

	res["expenses"] = expenses
	res["bonuses"] = bonuses
	json.NewEncoder(w).Encode(res)
}

func fetchWashesReport(start, end, role string, targetUserID int) []Wash {
	washes := make([]Wash, 0)
	var args []interface{}
	query := `SELECT w.id, w.user_id, w.co_worker_id, w.car_model, w.description, w.price, TO_CHAR(w.date_only, 'YYYY-MM-DD'), w.is_free, u.name, cu.name 
			  FROM washes w LEFT JOIN users u ON w.user_id = u.id LEFT JOIN users cu ON w.co_worker_id = cu.id 
			  WHERE w.date_only BETWEEN $1 AND $2 `

	if role == "admin" && targetUserID > 0 {
		query += "AND (w.user_id = $3 OR w.co_worker_id = $4) "
		args = []interface{}{start, end, targetUserID, targetUserID}
	} else if role == "worker" {
		query += "AND (w.user_id = $3 OR w.co_worker_id = $4) "
		args = []interface{}{start, end, targetUserID, targetUserID}
	} else {
		args = []interface{}{start, end}
	}
	query += "ORDER BY w.date_only DESC, w.id DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return washes
	}
	defer rows.Close()

	for rows.Next() {
		var w Wash
		var cwId sql.NullInt64
		var wName, cwName sql.NullString
		var isFree sql.NullInt64

		if err := rows.Scan(&w.ID, &w.UserID, &cwId, &w.CarModel, &w.Description, &w.Price, &w.DateOnly, &isFree, &wName, &cwName); err == nil {
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
	return washes
}

func fetchExpensesReport(start, end string) []Expense {
	expenses := make([]Expense, 0)
	rows, err := db.Query("SELECT id, title, amount, TO_CHAR(date_only, 'YYYY-MM-DD') FROM expenses WHERE date_only BETWEEN $1 AND $2 ORDER BY date_only DESC", start, end)
	if err != nil {
		return expenses
	}
	defer rows.Close()
	for rows.Next() {
		var e Expense
		rows.Scan(&e.ID, &e.Title, &e.Amount, &e.DateOnly)
		expenses = append(expenses, e)
	}
	return expenses
}

func fetchBonusesReportAdmin(start, end string, filterID int) []Bonus {
	bonuses := make([]Bonus, 0)
	query := `SELECT b.id, b.user_id, b.amount, TO_CHAR(b.date_only, 'YYYY-MM-DD'), COALESCE(b.description, ''), COALESCE(u.name, '') 
			   FROM bonuses b JOIN users u ON b.user_id = u.id WHERE b.date_only BETWEEN $1 AND $2 `
	var args []interface{}
	if filterID > 0 {
		query += "AND b.user_id = $3 "
		args = []interface{}{start, end, filterID}
	} else {
		args = []interface{}{start, end}
	}
	query += "ORDER BY b.date_only DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return bonuses
	}
	defer rows.Close()
	for rows.Next() {
		var b Bonus
		rows.Scan(&b.ID, &b.UserID, &b.Amount, &b.DateOnly, &b.Description, &b.UserName)
		bonuses = append(bonuses, b)
	}
	return bonuses
}

func fetchBonusesReportWorker(start, end string, userID int) []Bonus {
	bonuses := make([]Bonus, 0)
	rows, err := db.Query("SELECT b.id, b.user_id, b.amount, TO_CHAR(b.date_only, 'YYYY-MM-DD'), COALESCE(b.description, ''), COALESCE(u.name, '') FROM bonuses b JOIN users u ON b.user_id = u.id WHERE b.date_only BETWEEN $1 AND $2 AND b.user_id = $3 ORDER BY b.date_only DESC", start, end, userID)
	if err != nil {
		return bonuses
	}
	defer rows.Close()
	for rows.Next() {
		var b Bonus
		rows.Scan(&b.ID, &b.UserID, &b.Amount, &b.DateOnly, &b.Description, &b.UserName)
		bonuses = append(bonuses, b)
	}
	return bonuses
}

func sendResponse(w http.ResponseWriter, err error) {
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}
}
