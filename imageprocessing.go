package main

import (
	"fmt"

	"github.com/otiai10/gosseract/v2"
)

func processImage(imgID int64, colID string, filename string) {
	client := gosseract.NewClient()
	defer client.Close()
	client.SetImage(filename)
	text, _ := client.Text()
	fmt.Println(text)

	dbMutex.Lock()
	_, err := db.Exec("UPDATE images SET processed = 1 WHERE id = ?", imgID)
	if err != nil {
		fmt.Println("Processing Error 1", err)
	}
	_, err = db.Exec("UPDATE images SET processing_data = ? WHERE id = ?", text, imgID)
	if err != nil {
		fmt.Println("Processing Error 2", err)
	}
	_, err = db.Exec("UPDATE collections SET processed = processed + 1 WHERE id = ?", colID)
	if err != nil {
		fmt.Println("Processing Error 3", err)
	}
	dbMutex.Unlock()
}
