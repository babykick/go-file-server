package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v3"
)

var db *sql.DB

type Config struct {
	Server struct {
		Port     int    `yaml:"port"`
		RootPath string `yaml:"root_path"`
	} `yaml:"server"`
	Site struct {
		Title           string `yaml:"title"`
		OpenInNewWindow bool   `yaml:"open_in_new_window"`
	} `yaml:"site"`
}

var config Config

type FileInfo struct {
	Name       string
	Size       string
	ModTime    string
	IsDir      bool
	Link       string
	IsFavorite bool
}

type Breadcrumb struct {
	Name string
	Path string
}

func main() {
	// Load config
	configData, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatal("读取配置文件失败: ", err)
	}
	if err := yaml.Unmarshal(configData, &config); err != nil {
		log.Fatal("解析配置文件失败: ", err)
	}

	// Initialize database
	initDB()

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")

	r.POST("/api/favorite", handleAddFavorite)
	r.DELETE("/api/favorite", handleRemoveFavorite)
	r.GET("/*path", handleRequest)

	addr := fmt.Sprintf(":%d", config.Server.Port)
	log.Printf("文件服务启动：http://0.0.0.0%s", addr)
	log.Fatal(r.Run(addr))
}

func handleRequest(c *gin.Context) {
	reqPath := c.Param("path")

	// Handle favorites page
	if reqPath == "/favorites" {
		handleFavorites(c)
		return
	}

	query := c.Query("q")
	rootPath := config.Server.RootPath

	// Clean and validate path
	cleanPath := filepath.Clean(reqPath)
	fullPath := filepath.Join(rootPath, cleanPath)

	// Security check
	if !strings.HasPrefix(fullPath, rootPath) {
		c.String(403, "Forbidden")
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		c.String(404, "Not Found")
		return
	}

	// If it's a file, serve it directly
	if !info.IsDir() {
		c.File(fullPath)
		return
	}

	// Build breadcrumbs
	var breadcrumbs []Breadcrumb
	if cleanPath != "/" && cleanPath != "." {
		parts := strings.Split(strings.Trim(cleanPath, "/"), "/")
		for i, part := range parts {
			breadcrumbs = append(breadcrumbs, Breadcrumb{
				Name: part,
				Path: "/" + strings.Join(parts[:i+1], "/"),
			})
		}
	}

	// Get parent path
	var parent string
	if cleanPath != "/" && cleanPath != "." {
		parent = filepath.Dir(cleanPath)
		if parent == "." {
			parent = "/"
		}
	}

	var files []FileInfo

	// If search query exists, use fdfind
	if query != "" {
		files = searchWithFd(query, rootPath)
	} else {
		// Read directory
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			c.String(500, "Error reading directory")
			return
		}

		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				continue
			}

			name := entry.Name()

			var size string
			if entry.IsDir() {
				size = "-"
			} else {
				size = formatSize(info.Size())
			}

			link := filepath.Join(cleanPath, name)
			if entry.IsDir() {
				link += "/"
			}

			files = append(files, FileInfo{
				Name:       name,
				Size:       size,
				ModTime:    info.ModTime().Format("2006-01-02 15:04"),
				IsDir:      entry.IsDir(),
				Link:       url.PathEscape(link),
				IsFavorite: isFavorite(link),
			})
		}
	}

	c.HTML(200, "index.html", gin.H{
		"Files":           files,
		"Breadcrumbs":     breadcrumbs,
		"Parent":          parent,
		"Query":           query,
		"Title":           config.Site.Title,
		"OpenInNewWindow": config.Site.OpenInNewWindow,
	})
}

func searchWithFd(query string, rootPath string) []FileInfo {
	var files []FileInfo

	cmd := exec.Command("/usr/bin/fdfind", "-i", query, rootPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return files
	}

	if err := cmd.Start(); err != nil {
		return files
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		fullPath := scanner.Text()

		// Get relative path from rootPath
		relPath, err := filepath.Rel(rootPath, fullPath)
		if err != nil {
			continue
		}

		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		var size string
		if info.IsDir() {
			size = "-"
		} else {
			size = formatSize(info.Size())
		}

		link := "/" + relPath
		if info.IsDir() {
			link += "/"
		}

		files = append(files, FileInfo{
			Name:       relPath,
			Size:       size,
			ModTime:    info.ModTime().Format("2006-01-02 15:04"),
			IsDir:      info.IsDir(),
			Link:       url.PathEscape(link),
			IsFavorite: isFavorite(link),
		})
	}

	cmd.Wait()
	return files
}

func formatSize(size int64) string {
	return formatNumber(size)
}

func formatNumber(n int64) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		if len(s) > 0 && len(s)%4 == 3 {
			s = "," + s
		}
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

func initDB() {
	var err error
	db, err = sql.Open("sqlite3", "./favorites.db")
	if err != nil {
		log.Fatal("打开数据库失败: ", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS favorites (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT UNIQUE NOT NULL,
		name TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		log.Fatal("创建表失败: ", err)
	}
}

func isFavorite(path string) bool {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM favorites WHERE path = ?", path).Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

func handleAddFavorite(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	_, err := db.Exec("INSERT OR IGNORE INTO favorites (path, name) VALUES (?, ?)", req.Path, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add favorite"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func handleRemoveFavorite(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	_, err := db.Exec("DELETE FROM favorites WHERE path = ?", req.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove favorite"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func handleFavorites(c *gin.Context) {
	rows, err := db.Query("SELECT path, name FROM favorites ORDER BY created_at DESC")
	if err != nil {
		c.String(500, "Error reading favorites")
		return
	}
	defer rows.Close()

	var files []FileInfo
	rootPath := config.Server.RootPath

	for rows.Next() {
		var path, name string
		if err := rows.Scan(&path, &name); err != nil {
			continue
		}

		fullPath := filepath.Join(rootPath, path)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		var size string
		if info.IsDir() {
			size = "-"
		} else {
			size = formatSize(info.Size())
		}

		files = append(files, FileInfo{
			Name:       name,
			Size:       size,
			ModTime:    info.ModTime().Format("2006-01-02 15:04"),
			IsDir:      info.IsDir(),
			Link:       url.PathEscape(path),
			IsFavorite: true,
		})
	}

	c.HTML(200, "index.html", gin.H{
		"Files":           files,
		"Breadcrumbs":     nil,
		"Parent":          "",
		"Query":           "",
		"Title":           config.Site.Title,
		"OpenInNewWindow": config.Site.OpenInNewWindow,
		"IsFavoritesPage": true,
	})
}
