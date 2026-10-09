package accessinventory

import "strings"

// SQLReferences extracts lexical FROM/JOIN/INTO/UPDATE sources, not a SQL AST.
// It deliberately does not infer VBA-generated SQL or evaluate Access expressions.
func SQLReferences(sql string) []string {
	tokens := sqlTokens(sql)
	var result []string
	seen := make(map[string]bool)
	add := func(name string) {
		key := strings.ToLower(name)
		if name != "" && name != "(" && name != ")" && !seen[key] {
			result = append(result, name)
			seen[key] = true
		}
	}
	addSource := func(index int) {
		for index < len(tokens) && tokens[index].text == "(" && !tokens[index].quoted {
			index++
		}
		if index < len(tokens) {
			token := tokens[index]
			if !token.quoted {
				switch strings.ToUpper(token.text) {
				case "SELECT", "PARAMETERS", "TRANSFORM":
					return
				}
			}
			add(token.text)
		}
	}
	fromLists := make(map[int]bool)
	depth := 0
	for index, token := range tokens {
		if token.text == "(" && !token.quoted {
			depth++
		} else if token.text == ")" && !token.quoted {
			delete(fromLists, depth)
			depth--
		}
		if token.quoted {
			continue
		}
		switch strings.ToUpper(token.text) {
		case "FROM":
			fromLists[depth] = true
			addSource(index + 1)
		case "JOIN", "INTO", "UPDATE":
			addSource(index + 1)
		case "WHERE", "GROUP", "ORDER", "HAVING", "UNION", "ON", ";":
			delete(fromLists, depth)
		case ",":
			if fromLists[depth] {
				addSource(index + 1)
			}
		}
	}
	return result
}

type sqlToken struct {
	text   string
	quoted bool
}

func sqlTokens(sql string) []sqlToken {
	var result []sqlToken
	for index := 0; index < len(sql); {
		char := sql[index]
		if char <= ' ' {
			index++
			continue
		}
		if char == '-' && index+1 < len(sql) && sql[index+1] == '-' {
			for index < len(sql) && sql[index] != '\n' {
				index++
			}
			continue
		}
		if char == '/' && index+1 < len(sql) && sql[index+1] == '*' {
			index += 2
			for index+1 < len(sql) && sql[index:index+2] != "*/" {
				index++
			}
			index = min(index+2, len(sql))
			continue
		}
		if char == '\'' || char == '"' || char == '[' {
			close := char
			if char == '[' {
				close = ']'
			}
			index++
			var word strings.Builder
			for index < len(sql) {
				current := sql[index]
				index++
				if current == close {
					if index < len(sql) && sql[index] == close {
						word.WriteByte(close)
						index++
						continue
					}
					break
				}
				word.WriteByte(current)
			}
			if char == '[' {
				result = append(result, sqlToken{text: word.String(), quoted: true})
			} else {
				// Access uses both quote styles for string literals.
				result = append(result, sqlToken{quoted: true})
			}
			continue
		}
		if strings.ContainsRune("(),;", rune(char)) {
			result = append(result, sqlToken{text: string(char)})
			index++
			continue
		}
		start := index
		for index < len(sql) && sql[index] > ' ' && !strings.ContainsRune("(),;'\"[]", rune(sql[index])) {
			index++
		}
		if index == start {
			index++
		} else {
			result = append(result, sqlToken{text: sql[start:index]})
		}
	}
	return result
}
