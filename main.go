package main

import (
	"log"
	"net/http"
)

func main() {
	fs := http.FileServer(http.Dir("/mnt/MOBILEDISK1/吉他/卓著谱"))

	http.Handle("/", fs)

	log.Println("文件服务启动：http://0.0.0.0:8085")
	log.Fatal(http.ListenAndServe(":8085", nil))
}
