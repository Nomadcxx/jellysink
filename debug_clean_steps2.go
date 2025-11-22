package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"github.com/Nomadcxx/jellysink/internal/scanner"
)

func removeSpecificYearLocal(name, year string) string {
	if year == "" {
		return name
	}
	patterns := []string{"(" + year + ")", "[" + year + "]", "." + year + ".", " " + year + " ", " " + year + ""}
	for _, p := range patterns {
		name = strings.ReplaceAll(name, p, " ")
	}
	return name
}

func debugCase(input string) {
	fmt.Println("\n=== CASE: ", input)
	ext := strings.ToLower(filepath.Ext(input))
	name := input
	if ext != "" {
		name = strings.TrimSuffix(name, ext)
	}
	fmt.Println("After ext strip: '" + name + "'")

	year := scanner.ExtractYear(name)
	fmt.Println("ExtractYear:", year)

	if year != "" {
		idx := strings.LastIndex(name, year)
		fmt.Println("LastIndex of year:", idx)
		if idx != -1 {
			nameBefore := strings.TrimSpace(name[:idx])
			fmt.Println("Slice before year:", nameBefore)
			name = nameBefore
		}
	}

	stripped := scanner.StripReleaseGroup(name)
	fmt.Println("After StripReleaseGroup:", stripped)

	removedYear := removeSpecificYearLocal(stripped, year)
	fmt.Println("After removeSpecificYearLocal:", removedYear)

	// We cannot call unexported stripOrphanedReleaseGroups directly, so call CleanMovieName to show final
	final := scanner.CleanMovieName(input)
	fmt.Println("Final CleanMovieName:", final)
}

func main() {
	cases := []string{
		"The Pentagon Wars 1998 1080p WEBRip X264 Ac3 SNAKE/The Pentagon Wars 1998 1080p WEBRip X264 Ac3 SNAKE.mkv",
		"Invasion.of.the.Body.Snatchers.1956.DVDRip.Plus.Commentary.x264-MaG-Chamele0n.mkv",
		"men.at.work.1990.720p.bluray.x264-psychd-ml.mkv",
		"Vite.vendute.2024.1080p.H264.iTA.Fre.AC3.5.1.Sub.iTA.EnG.NUEnG.AsPiDe-.MIRCrew.mkv",
		"Thelma.O.Unicornio.2024.1080p.NF.WEB-DL.DDP5.1.Atmos.x264.DUAL-CBR.mkv",
	}

	for _, c := range cases {
		debugCase(c)
	}
}
