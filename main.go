package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const rfc2822 = "Mon, 2 Jan 2006 15:04:05 -0700"

// Set the (a|m)time on `path` without following symlinks
func lutimes(path string, atime, mtime time.Time) error {
	times := []unix.Timespec{
		unix.NsecToTimespec(atime.UnixNano()),
		unix.NsecToTimespec(mtime.UnixNano()),
	}
	return unix.UtimesNanoAt(unix.AT_FDCWD, path, times, unix.AT_SYMLINK_NOFOLLOW)
}

func main() {
	lsFiles := exec.Command("git", "ls-files", "-z")

	out, err := lsFiles.Output()
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}

	dirMTimes := map[string]time.Time{}

	files := strings.Split(strings.TrimRight(string(out), "\x00"), "\x00")
	for _, file := range files {
		gitLog := exec.Command("git", "log", "-1", "--date=rfc2822", "--format=%cd", file)

		out, err := gitLog.Output()

		if err != nil {
			fmt.Fprint(os.Stderr, err)
			os.Exit(1)
		}

		mStr := strings.TrimSpace(strings.TrimLeft(string(out), "Date:"))
		mTime, err := time.Parse(rfc2822, mStr)

		if err != nil {
			fmt.Fprintf(os.Stderr, "%s on %s", err, file)
			os.Exit(1)
		}

		// Loop over each directory in the path to `file`, updating `dirMTimes`
		// to take the most recent time seen.
		dir := filepath.Dir(file)
		for {
			if other, ok := dirMTimes[dir]; ok {
				if mTime.After(other) {
					// file mTime is more recent than previous seen for 'dir'
					dirMTimes[dir] = mTime
				}
			} else {
				// first occurrence of dir
				dirMTimes[dir] = mTime
			}

			// Remove one directory from the path until it isn't changed anymore
			if dir == filepath.Dir(dir) {
				break
			}
			dir = filepath.Dir(dir)
		}

		err = lutimes(file, mTime, mTime)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s on %s", err, file)
			os.Exit(1)
		}

		fmt.Printf("%s: %s\n", file, mTime)
	}

	for dir, mTime := range dirMTimes {
		err = lutimes(dir, mTime, mTime)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s on %s", err, dir)
			os.Exit(1)
		}
		fmt.Printf("%s: %s\n", dir, mTime)
	}
}
