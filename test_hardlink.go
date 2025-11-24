package main

import (
	"fmt"
	"os"
	"syscall"
)

func main() {
	src := "/mnt/STORAGE2/MOVIES/Swordfish (2001)/Swordfish 2001 HEVC D3FiL3R (bd50)/Swordfish 2001 BluRay 1080p DD5.1 x265-D3FiL3R.mkv"
	target := "/mnt/STORAGE2/MOVIES/Swordfish (2001)/Swordfish (2001).mkv"

	srcInfo, err := os.Stat(src)
	if err != nil {
		fmt.Printf("Error stat source: %v\n", err)
		return
	}

	targetInfo, err := os.Stat(target)
	if err != nil {
		fmt.Printf("Error stat target: %v\n", err)
		return
	}

	srcSys := srcInfo.Sys().(*syscall.Stat_t)
	targetSys := targetInfo.Sys().(*syscall.Stat_t)

	fmt.Printf("Source inode: %d\n", srcSys.Ino)
	fmt.Printf("Target inode: %d\n", targetSys.Ino)
	fmt.Printf("Same file: %v\n", srcSys.Ino == targetSys.Ino)
}
