package main

import (
	"auth-service/cmd/api/config"
	data "auth-service/cmd/api/models"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/rpc"
	"os"
	"time"

	"github.com/go-chi/chi"
	"github.com/jackc/pgx/v5"
)

const PORT = 3001

func main() {

	// Connect to DB
	conn := connDB()

	// prepare the RPC client
	client := initRPCClient()

	defer client.Close()

	// Define app
	app := config.AppConfig{
		Router: chi.NewRouter(),
		DB:     conn,
		Models: data.New(conn),
		RPC:    client,
	}

	// Initialize Middlewares & Routes
	initMiddlewares(&app)
	initRoutes(&app)

	fmt.Printf("Starting auth-service in port %d \n", PORT)

	// http-server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", PORT),
		Handler: app.Router,
	}

	err := srv.ListenAndServe()

	if err != nil {
		fmt.Println("Failed to start auth-service")
		log.Panic(err)
	}
}

func connDB() *pgx.Conn {

	var retries int

	for {
		conn, err := pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
		if err == nil {
			fmt.Println("Connected to DB")
			return conn
		}

		retries++

		if retries > 10 {
			fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
			os.Exit(1)
		}

		wait := (time.Second / 2) * time.Duration(retries)
		time.Sleep(wait)
		fmt.Printf("DB not ready, retrying after %v \n", wait)
	}

}

func initRPCClient() *rpc.Client {
	// Create a TCP connection to localhost on port 1234
	rpcEndpoint := os.Getenv("RPC_ENDPOINT")
	client, err := rpc.Dial("tcp", rpcEndpoint)
	if err != nil {
		log.Fatal("Connection error: ", err)
	}

	fmt.Println("Connected to RPC Server")

	return client
}
