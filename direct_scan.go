package main

import (
	"fmt"
	"github.com/Nomadcxx/jellysink/internal/scanner"
)

func main() {
	paths := []string{"/tmp/test_jellysink_lib/MOVIES"}
	
	fmt.Println("Running compliance scan...")
	issues, err := scanner.ScanMovieCompliance(paths)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	fmt.Printf("\nFound %d compliance issues:\n\n", len(issues))
	
	for i, issue := range issues {
		fmt.Printf("%d. [%s] %s\n", i+1, issue.Type, issue.Problem)
		fmt.Printf("   Current: %s\n", issue.Path)
		fmt.Printf("   Fixed:   %s\n", issue.SuggestedPath)
		fmt.Printf("   Action:  %s\n\n", issue.SuggestedAction)
	}
}
