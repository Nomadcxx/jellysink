package main

import (
	"fmt"
	"github.com/Nomadcxx/jellysink/internal/scanner"
)

func main() {
	fmt.Println("Testing TVDB API...")

	tvdbKey := "b4a5a01e-02a8-4fd1-b2be-2ceed8afb4a4"
	client := scanner.NewTVDBClient(tvdbKey)

	// Test search for a well-known show
	results, err := client.SearchSeries("Breaking Bad")
	if err != nil {
		fmt.Printf("TVDB Error: %v\n", err)
	} else {
		fmt.Printf("TVDB Success! Found %d results\n", len(results))
		if len(results) > 0 {
			fmt.Printf("  First result: %s (Year: %s, First Aired: %s)\n",
				results[0].Name, results[0].Year, results[0].FirstAirTime)
		}
	}

	fmt.Println("\nTesting OMDB API...")

	// OMDB test would go here if we implement an OMDB client
	// For now, OMDB is not implemented in the scanner
	fmt.Println("OMDB integration not yet implemented in scanner")
}
