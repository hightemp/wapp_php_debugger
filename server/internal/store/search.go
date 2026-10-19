package store

import (
	"strings"
	"unicode"
)

// ftsWords splits user input into words the way the unicode61 tokenizer does (letters and digits).
func ftsWords(input string) []string {
	return strings.FieldsFunc(input, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// ftsQuery turns user input into a safe FTS5 query: every word is quoted (so FTS syntax
// characters are plain text) and matched as a prefix; the words are ANDed.
func ftsQuery(input string) string {
	words := ftsWords(input)
	quoted := make([]string, 0, len(words))
	for _, word := range words {
		quoted = append(quoted, `"`+strings.ReplaceAll(word, `"`, `""`)+`"*`)
	}
	return strings.Join(quoted, " ")
}

// requestSearch matches a request when every word is found in one of its messages (text or context)
// or the whole input is a substring of its URL.
func requestSearch(query string) (string, []any) {
	words := ftsWords(query)
	if len(words) == 0 {
		return "", nil
	}
	clauses := make([]string, 0, len(words))
	args := []any{}
	for _, word := range words {
		clauses = append(clauses, `EXISTS (SELECT 1 FROM messages_fts JOIN messages fm ON fm.id = messages_fts.rowid
			WHERE messages_fts MATCH ? AND fm.request_id = r.id)`)
		args = append(args, ftsQuery(word))
	}
	like := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(query) + "%"
	return "((" + strings.Join(clauses, " AND ") + `) OR r.url LIKE ? ESCAPE '\')`, append(args, like)
}
