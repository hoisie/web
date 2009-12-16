package ini

import (
    "fmt"
    "io/ioutil"
    "os"
    "strings"
)

func clean(line string) string {
    var i int
    for i = 0; i < len(line); i++ {
        if line[i] == ';' {
            break
        }
    }
    line = line[0:i]

    return strings.TrimSpace(line)
}

type LineError string

func (err LineError) String() string { return fmt.Sprintf("Error parsing line %q", err) }

func parseIni(contents string) (map[string]map[string]string, os.Error) {
    lines := strings.Split(contents, "\n", 0)

    parsed := make(map[string]map[string]string)
    var cur *map[string]string

    for _, line := range (lines) {
        cleaned := clean(line)
        if cleaned == "" {
            continue
        } else if cleaned[0] == '[' && cleaned[len(cleaned)-1] == ']' {
            name := cleaned[1 : len(cleaned)-1]
            ns := make(map[string]string)
            parsed[name] = ns
            cur = &ns
        } else if strings.Index(cleaned, "=") != -1 {
            a := strings.Split(line, "=", 0)
            key, value := strings.TrimSpace(a[0]), strings.TrimSpace(a[1])
            (*cur)[key] = value
        } else {
            return nil, LineError(line)
        }
    }

    return parsed, nil
}

func ParseString(contents string) (map[string]map[string]string, os.Error) {
    return parseIni(contents)
}

func ParseFile(filename string) (map[string]map[string]string, os.Error) {
    contents, err := ioutil.ReadFile(filename)
    if err != nil {
        return nil, err
    }

    return parseIni(string(contents))
}
