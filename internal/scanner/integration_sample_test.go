package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegrationSampleScan(t *testing.T) {
	tmpDir := t.TempDir()

	// Helper to create files
	writeFile := func(path string) {
		full := filepath.Join(tmpDir, path)
		dir := filepath.Dir(full)
		if dir != "" {
			os.MkdirAll(dir, 0755)
		}
		os.WriteFile(full, []byte("movie"), 0644)
	}

	// 1. The Pentagon Wars - both compliant and release-group
	writeFile("The Pentagon Wars (1998)/The Pentagon Wars (1998).mkv")
	writeFile("The.Pentagon.Wars.1998.1080p.WEBRip.X264.Ac3-SNAKE/The.Pentagon.Wars.1998.1080p.WEBRip.X264.Ac3-SNAKE.mkv")

	// 2. Pinocchio - release group only
	writeFile("Pinocchio.1940.1080p.BluRay.H264.AC3.DD5.1.Will1869/Pinocchio.1940.1080p.BluRay.H264.AC3.DD5.1.Will1869.mkv")

	// 3. Thelma/El Unicornio - foreign file in release group folder but parent prefer
	writeFile("Thelma the Unicorn (2024)/Thelma.O.Unicornio.2024.1080p.NF.WEB-DL.DDP5.1.Atmos.x264.DUAL-CBR.mkv")

	// 4. Men At Work hyphenated group
	writeFile("men.at.work.1990.720p.bluray.x264-psychd-ml/men.at.work.1990.720p.bluray.x264-psychd-ml.mkv")

	// 5. Vite Vendute - foreign language tokens
	writeFile("Vite.vendute.2024.1080p.H264.iTA.Fre.AC3.5.1.Sub.iTA.EnG.NUEnG.AsPiDe-/Vite.vendute.2024.1080p.H264.iTA.Fre.AC3.5.1.Sub.iTA.EnG.NUEnG.AsPiDe-.mkv")

	// Run duplicate scan
	dups, err := ScanMovies([]string{tmpDir})
	if err != nil {
		t.Fatalf("ScanMovies() error: %v", err)
	}

	// Expect at least one duplicate group (Pentagon Wars)
	foundPentagon := false
	for _, g := range dups {
		if strings.Contains(g.NormalizedName, "pentagon") && g.Year == "1998" && len(g.Files) == 2 {
			foundPentagon = true
			break
		}
	}
	if !foundPentagon {
		t.Errorf("Expected Pentagon Wars duplicate group; groups: %#v", dups)
	}

	// Run compliance scan excluding duplicates for deletion
	// Mark keep/delete
	marked := MarkKeepDelete(dups)
	deleteList := GetDeleteList(marked)

	issues, err := ScanMovieCompliance([]string{tmpDir}, deleteList...)
	if err != nil {
		t.Fatalf("ScanMovieCompliance() error: %v", err)
	}

	// Expect issues for Pinocchio, Men At Work, Vite Vendute (release-group naming)
	paths := make(map[string]bool)
	for _, iss := range issues {
		paths[iss.Path] = true
	}

	if !paths[filepath.Join(tmpDir, "Pinocchio.1940.1080p.BluRay.H264.AC3.DD5.1.Will1869", "Pinocchio.1940.1080p.BluRay.H264.AC3.DD5.1.Will1869.mkv")] {
		t.Error("Expected Pinocchio to be reported as compliance issue")
	}
	if !paths[filepath.Join(tmpDir, "men.at.work.1990.720p.bluray.x264-psychd-ml", "men.at.work.1990.720p.bluray.x264-psychd-ml.mkv")] {
		t.Error("Expected Men At Work to be reported as compliance issue")
	}
	if !paths[filepath.Join(tmpDir, "Vite.vendute.2024.1080p.H264.iTA.Fre.AC3.5.1.Sub.iTA.EnG.NUEnG.AsPiDe-", "Vite.vendute.2024.1080p.H264.iTA.Fre.AC3.5.1.Sub.iTA.EnG.NUEnG.AsPiDe-.mkv")] {
		t.Error("Expected Vite Vendute to be reported as compliance issue")
	}

	// Thelma is in parent folder Thelma the Unicorn (2024) and should suggest reorganization
	expectedThelmaSuggested := filepath.Join(tmpDir, "Thelma The Unicorn (2024)", "Thelma The Unicorn (2024).mkv")
	foundThelma := false
	for _, iss := range issues {
		if filepath.Base(iss.Path) == "Thelma.O.Unicornio.2024.1080p.NF.WEB-DL.DDP5.1.Atmos.x264.DUAL-CBR.mkv" {
			if iss.SuggestedPath == expectedThelmaSuggested {
				foundThelma = true
			}
		}
	}
	if !foundThelma {
		t.Errorf("Expected Thelma to be suggested path %s, issues: %+v", expectedThelmaSuggested, issues)
	}
}
