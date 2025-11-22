package main

import (
	"fmt"
	"github.com/Nomadcxx/jellysink/internal/scanner"
)

func debug(name string) {
	fmt.Printf("\n=== Debug: %s ===\n", name)
	fmt.Printf("Input: %s\n", name)
	fmt.Printf("CleanMovieName: %s\n", scanner.CleanMovieName(name))
}

func main() {
	cases := []string{
		"Invasion.of.the.Body.Snatchers.1956.DVDRip.Plus.Commentary.x264-MaG-Chamele0n.mkv",
		"men.at.work.1990.720p.bluray.x264-psychd-ml.mkv",
		"Vite.vendute.2024.1080p.H264.iTA.Fre.AC3.5.1.Sub.iTA.EnG.NUEnG.AsPiDe-.MIRCrew.mkv",
		"Thelma.O.Unicornio.2024.1080p.NF.WEB-DL.DDP5.1.Atmos.x264.DUAL-CBR.mkv",
	}

	for _, c := range cases {
		debug(c)
	}
}
