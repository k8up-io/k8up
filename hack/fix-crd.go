// +build fix-crd

package main
import (
    "bufio"
    "fmt"
    "log"
    "os"
)

func main() {

	fileName := os.Args[1]
	log.Println(fmt.Sprintf("Reading file %s", fileName))
    lines, err := readLines(fileName)
    if err != nil {
        log.Fatalf("readLines: %s", err)
    }
    count := 0
	var result []string
    for i, line := range lines {
    	if line == "                            - protocol" {
    		count++
    		log.Println(fmt.Sprintf("Removed 'protocol' in line %d", i))
		} else {
			result = append(result, line)
		}
	}

	log.Println(fmt.Sprintf("Writing new file to %s", fileName))
    if err := writeLines(result, fileName); err != nil {
        log.Fatalf("writeLines: %s", err)
    }
}

// readLines reads a whole file into memory
// and returns a slice of its lines.
func readLines(path string) ([]string, error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    var lines []string
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        lines = append(lines, scanner.Text())
    }
    return lines, scanner.Err()
}

// writeLines writes the lines to the given file.
func writeLines(lines []string, path string) error {
    file, err := os.Create(path)
    if err != nil {
        return err
    }
    defer file.Close()

    w := bufio.NewWriter(file)
    for _, line := range lines {
        fmt.Fprintln(w, line)
    }
    return w.Flush()
}
