package cli

import (
	"fmt"
	"strings"

	"w2g-cli/internal/client"
)

func resolvePlaylist(state *client.PlaylistsState, query string) (*client.Playlist, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, fmt.Errorf("specify a playlist by name or key")
	}

	for i := range state.Lists {
		if state.Lists[i].Key == q {
			return &state.Lists[i], nil
		}
	}

	var exact []*client.Playlist
	for i := range state.Lists {
		if strings.EqualFold(state.Lists[i].Title, q) {
			exact = append(exact, &state.Lists[i])
		}
	}
	if len(exact) == 1 {
		return exact[0], nil
	}
	if len(exact) > 1 {
		return nil, ambiguous(q, exact)
	}

	var subs []*client.Playlist
	lq := strings.ToLower(q)
	for i := range state.Lists {
		if strings.Contains(strings.ToLower(state.Lists[i].Title), lq) {
			subs = append(subs, &state.Lists[i])
		}
	}
	switch len(subs) {
	case 1:
		return subs[0], nil
	case 0:
		return nil, fmt.Errorf("no playlist matching %q (run `w2g playlists` to see them)", query)
	default:
		return nil, ambiguous(q, subs)
	}
}

func ambiguous(q string, cands []*client.Playlist) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%q is ambiguous; matches:", q)
	for _, p := range cands {
		fmt.Fprintf(&b, "\n - %s (%s)", p.Title, p.Key)
	}
	b.WriteString("\nUse the exact key instead.")
	return fmt.Errorf("%s", b.String())
}

func resolveRoom(rooms []client.Room, query string) (*client.Room, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, fmt.Errorf("specify a room by name or id")
	}

	for i := range rooms {
		if rooms[i].StreamKey == q {
			return &rooms[i], nil
		}
	}

	var exact []*client.Room
	for i := range rooms {
		if strings.EqualFold(rooms[i].PersistentName, q) {
			exact = append(exact, &rooms[i])
		}
	}
	if len(exact) == 1 {
		return exact[0], nil
	}
	if len(exact) > 1 {
		return nil, ambiguousRoom(q, exact)
	}

	var subs []*client.Room
	lq := strings.ToLower(q)
	for i := range rooms {
		if strings.Contains(strings.ToLower(rooms[i].PersistentName), lq) {
			subs = append(subs, &rooms[i])
		}
	}
	switch len(subs) {
	case 1:
		return subs[0], nil
	case 0:
		return nil, fmt.Errorf("no room matching %q (run `w2g rooms` to see them)", query)
	default:
		return nil, ambiguousRoom(q, subs)
	}
}

func ambiguousRoom(q string, cands []*client.Room) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%q is ambiguous; matches:", q)
	for _, r := range cands {
		fmt.Fprintf(&b, "\n - %s (%s)", r.PersistentName, r.StreamKey)
	}
	b.WriteString("\nUse the exact room id instead.")
	return fmt.Errorf("%s", b.String())
}
