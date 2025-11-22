package main

import (
	"fmt"
	"github.com/Nomadcxx/jellysink/internal/scanner"
	"regexp"
	"strings"
)

func trace(input string) {
	fmt.Printf("\n== TRACE for: %s ==\n", input)
	name := strings.ReplaceAll(input, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")
	fmt.Printf("start: '%s'\n", name)

	// replicate release patterns from fuzzy.go
	patterns := scannerGetPatterns()
	for i, pat := range patterns {
		re := regexp.MustCompile(`(?i)` + pat)
		if re.MatchString(name) {
			before := name
			name = re.ReplaceAllString(name, " ")
			fmt.Printf("pat %03d matched: %s -> %s\n", i, pat, strings.TrimSpace(name))
		}
	}
	fmt.Printf("after pass: '%s'\n", strings.TrimSpace(name))
}

// scannerGetPatterns pulls the raw patterns from the compiled binary via reflection - but we can't import internals easily.
// So, for this debug, we hardcode a few patterns relevant to cases.
func scannerGetPatterns() []string {
	return []string{
		`\b\d{3,4}[pi]\b`,
		`\b(DVDRip|BluRay|x264|x265|HEVC|WEB-DL|WEBRip|HDTV|REMUX|BRRip|BDRip)\b`,
		`\b(Plus Commentary|Commentary|Audio Commentary)\b`,
		`\b(Plus|Extended Commentary)\b`,
		`\d+Audio`,
		`MA\d+\s\d+`,
		`\b\d\.\d\b`,
		`-[A-Za-z0-9]+(-[A-Za-z0-9]+)*$`,
		`\s?[-~]\s?[A-Za-z0-9]+(\s[A-Za-z0-9]+)*$`,
		`\[.*?\]`,
	}
}

func main() {
	cases := []string{
		"Invasion.of.the.Body.Snatchers.1956.DVDRip.Plus.Commentary.x264-MaG-Chamele0n.mkv",
		"men.at.work.1990.720p.bluray.x264-psychd-ml.mkv",
		"Vite.vendute.2024.1080p.H264.iTA.Fre.AC3.5.1.Sub.iTA.EnG.NUEnG.AsPiDe-.MIRCrew.mkv",
		"Thelma.O.Unicornio.2024.1080p.NF.WEB-DL.DDP5.1.Atmos.x264.DUAL-CBR.mkv",
	}

	for _, c := range cases {
		trace(c)
	}
}
