package importer

import "strings"

const (
	chunkWords  = 500
	overlapWords = 50
)

func chunk(text string) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	var chunks []string
	step := chunkWords - overlapWords
	for i := 0; i < len(words); i += step {
		end := i + chunkWords
		if end > len(words) {
			end = len(words)
		}
		chunks = append(chunks, strings.Join(words[i:end], " "))
		if end == len(words) {
			break
		}
	}
	return chunks
}
