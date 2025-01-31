package main

import (
    "bufio"
    "encoding/csv"
    "flag"
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "sort"
    "strconv"
    "strings"
    "time"
)

type DirInfo struct {
    Path    string
    Size    string // Size as string from du
    ModTime time.Time
    Bytes   int64  // Numeric size for sorting
}

func main() {
    startPath := flag.String("path", "", "Starting path for search")
    daysOld := flag.Int("days", 30, "Minimum age in days")
    flag.Parse()

    if *startPath == "" {
        fmt.Println("Usage: go run main.go -path /path/to/search [-days 30]")
        os.Exit(1)
    }

    fmt.Println("Finding old directories...")
    dirs := findOldDirs(*startPath, *daysOld)
    
    if len(dirs) == 0 {
        fmt.Println("No matching directories found.")
        return
    }

    fmt.Printf("\nCalculating sizes for %d directories...\n", len(dirs))
    dirs = calculateSizes(dirs)

    // Sort by size (largest first)
    sort.Slice(dirs, func(i, j int) bool {
        return dirs[i].Bytes > dirs[j].Bytes
    })

    // Write to CSV file
    timestamp := time.Now().Format("2006-01-02_150405")
    filename := fmt.Sprintf("cleanup_%s.csv", timestamp)
    
    file, err := os.Create(filename)
    if err != nil {
        fmt.Printf("Error creating file: %v\n", err)
        os.Exit(1)
    }
    defer file.Close()

    writer := csv.NewWriter(file)
    defer writer.Flush()

    writer.Write([]string{"Path", "Size", "Modified Date"})

    fmt.Printf("\nFound directories older than %d days:\n\n", *daysOld)
    for i, dir := range dirs {
        fmt.Printf("%d. %s\n", i+1, dir.Path)
        fmt.Printf("   Size: %s\n", dir.Size)
        fmt.Printf("   Modified: %s\n\n", dir.ModTime.Format("2006-01-02 15:04:05"))

        writer.Write([]string{
            dir.Path,
            dir.Size,
            dir.ModTime.Format("2006-01-02 15:04:05"),
        })
    }

    fmt.Printf("Results written to: %s\n\n", filename)

    if len(dirs) > 0 {
        fmt.Print("Would you like to delete any directories? (y/n): ")
        reader := bufio.NewReader(os.Stdin)
        response, _ := reader.ReadString('\n')
        response = strings.TrimSpace(strings.ToLower(response))

        if response == "y" {
            fmt.Print("Enter the numbers of directories to delete (e.g., 1,2,3 or 1-3), or 'all': ")
            numStr, _ := reader.ReadString('\n')
            numStr = strings.TrimSpace(strings.ToLower(numStr))

            var toDelete []int
            if numStr == "all" {
                for i := range dirs {
                    toDelete = append(toDelete, i)
                }
            } else {
                ranges := strings.Split(numStr, ",")
                for _, r := range ranges {
                    if strings.Contains(r, "-") {
                        parts := strings.Split(r, "-")
                        if len(parts) == 2 {
                            start, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
                            end, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
                            for i := start - 1; i < end; i++ {
                                if i >= 0 && i < len(dirs) {
                                    toDelete = append(toDelete, i)
                                }
                            }
                        }
                    } else {
                        num, err := strconv.Atoi(strings.TrimSpace(r))
                        if err == nil && num > 0 && num <= len(dirs) {
                            toDelete = append(toDelete, num-1)
                        }
                    }
                }
            }

            if len(toDelete) > 0 {
                fmt.Println("\nThe following directories will be deleted:")
                for _, i := range toDelete {
                    fmt.Printf("- %s (%s)\n", dirs[i].Path, dirs[i].Size)
                }
                fmt.Print("\nConfirm deletion? (y/n): ")
                confirm, _ := reader.ReadString('\n')
                confirm = strings.TrimSpace(strings.ToLower(confirm))

                if confirm == "y" {
                    for _, i := range toDelete {
                        fmt.Printf("Deleting %s...\n", dirs[i].Path)
                        err := os.RemoveAll(dirs[i].Path)
                        if err != nil {
                            fmt.Printf("Error deleting %s: %v\n", dirs[i].Path, err)
                        }
                    }
                    fmt.Println("Deletion complete!")
                }
            }
        }
    }
}

func findOldDirs(startPath string, days int) []DirInfo {
    var dirs []DirInfo
    cutoffDate := time.Now().AddDate(0, 0, -days)

    filepath.Walk(startPath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return nil
        }

        if info.IsDir() && (info.Name() == "node_modules" ||
            info.Name() == ".venv" ||
            info.Name() == ".env") {

            if info.ModTime().After(cutoffDate) {
                return filepath.SkipDir
            }

            dirs = append(dirs, DirInfo{
                Path:    path,
                ModTime: info.ModTime(),
            })
            return filepath.SkipDir
        }
        return nil
    })

    return dirs
}

func calculateSizes(dirs []DirInfo) []DirInfo {
    for i, dir := range dirs {
        // Show progress
        fmt.Printf("\rCalculating size %d/%d: %s", i+1, len(dirs), dir.Path)

        // Run du -sh for each directory
        cmd := exec.Command("du", "-sh", dir.Path)
        output, err := cmd.Output()
        if err != nil {
            dirs[i].Size = "error"
            continue
        }

        // Parse du output
        parts := strings.Fields(string(output))
        if len(parts) > 0 {
            dirs[i].Size = parts[0]
            
            // Convert size to bytes for sorting
            size, unit := parseSize(parts[0])
            dirs[i].Bytes = convertToBytes(size, unit)
        }
    }
    fmt.Println() // New line after progress
    return dirs
}

func parseSize(sizeStr string) (float64, string) {
    var size float64
    var unit string
    
    fmt.Sscanf(sizeStr, "%f%s", &size, &unit)
    return size, unit
}

func convertToBytes(size float64, unit string) int64 {
    multiplier := map[string]int64{
        "B":  1,
        "K":  1024,
        "M":  1024 * 1024,
        "G":  1024 * 1024 * 1024,
        "T":  1024 * 1024 * 1024 * 1024,
    }

    // Clean up unit string (remove 'B' if present)
    unit = strings.TrimSuffix(strings.ToUpper(unit), "B")
    
    if mult, ok := multiplier[unit]; ok {
        return int64(size * float64(mult))
    }
    return 0
}
