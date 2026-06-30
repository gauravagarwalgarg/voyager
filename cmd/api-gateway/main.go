package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// ─── Data Models ────────────────────────────────────────────────────────────────

type Image struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
}

type Fact struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Category string `json:"category"`
	Source   string `json:"source"`
}

type FeedItem struct {
	Image Image  `json:"image"`
	Fact  Fact   `json:"fact"`
	Score float64 `json:"score"`
}

type TelemetryData struct {
	GatewayThroughput    int     `json:"gateway_throughput_rps"`
	ImageQueueDepth      int     `json:"image_queue_depth"`
	NSQMessageBacklog    int     `json:"nsq_message_backlog"`
	WorkerUtilization    float64 `json:"worker_utilization_pct"`
	PostgresConnections  int     `json:"postgres_connections"`
	PostgresMaxConns     int     `json:"postgres_max_connections"`
	MinioStorageUsedMB   int     `json:"minio_storage_used_mb"`
	UptimeSeconds        int64   `json:"uptime_seconds"`
	ActiveTraces         int     `json:"active_traces"`
	ServiceStatus        map[string]string `json:"service_status"`
}

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Total      int         `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

// ─── In-Memory Store ────────────────────────────────────────────────────────────

var (
	images   []Image
	facts    []Fact
	mu       sync.RWMutex
	startTime time.Time
)

func seedData() {
	startTime = time.Now()

	images = []Image{
		{ID: "img-001", URL: "https://images.unsplash.com/photo-1446776811953-b23d57bd21aa?w=800", Title: "Earth from Space", Description: "Our pale blue dot as seen from orbit", Width: 800, Height: 600, Tags: []string{"space", "earth", "nasa"}, CreatedAt: time.Now().Add(-72 * time.Hour)},
		{ID: "img-002", URL: "https://images.unsplash.com/photo-1451187580459-43490279c0fa?w=800", Title: "Digital Network", Description: "Abstract visualization of global connectivity", Width: 800, Height: 533, Tags: []string{"technology", "network", "abstract"}, CreatedAt: time.Now().Add(-60 * time.Hour)},
		{ID: "img-003", URL: "https://images.unsplash.com/photo-1507003211169-0a1dd7228f2d?w=800", Title: "Northern Lights", Description: "Aurora borealis dancing over frozen lake", Width: 800, Height: 600, Tags: []string{"nature", "aurora", "night"}, CreatedAt: time.Now().Add(-48 * time.Hour)},
		{ID: "img-004", URL: "https://images.unsplash.com/photo-1614728263952-84ea256f9679?w=800", Title: "Mars Surface", Description: "Red planet terrain captured by rover", Width: 800, Height: 533, Tags: []string{"space", "mars", "exploration"}, CreatedAt: time.Now().Add(-36 * time.Hour)},
		{ID: "img-005", URL: "https://images.unsplash.com/photo-1518770660439-4636190af475?w=800", Title: "Circuit Board", Description: "Macro shot of a processor chip", Width: 800, Height: 600, Tags: []string{"technology", "hardware", "macro"}, CreatedAt: time.Now().Add(-30 * time.Hour)},
		{ID: "img-006", URL: "https://images.unsplash.com/photo-1470071459604-3b5ec3a7fe05?w=800", Title: "Forest Canopy", Description: "Sunlight filtering through ancient trees", Width: 800, Height: 533, Tags: []string{"nature", "forest", "light"}, CreatedAt: time.Now().Add(-24 * time.Hour)},
		{ID: "img-007", URL: "https://images.unsplash.com/photo-1462331940025-496dfbfc7564?w=800", Title: "Nebula", Description: "Stellar nursery in deep space", Width: 800, Height: 600, Tags: []string{"space", "nebula", "stars"}, CreatedAt: time.Now().Add(-20 * time.Hour)},
		{ID: "img-008", URL: "https://images.unsplash.com/photo-1550751827-4bd374c3f58b?w=800", Title: "Cybersecurity", Description: "Data encryption visualization", Width: 800, Height: 533, Tags: []string{"technology", "security", "data"}, CreatedAt: time.Now().Add(-16 * time.Hour)},
		{ID: "img-009", URL: "https://images.unsplash.com/photo-1505506874110-6a7a69069a08?w=800", Title: "Milky Way", Description: "Galaxy core visible over mountain range", Width: 800, Height: 600, Tags: []string{"space", "galaxy", "photography"}, CreatedAt: time.Now().Add(-12 * time.Hour)},
		{ID: "img-010", URL: "https://images.unsplash.com/photo-1441974231531-c6227db76b6e?w=800", Title: "Rainforest", Description: "Biodiversity hotspot in the Amazon", Width: 800, Height: 533, Tags: []string{"nature", "rainforest", "biodiversity"}, CreatedAt: time.Now().Add(-8 * time.Hour)},
		{ID: "img-011", URL: "https://images.unsplash.com/photo-1488590528505-98d2b5aba04b?w=800", Title: "Code Editor", Description: "Developer workspace with multiple monitors", Width: 800, Height: 600, Tags: []string{"technology", "coding", "workspace"}, CreatedAt: time.Now().Add(-4 * time.Hour)},
		{ID: "img-012", URL: "https://images.unsplash.com/photo-1484589065579-248aad0d8b13?w=800", Title: "Saturn Rings", Description: "Gas giant and its magnificent ring system", Width: 800, Height: 533, Tags: []string{"space", "saturn", "planets"}, CreatedAt: time.Now().Add(-1 * time.Hour)},
	}

	facts = []Fact{
		{ID: "fact-001", Text: "A teaspoon of neutron star material would weigh about 6 billion tons on Earth.", Category: "space", Source: "NASA"},
		{ID: "fact-002", Text: "The human brain can process information at speeds up to 120 meters per second.", Category: "science", Source: "Nature Journal"},
		{ID: "fact-003", Text: "There are more trees on Earth than stars in the Milky Way galaxy.", Category: "nature", Source: "Yale School of Forestry"},
		{ID: "fact-004", Text: "The first computer programmer was Ada Lovelace, who wrote algorithms for Charles Babbage's Analytical Engine in 1843.", Category: "technology", Source: "Computer History Museum"},
		{ID: "fact-005", Text: "The Great Wall of China is not visible from space with the naked eye, contrary to popular belief.", Category: "history", Source: "NASA"},
		{ID: "fact-006", Text: "Light from the Sun takes approximately 8 minutes and 20 seconds to reach Earth.", Category: "space", Source: "ESA"},
		{ID: "fact-007", Text: "Octopuses have three hearts, nine brains, and blue blood.", Category: "nature", Source: "Marine Biology Review"},
		{ID: "fact-008", Text: "The first email was sent in 1971 by Ray Tomlinson to himself.", Category: "technology", Source: "Internet Society"},
		{ID: "fact-009", Text: "Water can exist in three states simultaneously at 0.01°C and 611.73 pascals - the triple point.", Category: "science", Source: "Physics Today"},
		{ID: "fact-010", Text: "The Library of Alexandria held an estimated 400,000 scrolls at its peak.", Category: "history", Source: "Ancient History Encyclopedia"},
		{ID: "fact-011", Text: "A day on Venus is longer than a year on Venus - it takes 243 Earth days to rotate once.", Category: "space", Source: "NASA JPL"},
		{ID: "fact-012", Text: "Honey never spoils. Archaeologists have found 3000-year-old honey in Egyptian tombs that was still edible.", Category: "science", Source: "Smithsonian"},
		{ID: "fact-013", Text: "The Amazon rainforest produces about 20% of the world's oxygen supply.", Category: "nature", Source: "WWF"},
		{ID: "fact-014", Text: "The first 1GB hard drive, introduced in 1980, weighed about 550 pounds and cost $40,000.", Category: "technology", Source: "IBM Archives"},
		{ID: "fact-015", Text: "The Roman Empire's road network covered over 250,000 miles at its peak.", Category: "history", Source: "Oxford Classical Dictionary"},
		{ID: "fact-016", Text: "Tardigrades can survive in the vacuum of space, extreme temperatures, and radiation.", Category: "science", Source: "Current Biology"},
		{ID: "fact-017", Text: "A single bolt of lightning contains enough energy to toast 100,000 slices of bread.", Category: "nature", Source: "National Geographic"},
		{ID: "fact-018", Text: "The International Space Station travels at approximately 17,500 mph, orbiting Earth every 90 minutes.", Category: "space", Source: "NASA"},
		{ID: "fact-019", Text: "ENIAC, the first general-purpose computer, weighed 30 tons and occupied 1,800 square feet.", Category: "technology", Source: "University of Pennsylvania"},
		{ID: "fact-020", Text: "Cleopatra lived closer in time to the Moon landing than to the construction of the Great Pyramid.", Category: "history", Source: "Ancient History Review"},
	}
}

// ─── Middleware ──────────────────────────────────────────────────────────────────

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ─── Handlers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func handleGetImages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	mu.RLock()
	defer mu.RUnlock()

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 50 {
		pageSize = 12
	}

	tag := r.URL.Query().Get("tag")

	filtered := make([]Image, 0)
	for _, img := range images {
		if tag != "" {
			found := false
			for _, t := range img.Tags {
				if t == tag {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		filtered = append(filtered, img)
	}

	total := len(filtered)
	totalPages := (total + pageSize - 1) / pageSize
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:       filtered[start:end],
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

func handleGetImageByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract ID from path: /v1/images/{id}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/images/"), "/")
	id := parts[0]

	mu.RLock()
	defer mu.RUnlock()

	for _, img := range images {
		if img.ID == id {
			writeJSON(w, http.StatusOK, img)
			return
		}
	}

	writeError(w, http.StatusNotFound, "image not found")
}

func handlePostImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse multipart form (max 32MB)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form: "+err.Error())
		return
	}

	title := r.FormValue("title")
	description := r.FormValue("description")
	tags := strings.Split(r.FormValue("tags"), ",")

	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "image file required: "+err.Error())
		return
	}
	defer file.Close()

	// Mock: generate an ID and store the metadata (no actual MinIO upload in mock mode)
	newID := fmt.Sprintf("img-%03d", len(images)+1)
	newImage := Image{
		ID:          newID,
		URL:         fmt.Sprintf("http://localhost:9000/voyager-images/%s/%s", newID, filepath.Base(header.Filename)),
		Title:       title,
		Description: description,
		Width:       800,
		Height:      600,
		Tags:        tags,
		CreatedAt:   time.Now(),
	}

	mu.Lock()
	images = append(images, newImage)
	mu.Unlock()

	writeJSON(w, http.StatusCreated, newImage)
}

func handleImages(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/images")
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		switch r.Method {
		case http.MethodGet:
			handleGetImages(w, r)
		case http.MethodPost:
			handlePostImage(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// /v1/images/{id}
	handleGetImageByID(w, r)
}

func handleFeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	mu.RLock()
	defer mu.RUnlock()

	feed := make([]FeedItem, 0, len(images))
	for _, img := range images {
		fact := facts[rand.Intn(len(facts))]
		feed = append(feed, FeedItem{
			Image: img,
			Fact:  fact,
			Score: rand.Float64(),
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": feed,
		"total": len(feed),
	})
}

func handleRandomFact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	mu.RLock()
	defer mu.RUnlock()

	fact := facts[rand.Intn(len(facts))]
	writeJSON(w, http.StatusOK, fact)
}

func handleFactCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	mu.RLock()
	defer mu.RUnlock()

	categorySet := make(map[string]int)
	for _, f := range facts {
		categorySet[f.Category]++
	}

	type CategoryInfo struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	categories := make([]CategoryInfo, 0)
	for name, count := range categorySet {
		categories = append(categories, CategoryInfo{Name: name, Count: count})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"categories": categories,
	})
}

func handleTelemetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	uptime := int64(time.Since(startTime).Seconds())

	telemetry := TelemetryData{
		GatewayThroughput:   120 + rand.Intn(80),
		ImageQueueDepth:     rand.Intn(15),
		NSQMessageBacklog:   rand.Intn(50),
		WorkerUtilization:   30.0 + rand.Float64()*55.0,
		PostgresConnections: 5 + rand.Intn(15),
		PostgresMaxConns:    25,
		MinioStorageUsedMB:  256 + rand.Intn(128),
		UptimeSeconds:       uptime,
		ActiveTraces:        3 + rand.Intn(12),
		ServiceStatus: map[string]string{
			"api-gateway": "healthy",
			"image-svc":   "healthy",
			"facts-svc":   "healthy",
			"worker-svc":  "healthy",
			"postgres":    "healthy",
			"minio":       "healthy",
			"nsq":         "healthy",
		},
	}

	writeJSON(w, http.StatusOK, telemetry)
}

// ─── Main ───────────────────────────────────────────────────────────────────────

func main() {
	// Initialize structured logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Seed in-memory data
	seedData()

	logger.Info("Starting Voyager API Gateway",
		zap.String("version", "0.2.0"),
		zap.String("http_port", "8080"),
		zap.Int("images_seeded", len(images)),
		zap.Int("facts_seeded", len(facts)),
	)

	// HTTP server
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","service":"api-gateway","version":"0.2.0","uptime_seconds":%d}`, int64(time.Since(startTime).Seconds()))
	})

	// Metrics endpoint (Prometheus)
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "# HELP voyager_gateway_requests_total Total HTTP requests\n")
		fmt.Fprintf(w, "# TYPE voyager_gateway_requests_total counter\n")
		fmt.Fprintf(w, "voyager_gateway_requests_total{method=\"GET\",path=\"/v1/images\"} %d\n", 100+rand.Intn(500))
		fmt.Fprintf(w, "voyager_gateway_requests_total{method=\"GET\",path=\"/v1/feed\"} %d\n", 50+rand.Intn(200))
		fmt.Fprintf(w, "voyager_gateway_requests_total{method=\"GET\",path=\"/v1/facts/random\"} %d\n", 200+rand.Intn(300))
		fmt.Fprintf(w, "# HELP voyager_gateway_uptime_seconds Gateway uptime\n")
		fmt.Fprintf(w, "# TYPE voyager_gateway_uptime_seconds gauge\n")
		fmt.Fprintf(w, "voyager_gateway_uptime_seconds %d\n", int64(time.Since(startTime).Seconds()))
	})

	// API routes
	mux.HandleFunc("/v1/images", handleImages)
	mux.HandleFunc("/v1/images/", handleImages)
	mux.HandleFunc("/v1/feed", handleFeed)
	mux.HandleFunc("/v1/facts/random", handleRandomFact)
	mux.HandleFunc("/v1/facts/categories", handleFactCategories)
	mux.HandleFunc("/v1/telemetry", handleTelemetry)

	// Serve frontend static files
	frontendDir := "./frontend/dist"
	if _, err := os.Stat(frontendDir); os.IsNotExist(err) {
		// Fallback: try relative to binary location
		frontendDir = "frontend/dist"
	}
	fs := http.FileServer(http.Dir(frontendDir))
	mux.Handle("/", fs)

	// Wrap with CORS
	handler := corsMiddleware(mux)

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	logger.Info("API Gateway running",
		zap.String("addr", ":8080"),
		zap.String("frontend", frontendDir),
	)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited cleanly")
}
