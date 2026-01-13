package main

import (
	"log"
	"net/http"
)

func main() {
	fs := http.FileServer(http.Dir("/Users/stephen/Desktop"))

	http.Handle("/", fs)

	log.Println("文件服务启动：http://0.0.0.0:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
