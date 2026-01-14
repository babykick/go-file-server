package main

import (
	"bufio"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

const rootPath = "/mnt/MOBILEDISK1/吉他/卓著谱"

type FileInfo struct {
	Name    string
	Size    string
	ModTime string
	IsDir   bool
	Link    string
}

type Breadcrumb struct {
	Name string
	Path string
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")

	r.GET("/*path", handleRequest)

	log.Println("文件服务启动：http://0.0.0.0:8085")
	log.Fatal(r.Run(":8085"))
}

func handleRequest(c *gin.Context) {
	reqPath := c.Param("path")
	query := c.Query("q")

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
		files = searchWithFd(query)
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
				Name:    name,
				Size:    size,
				ModTime: info.ModTime().Format("2006-01-02 15:04"),
				IsDir:   entry.IsDir(),
				Link:    url.PathEscape(link),
			})
		}
	}

	c.HTML(200, "index.html", gin.H{
		"Files":       files,
		"Breadcrumbs": breadcrumbs,
		"Parent":      parent,
		"Query":       query,
	})
}

func searchWithFd(query string) []FileInfo {
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
			Name:    relPath,
			Size:    size,
			ModTime: info.ModTime().Format("2006-01-02 15:04"),
			IsDir:   info.IsDir(),
			Link:    url.PathEscape(link),
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
