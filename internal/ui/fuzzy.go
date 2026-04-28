package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"ide/internal/tmux"
)

func (m *Model) openFuzzySearch() tea.Cmd {
	m.showFuzzySearch = true
	m.showThemePicker = false
	m.showShortcuts = false
	m.fuzzySearchQuery.SetValue("")
	m.fuzzySearchCursor = 0
	m.fuzzySearchQuery.Focus()
	m.fuzzySearchResults = m.computeFuzzySearchResults()
	return textinput.Blink
}

func fuzzyMatch(query, target string) bool {
	qi := 0
	for i := 0; i < len(target) && qi < len(query); i++ {
		if target[i] == query[qi] {
			qi++
		}
	}
	return qi == len(query)
}

func (m *Model) rebuildFuzzyIndex() {
	cache := make([]fuzzyEnvCacheEntry, 0, len(m.environments))
	for envIdx, env := range m.environments {
		session := tmux.SessionName(env.Name)
		_, running := m.sessions[session]
		sessionStatus := m.getSessionAgentStatus(env)

		windows := m.windowNamesForEnv(env)
		winEntries := make([]fuzzyWinCacheEntry, 0, len(windows))
		for winIdx, wName := range windows {
			var status AgentStatus
			var tags []string
			tmpl, hasTmpl := findWindowTemplate(env, wName)
			if hasTmpl {
				tags = tmpl.Tags
			}
			key := windowKey(session, wName)
			info, hasInfo := m.windowProcessInfo[key]
			cachedCmd := ""
			if hasInfo {
				cachedCmd = info.Command
			}
			if m.isAIWindow(env, wName, cachedCmd) && hasInfo {
				status = info.Status
			}

			tagStr := ""
			for _, t := range tags {
				tagStr += " [" + t + "]"
			}
			searchStr := strings.ToLower(env.Name + " " + wName + tagStr)
			if running {
				searchStr += " running up"
			}
			switch status {
			case AgentStatusCooking:
				searchStr += " cooking"
			case AgentStatusAwaitingInput:
				searchStr += " awaiting input"
			}

			winEntries = append(winEntries, fuzzyWinCacheEntry{
				item: fuzzySearchItem{
					EnvIndex:    envIdx,
					WindowIndex: winIdx,
					EnvName:     env.Name,
					WindowName:  wName,
					Status:      status,
					Tags:        tags,
					Running:     running,
				},
				haystack: searchStr,
			})
		}

		cache = append(cache, fuzzyEnvCacheEntry{
			header: fuzzySearchItem{
				EnvIndex:    envIdx,
				WindowIndex: -1,
				EnvName:     env.Name,
				Status:      sessionStatus,
				Running:     running,
				IsHeader:    true,
			},
			windows: winEntries,
		})
	}
	m.fuzzySearchCache = cache
}

func (m Model) computeFuzzySearchResults() []fuzzySearchItem {
	query := strings.ToLower(strings.TrimSpace(m.fuzzySearchQuery.Value()))
	var results []fuzzySearchItem

	for _, entry := range m.fuzzySearchCache {
		var matchedWindows []fuzzySearchItem
		for _, w := range entry.windows {
			if query == "" || fuzzyMatch(query, w.haystack) {
				matchedWindows = append(matchedWindows, w.item)
			}
		}
		if len(matchedWindows) > 0 {
			results = append(results, entry.header)
			results = append(results, matchedWindows...)
		}
	}
	return results
}

func (m *Model) normalizeFuzzySearchCursor() {
	if len(m.fuzzySearchResults) == 0 {
		m.fuzzySearchCursor = 0
		return
	}
	if m.fuzzySearchCursor < 0 {
		m.fuzzySearchCursor = 0
	}
	if m.fuzzySearchCursor >= len(m.fuzzySearchResults) {
		m.fuzzySearchCursor = len(m.fuzzySearchResults) - 1
	}
	// Skip header rows
	if m.fuzzySearchResults[m.fuzzySearchCursor].IsHeader {
		m.fuzzySearchCursor++
		if m.fuzzySearchCursor >= len(m.fuzzySearchResults) {
			// Try going backwards instead
			m.fuzzySearchCursor -= 2
			for m.fuzzySearchCursor >= 0 && m.fuzzySearchResults[m.fuzzySearchCursor].IsHeader {
				m.fuzzySearchCursor--
			}
			if m.fuzzySearchCursor < 0 {
				m.fuzzySearchCursor = 0
			}
		}
	}
}

func (m *Model) moveFuzzySearchCursor(direction int) {
	n := len(m.fuzzySearchResults)
	if n == 0 {
		return
	}
	original := m.fuzzySearchCursor
	m.fuzzySearchCursor += direction
	// Clamp
	if m.fuzzySearchCursor < 0 {
		m.fuzzySearchCursor = 0
	}
	if m.fuzzySearchCursor >= n {
		m.fuzzySearchCursor = n - 1
	}
	// Skip headers in the direction of movement
	for m.fuzzySearchCursor >= 0 && m.fuzzySearchCursor < n && m.fuzzySearchResults[m.fuzzySearchCursor].IsHeader {
		m.fuzzySearchCursor += direction
	}
	// If we walked off either end while skipping headers, search the
	// opposite direction from the clamped boundary so the cursor lands on
	// a real (non-header) row instead of being parked on a header.
	if m.fuzzySearchCursor < 0 || m.fuzzySearchCursor >= n {
		if direction < 0 {
			m.fuzzySearchCursor = 0
		} else {
			m.fuzzySearchCursor = n - 1
		}
		opposite := -direction
		for m.fuzzySearchCursor >= 0 && m.fuzzySearchCursor < n && m.fuzzySearchResults[m.fuzzySearchCursor].IsHeader {
			m.fuzzySearchCursor += opposite
		}
		// All rows are headers — give up and restore caller's position.
		if m.fuzzySearchCursor < 0 || m.fuzzySearchCursor >= n {
			m.fuzzySearchCursor = original
		}
	}
}

func (m Model) updateFuzzySearchMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.showFuzzySearch = false
		m.fuzzySearchQuery.Blur()
		m.status = "Search closed."
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "up", "ctrl+k":
		m.moveFuzzySearchCursor(-1)
		return m, nil
	case "down", "ctrl+j":
		m.moveFuzzySearchCursor(1)
		return m, nil
	case "enter":
		if m.fuzzySearchCursor < len(m.fuzzySearchResults) {
			item := m.fuzzySearchResults[m.fuzzySearchCursor]
			if item.IsHeader {
				return m, nil
			}
			m.selectedEnv = item.EnvIndex
			m.selectedWindow = item.WindowIndex
			m.showFuzzySearch = false
			m.fuzzySearchQuery.Blur()
			m.focusPane = focusPaneWindows
			return m.startAttachSelected()
		}
		return m, nil
	default:
		var cmd tea.Cmd
		m.fuzzySearchQuery, cmd = m.fuzzySearchQuery.Update(msg)
		m.fuzzySearchResults = m.computeFuzzySearchResults()
		m.fuzzySearchCursor = 0
		m.normalizeFuzzySearchCursor()
		return m, cmd
	}
}

var tagRe = regexp.MustCompile(`\[(\w+)\]`)

// extractTags extracts tags in format [tag1][tag2] from the entry
// Modifies entry to remove the tags and returns the list of tags
func extractTags(entry *string) []string {
	var tags []string
	matches := tagRe.FindAllStringSubmatch(*entry, -1)
	for _, match := range matches {
		if len(match) > 1 {
			tags = append(tags, match[1])
		}
	}
	*entry = tagRe.ReplaceAllString(*entry, "")
	*entry = strings.TrimSpace(*entry)
	return tags
}
