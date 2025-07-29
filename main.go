package main

import (
	"database/sql"
	"embed"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"net/http"
	"strconv"
	"time"
)

//go:embed test/index.html test/style.css test/script.js admin.html
var staticFiles embed.FS

// Типы заявок
var allTypes = []string{"base", "integrated", "it", "incident"}

func getDB() (*sql.DB, error) {
	host := "172.16.17.180"
	port := "3306"
	user := "krmu_app"
	pass := "KRMU*2025"
	dbname := "krmu_it"
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4", user, pass, host, port, dbname)
	return sql.Open("mysql", dsn)
}

func getAllowedTypes(db *sql.DB, userID int) ([]string, error) {
	rows, err := db.Query("SELECT type FROM user_allowed_types WHERE user_id=?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var types []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil {
			types = append(types, t)
		}
	}
	return types, nil
}

func setAllowedTypes(db *sql.DB, userID int, types []string) error {
	_, err := db.Exec("DELETE FROM user_allowed_types WHERE user_id=?", userID)
	if err != nil {
		return err
	}
	for _, t := range types {
		_, err := db.Exec("INSERT INTO user_allowed_types (user_id, type) VALUES (?, ?)", userID, t)
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	db, err := getDB()
	if err != nil {
		panic(err)
	}
	defer db.Close()

	r := gin.Default()
	store := cookie.NewStore([]byte("super-secret-key"))
	store.Options(sessions.Options{
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false,
	})
	r.Use(sessions.Sessions("helpdesk_session", store))

	// Serve static files
	r.GET("/", func(c *gin.Context) {
		c.Header("Content-Type", "text/html")
		data, _ := staticFiles.ReadFile("test/index.html")
		c.Writer.Write(data)
	})
	r.GET("/style.css", func(c *gin.Context) {
		c.Header("Content-Type", "text/css")
		data, _ := staticFiles.ReadFile("test/style.css")
		c.Writer.Write(data)
	})
	r.GET("/script.js", func(c *gin.Context) {
		c.Header("Content-Type", "application/javascript")
		data, _ := staticFiles.ReadFile("test/script.js")
		c.Writer.Write(data)
	})

	// Логин по login+password
	r.POST("/api/login", func(c *gin.Context) {
		type loginReq struct {
			Login    string `json:"login"`
			Password string `json:"password"`
		}
		var req loginReq
		if err := c.ShouldBindJSON(&req); err != nil || req.Login == "" || req.Password == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Введите логин и пароль"})
			return
		}
		var userID int
		var fio, role, dbPassword string
		err := db.QueryRow("SELECT id, fio, role, password FROM users WHERE login=?", req.Login).Scan(&userID, &fio, &role, &dbPassword)
		if err == sql.ErrNoRows || dbPassword != req.Password {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Неверный логин или пароль"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка поиска пользователя"})
			return
		}
		allowedTypes, _ := getAllowedTypes(db, userID)
		sess := sessions.Default(c)
		sess.Set("user_id", userID)
		sess.Set("fio", fio)
		sess.Set("role", role)
		sess.Set("allowed_types", allowedTypes)
		sess.Save()
		c.JSON(http.StatusOK, gin.H{"fio": fio, "role": role})
	})

	// Logout (clears session)
	r.POST("/api/logout", func(c *gin.Context) {
		sess := sessions.Default(c)
		sess.Clear()
		sess.Save()
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Получить allowed_types для текущего пользователя
	r.GET("/api/allowed-types", func(c *gin.Context) {
		sess := sessions.Default(c)
		userID, ok := sess.Get("user_id").(int)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Не авторизован"})
			return
		}
		types, err := getAllowedTypes(db, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения типов"})
			return
		}
		c.JSON(http.StatusOK, types)
	})

	// Middleware to require login
	requireLogin := func(c *gin.Context) {
		sess := sessions.Default(c)
		userID := sess.Get("user_id")
		if userID == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Требуется вход"})
			c.Abort()
			return
		}
		c.Set("user_id", userID)
		c.Next()
	}

	// Middleware: только для IT-Admin
	requireAdmin := func(c *gin.Context) {
		sess := sessions.Default(c)
		role := sess.Get("role")
		if role != "IT-Admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Доступ только для IT-Admin"})
			c.Abort()
			return
		}
		c.Next()
	}

	// Get current user's requests
	r.GET("/api/my-requests", requireLogin, func(c *gin.Context) {
		userID := c.GetInt("user_id")
		rows, err := db.Query("SELECT id, title, status, created_at FROM requests WHERE user_id=? ORDER BY created_at DESC", userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения заявок"})
			return
		}
		defer rows.Close()
		type Req struct {
			ID     int    `json:"id"`
			Title  string `json:"title"`
			Status string `json:"status"`
			Date   string `json:"date"`
		}
		var reqs []Req
		for rows.Next() {
			var r Req
			var created time.Time
			if err := rows.Scan(&r.ID, &r.Title, &r.Status, &created); err == nil {
				r.Date = created.Format("2006-01-02")
				reqs = append(reqs, r)
			}
		}
		c.JSON(http.StatusOK, reqs)
	})

	// Create new request for current user
	r.POST("/api/new-request", requireLogin, func(c *gin.Context) {
		userID := c.GetInt("user_id")
		type newReq struct {
			Title string `json:"title"`
			Descr string `json:"descr"`
		}
		var req newReq
		if err := c.ShouldBindJSON(&req); err != nil || req.Title == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Тема обязательна"})
			return
		}
		_, err := db.Exec("INSERT INTO requests (user_id, title, status, description) VALUES (?, ?, ?, ?)", userID, req.Title, "Зарегистрировано", req.Descr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания заявки"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Получить всех пользователей (с allowed_types)
	r.GET("/api/admin/users", requireAdmin, func(c *gin.Context) {
		rows, err := db.Query("SELECT id, login, fio, role FROM users ORDER BY id")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения пользователей"})
			return
		}
		defer rows.Close()
		type User struct {
			ID           int      `json:"id"`
			Login        string   `json:"login"`
			Fio          string   `json:"fio"`
			Role         string   `json:"role"`
			AllowedTypes []string `json:"allowed_types"`
		}
		var users []User
		for rows.Next() {
			var u User
			if err := rows.Scan(&u.ID, &u.Login, &u.Fio, &u.Role); err == nil {
				u.AllowedTypes, _ = getAllowedTypes(db, u.ID)
				users = append(users, u)
			}
		}
		c.JSON(http.StatusOK, users)
	})

	// Создать пользователя
	r.POST("/api/admin/users", requireAdmin, func(c *gin.Context) {
		type reqUser struct {
			Login        string   `json:"login"`
			Password     string   `json:"password"`
			Fio          string   `json:"fio"`
			Role         string   `json:"role"`
			AllowedTypes []string `json:"allowed_types"`
		}
		var req reqUser
		if err := c.ShouldBindJSON(&req); err != nil || req.Login == "" || req.Password == "" || req.Fio == "" || req.Role == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Все поля обязательны"})
			return
		}
		var exists int
		err := db.QueryRow("SELECT COUNT(*) FROM users WHERE login=?", req.Login).Scan(&exists)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка при проверке пользователя"})
			return
		}
		if exists > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Пользователь с таким логином уже существует"})
			return
		}
		res, err := db.Exec("INSERT INTO users (login, password, fio, role) VALUES (?, ?, ?, ?)", req.Login, req.Password, req.Fio, req.Role)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка создания пользователя"})
			return
		}
		id64, _ := res.LastInsertId()
		userID := int(id64)
		types := req.AllowedTypes
		if len(types) == 0 {
			types = allTypes
		}
		setAllowedTypes(db, userID, types)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Редактировать пользователя
	r.PUT("/api/admin/users/:id", requireAdmin, func(c *gin.Context) {
		type reqUser struct {
			Fio          string   `json:"fio"`
			Role         string   `json:"role"`
			Password     string   `json:"password"`
			AllowedTypes []string `json:"allowed_types"`
		}
		var req reqUser
		id := c.Param("id")
		idInt, _ := strconv.Atoi(id)
		if err := c.ShouldBindJSON(&req); err != nil || req.Fio == "" || req.Role == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ФИО и роль обязательны"})
			return
		}
		if req.Password != "" {
			_, err := db.Exec("UPDATE users SET fio=?, role=?, password=? WHERE id=?", req.Fio, req.Role, req.Password, id)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления пользователя"})
				return
			}
		} else {
			_, err := db.Exec("UPDATE users SET fio=?, role=? WHERE id=?", req.Fio, req.Role, id)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления пользователя"})
				return
			}
		}
		setAllowedTypes(db, idInt, req.AllowedTypes)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Получить все заявки
	r.GET("/api/admin/requests", requireAdmin, func(c *gin.Context) {
		rows, err := db.Query(`SELECT r.id, u.fio, r.title, r.status, r.created_at, r.description FROM requests r JOIN users u ON r.user_id = u.id ORDER BY r.created_at DESC`)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения заявок"})
			return
		}
		defer rows.Close()
		type Req struct {
			ID     int    `json:"id"`
			User   string `json:"user"`
			Title  string `json:"title"`
			Status string `json:"status"`
			Date   string `json:"date"`
			Descr  string `json:"descr"`
		}
		var reqs []Req
		for rows.Next() {
			var r Req
			var created time.Time
			if err := rows.Scan(&r.ID, &r.User, &r.Title, &r.Status, &created, &r.Descr); err == nil {
				r.Date = created.Format("2006-01-02")
				reqs = append(reqs, r)
			}
		}
		c.JSON(http.StatusOK, reqs)
	})

	// Изменить статус заявки
	r.POST("/api/admin/request-status", requireAdmin, func(c *gin.Context) {
		type reqStatus struct {
			ID     int    `json:"id"`
			Status string `json:"status"`
		}
		var req reqStatus
		if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 || req.Status == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ID и статус обязательны"})
			return
		}
		_, err := db.Exec("UPDATE requests SET status=? WHERE id=?", req.Status, req.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка обновления статуса"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Test DB connection endpoint
	r.GET("/api/test-db", func(c *gin.Context) {
		err := db.Ping()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Serve test page
	r.GET("/test-db", func(c *gin.Context) {
		html := `<!DOCTYPE html><html lang='ru'><head><meta charset='UTF-8'><title>Тест соединения с БД</title></head><body style='font-family:sans-serif;padding:2em;'><h2>Тест соединения с базой данных</h2><button id='btn'>Проверить соединение</button><div id='result' style='margin-top:1em;'></div><script>document.getElementById('btn').onclick=async()=>{const r=document.getElementById('result');r.textContent='Проверка...';try{const res=await fetch('/api/test-db');const data=await res.json();if(data.ok){r.textContent='✅ Соединение с БД успешно!';}else{r.textContent='❌ Ошибка: '+(data.error||'Неизвестно');}}catch(e){r.textContent='❌ Ошибка: '+e;}}</script></body></html>`
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, html)
	})

	// Страница админ-панели
	r.GET("/admin", func(c *gin.Context) {
		c.Header("Content-Type", "text/html")
		adminPanel, _ := staticFiles.ReadFile("admin.html")
		c.Writer.Write(adminPanel)
	})

	r.Run("0.0.0.0:8080")
} 
 