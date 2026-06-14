package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/genai"
)

// for sending to server
type ProcessedData struct {
	Unfiltered_text  string
	Issues           []string
	Potential_issues []string
	Legal            bool
	Unsure           bool
}

type Image struct {
	ID              int
	CollectionID    int
	Name            string
	Type            string
	Alcohol_content string
	Net_content     string
	Origin          string
	Filename        string
	Processed       int
	Processing_data ProcessedData

	UploadDate time.Time
}

type Collection struct {
	ID          int
	Name        string
	Address     string
	Email       string
	Processed   int
	CreatedDate time.Time

	//used for server, not in db.
	ImageCount         int
	Images             []Image
	FirstImageFilename string
}

var (
	db        *sql.DB
	templates *template.Template
	dbMutex   sync.RWMutex
)

var uploadPort = ":8080"
var galleryPort = ":8081"
var aiClient *genai.Client

func main() {
	//load api key
	data, err := os.ReadFile("apikey.txt")
	if err != nil {
		fmt.Println("Error loading apikey.txt: ", err)
		return
	}
	apiKey := strings.TrimSpace(string(data))

	ctx := context.Background()

	//create gemma connection
	aiClient, err = genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})

	// Parse command line arguments for ports

	if len(os.Args) > 1 {
		uploadPort = ":" + os.Args[1]
	}
	if len(os.Args) > 2 {
		galleryPort = ":" + os.Args[2]
	}

	// Initialize database
	db, err = sql.Open("sqlite3", "./image_collections.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create tables
	createTables()

	//clear processing entries
	deleteUnprocessedCollections()

	// Parse templates
	templates = template.Must(template.ParseGlob("templates/*.html"))

	// Create uploads directory
	os.MkdirAll("uploads", 0755)

	// Create wait group for both servers
	var wg sync.WaitGroup
	wg.Add(2)

	// Start Upload Server on upload port
	go func() {
		defer wg.Done()
		uploadMux := http.NewServeMux()

		// Serve static files for upload server
		uploadMux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
		uploadMux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

		// Upload routes
		uploadMux.HandleFunc("/", indexHandler)
		uploadMux.HandleFunc("/upload", uploadHandler)
		uploadMux.HandleFunc("/collection/", collectionHandler)

		uploadServer := &http.Server{
			Addr:    uploadPort,
			Handler: uploadMux,
		}

		fmt.Printf("Upload server starting on http://localhost%s\n", uploadPort)
		if err := uploadServer.ListenAndServe(); err != nil {
			log.Printf("Upload server error: %v", err)
		}
	}()

	// Start Gallery Server on gallery port
	go func() {
		defer wg.Done()
		galleryMux := http.NewServeMux()

		// Serve static files for gallery
		galleryMux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
		galleryMux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

		// Gallery routes
		galleryMux.HandleFunc("/", galleryHandler)
		galleryMux.HandleFunc("/collection/", viewCollectionHandler)
		galleryMux.HandleFunc("/delete/", deleteHandler)
		galleryMux.HandleFunc("/delete-collection/", deleteCollectionHandler)
		galleryMux.HandleFunc("/image/", viewImageHandler)

		galleryServer := &http.Server{
			Addr:    galleryPort,
			Handler: galleryMux,
		}

		fmt.Printf("Gallery server starting on http://localhost%s\n", galleryPort)
		if err := galleryServer.ListenAndServe(); err != nil {
			log.Printf("Gallery server error: %v", err)
		}
	}()

	// Wait for both servers
	wg.Wait()
}

func createTables() {
	queries := []string{
		//non-prototype would have email/company info as well
		`CREATE TABLE IF NOT EXISTS collections (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            address TEXT,
            email TEXT,
			processed INTEGER DEFAULT 0,
            created_date DATETIME DEFAULT CURRENT_TIMESTAMP
        )`,

		//non-prototype would have 2 images
		`CREATE TABLE IF NOT EXISTS images (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            collection_id INTEGER,
            name TEXT NOT NULL,
            type TEXT,
            alcohol_content TEXT,
            net_content TEXT,
			origin TEXT,
            filename TEXT NOT NULL,
			processed integer DEFAULT 0,
			processing_data TEXT,
            upload_date DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (collection_id) REFERENCES collections(id)
        )`,
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			log.Fatal(err)
		}
	}
}

// Helper function to get collections
func getCollections() ([]Collection, error) {
	dbMutex.RLock()
	rows, err := db.Query(`
        SELECT c.id, c.name, c.created_date,
            COUNT(i.id) as image_count,
			(SELECT i2.filename FROM images i2
			WHERE i2.collection_id = c.id
			ORDER BY i2.upload_date ASC LIMIT 1) as first_filename
        FROM collections c
        LEFT JOIN images i ON c.id = i.collection_id
        GROUP BY c.id
		HAVING c.processed >= image_count
        ORDER BY c.created_date DESC
    `)
	dbMutex.RUnlock()

	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	var collections []Collection
	for rows.Next() {
		var c Collection
		err := rows.Scan(&c.ID, &c.Name, &c.CreatedDate, &c.ImageCount, &c.FirstImageFilename)
		if err != nil {
			continue
		}
		collections = append(collections, c)
	}

	rows.Close()

	return collections, nil
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	collections, err := getCollections()
	if err != nil {
		http.Error(w, "Error loading collections", http.StatusInternalServerError)
		return
	}

	data := struct {
		Collections []Collection
		Error       string
	}{
		Collections: collections,
	}

	templates.ExecuteTemplate(w, "index.html", data)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		// Parse multipart form (500 MB max for multiple files)
		r.ParseMultipartForm(500 << 20)

		collectionID := r.FormValue("collection_id")
		collection_name := r.FormValue("name")
		collection_address := r.FormValue("address")

		if collection_name == "" && collectionID == "" {
			http.Error(w, "No collection name", http.StatusBadRequest)
			return
		}

		if collectionID == "" {
			dbMutex.Lock()
			res, err := db.Exec(
				"INSERT INTO collections (name, address) VALUES (?, ?)", collection_name, collection_address,
			)
			dbMutex.Unlock()

			if err == nil {
				lastid, err := res.LastInsertId()
				if err != nil {
					fmt.Println("Error: lastid")
					return
				}
				collectionID = strconv.Itoa(int(lastid))
			} else {
				fmt.Println("Collection creation error: ", err)
			}
		}

		// Get all files
		files := r.MultipartForm.File["images"]
		if len(files) == 0 {
			collections, _ := getCollections()
			templates.ExecuteTemplate(w, "index.html", map[string]interface{}{
				"Collections": collections,
				"Error":       "Please select at least one image",
			})
			return
		}

		successCount := 0
		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				continue
			}
			defer file.Close()

			// Get individual image form fields (using indexed naming)
			index := strconv.Itoa(successCount)
			name := r.FormValue("name_" + index)
			title := r.FormValue("name_" + index)
			typee := r.FormValue("type_" + index)
			alcohol_content := r.FormValue("alcohol_content_" + index)
			net_content := r.FormValue("net_content_" + index)
			origin := r.FormValue("origin_" + index)

			if name == "" {
				name = strings.TrimSuffix(fileHeader.Filename, filepath.Ext(fileHeader.Filename))
			}

			// Create unique filename
			filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), fileHeader.Filename)
			filepath := filepath.Join("uploads", filename)

			// Save file
			dst, err := os.Create(filepath)
			if err != nil {
				continue
			}

			if _, err := io.Copy(dst, file); err != nil {
				dst.Close()
				continue
			}
			dst.Close()

			dbMutex.Lock()
			res, err := db.Exec(
				"INSERT INTO images (collection_id, name, type, alcohol_content, net_content, origin, filename) VALUES (?, ?, ?, ?, ?, ?, ?)",
				collectionID, title, typee, alcohol_content, net_content, origin, filename,
			)
			dbMutex.Unlock()

			if err == nil {
				successCount++
				lastid, err := res.LastInsertId()
				if err != nil {
					fmt.Println("Error: lastid")
					return
				}
				go processImage(lastid, collectionID)

			}

		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func collectionHandler(w http.ResponseWriter, r *http.Request) {
	collectionID := r.URL.Query().Get("id")
	if collectionID == "" {
		http.NotFound(w, r)
		return
	}

	// Get collection details
	dbMutex.RLock()
	var c Collection
	err := db.QueryRow(
		"SELECT id, name, address, created_date FROM collections WHERE id = ?",
		collectionID,
	).Scan(&c.ID, &c.Name, &c.Address, &c.CreatedDate)

	if err != nil {
		dbMutex.RUnlock()
		http.NotFound(w, r)
		return
	}

	// Get images for this collection
	rows, err := db.Query(
		"SELECT id, collection_id, name, type, alcohol_content, net_content, origin, filename, upload_date FROM images WHERE collection_id = ? ORDER BY upload_date DESC",
		collectionID,
	)
	dbMutex.RUnlock()

	if err != nil {
		http.Error(w, "Error loading images", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var img Image
		err := rows.Scan(&img.ID, &img.CollectionID, &img.Name, &img.Type,
			&img.Alcohol_content, &img.Net_content, &img.Origin, &img.Filename, &img.UploadDate)
		if err != nil {
			fmt.Println("Err:", err)
			continue
		}

		c.Images = append(c.Images, img)
	}
	c.ImageCount = len(c.Images)

	fmt.Println("hi")

	templates.ExecuteTemplate(w, "collection.html", c)
}

func galleryHandler(w http.ResponseWriter, r *http.Request) {
	collections, err := getCollections()
	if err != nil {
		http.Error(w, "Error loading collections", http.StatusInternalServerError)
		return
	}

	var currentlyProcessing int
	dbMutex.RLock()
	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM (
			SELECT COUNT(i.id) as image_count
        	FROM collections c
        	LEFT JOIN images i ON c.id = i.collection_id
        	GROUP BY c.id
			HAVING c.processed < image_count AND image_count > 0
        	ORDER BY c.created_date DESC
		)`,
	).Scan(&currentlyProcessing)
	dbMutex.RUnlock()

	if err != nil {
		http.Error(w, "Error loading collections", http.StatusInternalServerError)
		return
	}

	//this is a remnant of something else the AI did
	data := struct {
		Collections         []Collection
		CurrentlyProcessing int
	}{
		Collections:         collections,
		CurrentlyProcessing: currentlyProcessing,
	}

	templates.ExecuteTemplate(w, "gallery.html", data)
}

func viewCollectionHandler(w http.ResponseWriter, r *http.Request) {
	collectionID := r.URL.Query().Get("id")
	if collectionID == "" {
		http.NotFound(w, r)
		return
	}

	dbMutex.RLock()
	var c Collection
	err := db.QueryRow(
		"SELECT id, name, address, created_date FROM collections WHERE id = ?",
		collectionID,
	).Scan(&c.ID, &c.Name, &c.Address, &c.CreatedDate)

	if err != nil {
		dbMutex.RUnlock()
		http.NotFound(w, r)
		return
	}

	rows, err := db.Query(
		"SELECT id, collection_id, name, type, alcohol_content, net_content, origin, filename, processing_data, upload_date FROM images WHERE collection_id = ? ORDER BY upload_date DESC",
		collectionID,
	)
	dbMutex.RUnlock()

	if err != nil {
		http.Error(w, "Error loading images", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var img Image
		var processing_data string
		err := rows.Scan(&img.ID, &img.CollectionID, &img.Name, &img.Type,
			&img.Alcohol_content, &img.Net_content, &img.Origin, &img.Filename, &processing_data, &img.UploadDate)
		if err != nil {
			fmt.Println("viewCollectionHandler error: ", err)
			continue
		}
		//get processed data as struct
		err = json.Unmarshal([]byte(processing_data), &img.Processing_data)
		if err != nil {
			fmt.Println("JSON unmarshal error", err)
			continue
		}
		c.Images = append(c.Images, img)
	}

	err = templates.ExecuteTemplate(w, "collection.html", c)
	if err != nil {
		fmt.Println("viewCollectionHandler template error:", err)
	}
}

func viewImageHandler(w http.ResponseWriter, r *http.Request) {
	// Extract image ID from URL: /image/123
	id := strings.TrimPrefix(r.URL.Path, "/image/")

	dbMutex.RLock()
	var img Image
	var processing_data string
	err := db.QueryRow(
		"SELECT id, collection_id, name, type, alcohol_content, net_content, origin, filename, processing_data, upload_date FROM images WHERE id = ?",
		id,
	).Scan(&img.ID, &img.CollectionID, &img.Name, &img.Type,
		&img.Alcohol_content, &img.Net_content, &img.Origin, &img.Filename, &processing_data, &img.UploadDate)
	dbMutex.RUnlock()

	//get processed data as struct
	err = json.Unmarshal([]byte(processing_data), &img.Processing_data)
	if err != nil {
		fmt.Println("JSON unmarshal error", err)
	}
	if err != nil {
		http.NotFound(w, r)
		return
	}

	//render single image template
	templates.ExecuteTemplate(w, "singleimage.html", img)
}

func formatTags(tags string) string {
	if tags == "" {
		return ""
	}
	tagList := strings.Split(tags, ",")
	var htmlTags string
	for _, tag := range tagList {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			htmlTags += fmt.Sprintf(`<span class="tag">%s</span>`, tag)
		}
	}
	return htmlTags
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Handle both /delete/ and /delete?id= patterns
	id := r.FormValue("id")
	if id == "" {
		id = strings.TrimPrefix(r.URL.Path, "/delete/")
	}

	// Get filename before deleting
	dbMutex.RLock()
	var filename string
	err := db.QueryRow("SELECT filename FROM images WHERE id = ?", id).Scan(&filename)
	dbMutex.RUnlock()

	if err != nil {
		http.Error(w, "Image not found", http.StatusNotFound)
		return
	}

	// Delete from database
	dbMutex.Lock()
	_, err = db.Exec("DELETE FROM images WHERE id = ?", id)
	dbMutex.Unlock()

	if err != nil {
		http.Error(w, "Error deleting image", http.StatusInternalServerError)
		return
	}

	// Delete file from filesystem
	os.Remove(filepath.Join("uploads", filename))

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func deleteCollectionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.FormValue("id")
	if id == "" {
		id = strings.TrimPrefix(r.URL.Path, "/delete-collection/")
	}

	// Get all filenames in collection
	dbMutex.RLock()
	rows, err := db.Query("SELECT filename FROM images WHERE collection_id = ?", id)
	if err == nil {
		var filenames []string
		for rows.Next() {
			var filename string
			rows.Scan(&filename)
			filenames = append(filenames, filename)
		}
		rows.Close()

		// Delete files
		for _, filename := range filenames {
			os.Remove(filepath.Join("uploads", filename))
		}
	}
	dbMutex.RUnlock()

	// Delete from database
	dbMutex.Lock()
	db.Exec("DELETE FROM images WHERE collection_id = ?", id)
	db.Exec("DELETE FROM collections WHERE id = ?", id)
	dbMutex.Unlock()

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func deleteUnprocessedCollections() {
	//get all unprocessed collections
	var collections []int
	dbMutex.RLock()
	rowsOuter, err := db.Query(`
        SELECT c.id, COUNT(i.id) as image_count
        FROM collections c
        LEFT JOIN images i ON c.id = i.collection_id
        GROUP BY c.id
		HAVING c.processed < image_count
    `)
	if err != nil {
		log.Fatal("delete unprocessed collections sql errer", err)
		return
	}
	for rowsOuter.Next() {
		var colID int
		var temp int
		rowsOuter.Scan(&colID, &temp)
		collections = append(collections, colID)
	}
	defer rowsOuter.Close()
	dbMutex.RUnlock()

	for _, colID := range collections {
		fmt.Println("Purging collection", colID)

		// Get all filenames in collection
		dbMutex.RLock()
		rows, err := db.Query("SELECT filename FROM images WHERE collection_id = ?", colID)
		if err == nil {
			var filenames []string
			for rows.Next() {
				var filename string
				rows.Scan(&filename)
				filenames = append(filenames, filename)
			}
			rows.Close()

			// Delete files
			for _, filename := range filenames {
				os.Remove(filepath.Join("uploads", filename))
			}
		}
		dbMutex.RUnlock()

		// Delete from database
		dbMutex.Lock()
		db.Exec("DELETE FROM images WHERE collection_id = ?", colID)
		db.Exec("DELETE FROM collections WHERE id = ?", colID)
		dbMutex.Unlock()
	}
}
