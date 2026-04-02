package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/modfin/bellman/tools/ptc/bench/bfcl"
	"github.com/modfin/bellman/tools/ptc/bench/cfb"
	"github.com/modfin/bellman/tools/ptc/bench/nestful"
)

func main() {
	// Create persistent handler caches
	bfclCache := bfcl.NewCache()
	cfbCache := cfb.NewCache()

	// Register API Endpoint
	http.HandleFunc("/bfcl", bfclCache.HandleGenerateBFCL)
	http.HandleFunc("/cfb", cfbCache.HandleGenerateCFB)
	http.HandleFunc("/nestful", nestful.NesfulHandlerFromEnv())

	fmt.Println("---------------------------------------------------------")
	fmt.Println(" Toolman Bench Server Running")
	fmt.Println(" BFCL API Endpoint:   		http://localhost:8080/bfcl")
	fmt.Println(" CFB API Endpoint:    		http://localhost:8080/cfb")
	fmt.Println(" NESTFUL API Endpoint:    	http://localhost:8080/nestful")
	fmt.Println("---------------------------------------------------------")

	fmt.Println("Toolman Benchmark Server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
