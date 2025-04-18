// Package main är huvudpaketet för e-Arkive backend-servern
package main

import (
	"database/sql"
	"graphql-backend/graph"
	"log"
	"net"
	"net/http"
	"os"

	_ "github.com/mattn/go-sqlite3"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/rs/cors"
	"github.com/vektah/gqlparser/v2/ast"
)

// Globala variabler
var db *sql.DB // Delar databasanslutningen genom hela applikationen

// Standardport för servern om ingen annan specificerats
const defaultPort = "8080"

// initDB initierar anslutningen till SQLite-databasen
// Skapar en ny databasfil om den inte redan finns
func initDB() {
	var err error
	connString := "./e-Arkive.db"
	db, err = sql.Open("sqlite3", connString)
	if err != nil {
		log.Fatalf("Failed to connect to SQLite database: %v", err)
	}

	// Verifiera databasanslutningen
	if err = db.Ping(); err != nil {
		log.Fatalf("Failed to ping SQLite database: %v", err)
	}

	// Skapa tabellerna om de inte redan finns
	createTables()

	log.Println("Connected to SQLite database successfully!")
}

// createTables skapar alla nödvändiga tabeller i databasen om de inte redan finns
func createTables() {
	// Skapa files-tabell
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS files (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			size INTEGER NOT NULL,
			content_type TEXT NOT NULL,
			created_at TEXT NOT NULL,
			file_data BLOB
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create files table: %v", err)
	}

	// Skapa metadata-tabell
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS metadata (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			file_id INTEGER NOT NULL,
			key TEXT NOT NULL,
			value TEXT NOT NULL,
			FOREIGN KEY (file_id) REFERENCES files (id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create metadata table: %v", err)
	}

	log.Println("Database tables created or already exist")
}

// logRequest loggar alla inkommande HTTP-förfrågningar
// Innehåller metod, sökväg och klientens IP-adress
func logRequest(r *http.Request) {
	log.Printf("%s %s %s", r.Method, r.URL.Path, r.RemoteAddr)
}

// logAction loggar viktiga händelser i systemet
func logAction(action string) {
	log.Printf("[ACTION] %s", action)
}

// getLocalIP hämtar serverns lokala IP-adress
// Används för att visa korrekt serveradress i loggarna
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Printf("Error getting local IP: %v", err)
		return "localhost"
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			return ipNet.IP.String()
		}
	}

	return "localhost"
}

// main är huvudfunktionen som startar servern
func main() {
	// Initierar databasen
	initDB()

	// Konfigurerar serverporten
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	// Skapar en ny GraphQL-server med vår schema och resolver
	srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: graph.NewResolver(db)}))

	// Konfigurerar tillåtna transportmetoder
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

	// Konfigurerar cache för query-optimering
	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	// Aktiverar GraphQL-tillägg
	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	// Konfigurerar endpoints
	http.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		logAction("GraphQL query received")
		srv.ServeHTTP(w, r)
	})

	http.HandleFunc("/graphiql", func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		http.ServeFile(w, r, "graphiql.html")
	})

	http.Handle("/sandbox", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(sandboxHTML)
	}))

	localIP := getLocalIP()

	// Konfigurerar CORS för att tillåta anrop från frontend
	handlerWithCORS := cors.New(cors.Options{
		AllowedOrigins: []string{"http://localhost:5173"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	}).Handler(http.DefaultServeMux)

	// Loggar serverinformation
	log.Printf("Server is running at http://%s:%s/query", localIP, port)
	log.Printf("GraphiQL is available at http://%s:%s/graphiql", localIP, port)
	log.Printf("Sandbox is available at http://%s:%s/sandbox", localIP, port)

	log.Printf("Server is starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, handlerWithCORS))
}

// sandboxHTML innehåller HTML för GraphQL Playground
var sandboxHTML = []byte(`
<!DOCTYPE html>
<html lang="en">
<body style="margin: 0; overflow-x: hidden; overflow-y: hidden">
<div id="sandbox" style="height:100vh; width:100vw;"></div>
<script src="https://embeddable-sandbox.cdn.apollographql.com/_latest/embeddable-sandbox.umd.production.min.js"></script>
<script>
 new window.EmbeddedSandbox({
   target: "#sandbox",
   // Pass through your server href if you are embedding on an endpoint.
   // Otherwise, you can pass whatever endpoint you want Sandbox to start up with here.
   initialEndpoint: "http://localhost:8080/query",
 });
 // advanced options: https://www.apollographql.com/docs/studio/explorer/sandbox#embedding-sandbox
</script>
</body>

</html>`)
