package main

import (
	"fmt"
	"github.com/Nomadcxx/jellysink/internal/scanner"
)

func main() {
	// Exact folder name from user's scan
	folderName := "The Pentagon Wars 1998 1080p WEBRip X264 Ac3 SNAKE"
	result := scanner.CleanMovieName(folderName)
	fmt.Printf("Input:  '%s'\n", folderName)
	fmt.Printf("Output: '%s'\n", result)
	
	if result == "The Pentagon Wars (1998)" {
		fmt.Println("✓ CORRECT")
	} else {
		fmt.Println("✗ WRONG - Still broken")
	}
	
	// Pinocchio case
	fmt.Println()
	folderName2 := "Pinocchio.1940.1080p.BluRay.H264.AC3.DD5.1.Will1869"
	result2 := scanner.CleanMovieName(folderName2)
	fmt.Printf("Input:  '%s'\n", folderName2)
	fmt.Printf("Output: '%s'\n", result2)
	
	if result2 == "Pinocchio (1940)" {
		fmt.Println("✓ CORRECT")
	} else {
		fmt.Println("✗ WRONG - Still broken")
	}
}
