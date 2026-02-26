package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/joho/godotenv"
	"github.com/modfin/bellman/tools/ptc/bench/replay"
	"github.com/modfin/bellman/tools/ptc/bfcl"
	"github.com/modfin/bellman/tools/ptc/cfb"
)

func main() {
	// godotenv.Load() ...
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}

	// Create persistent cache and inject into handlers
	bfclReplay := &bfcl.Replay{ReplayCache: replay.NewCache()}
	cfbReplay := &cfb.Replay{ReplayCache: replay.NewCache()}

	// Register API Endpoint
	http.HandleFunc("/bfcl", MiddlewareDebugLogger("BFCL", bfclReplay.HandleGenerateBFCL))
	http.HandleFunc("/cfb", MiddlewareDebugLogger("CFB", cfbReplay.HandleGenerateCFB))

	// Register Debug UI Endpoints
	http.HandleFunc("/debug", HandleDebugUI)
	http.HandleFunc("/debug/api/data", HandleDebugData)
	http.HandleFunc("/debug/api/clear", HandleDebugClear)

	fmt.Println("---------------------------------------------------------")
	fmt.Println(" Toolman Bench Server Running")
	fmt.Println(" BFCL API Endpoint:	http://localhost:8080/bfcl")
	fmt.Println(" CFB API Endpoint:		http://localhost:8080/cfb")
	fmt.Println(" Debug UI:				http://localhost:8080/debug")
	fmt.Println("---------------------------------------------------------")

	fmt.Println("Toolman Benchmark Server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
