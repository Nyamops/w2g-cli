package client

import (
	"encoding/json"
	"fmt"
	"net/url"
)

type JoinResponse struct {
	StreamKey      string `json:"streamkey"`
	User           string `json:"user"`
	AccessKey      string `json:"access_key"`
	ConfirmedUser  bool   `json:"confirmed_user"`
	ConfirmedEmail bool   `json:"confirmed_email"`
	PersistentName string `json:"persistent_name"`
	Owner          bool   `json:"owner"`
	Plus           bool   `json:"plus"`
	Pro            bool   `json:"pro"`
	Location       string `json:"location"`
}

type Playlist struct {
	Title      string `json:"title"`
	Key        string `json:"key"`
	Shuffle    int    `json:"shuffle"`
	Autoplay   bool   `json:"autoplay"`
	Queue      bool   `json:"queue"`
	IsPersonal bool   `json:"is_personal"`
}

type PlaylistsState struct {
	Lists            []Playlist `json:"lists"`
	SelectedPlaylist string     `json:"selectedPlaylist"`
}

type Room struct {
	PersistentName string     `json:"persistent_name"`
	StreamKey      string     `json:"streamkey"`
	Thumb          string     `json:"thumb"`
	Users          []RoomUser `json:"users"`
}

type RoomUser struct {
	Nickname string `json:"nname"`
	Role     string `json:"role"`
	Online   bool   `json:"online"`
}

func (r Room) Owner() string {
	for _, u := range r.Users {
		if u.Role == "owner" {
			return u.Nickname
		}
	}
	return ""
}

type PlaylistItem struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Thumb     string `json:"thumb"`
	Start     int    `json:"start"`
	Active    bool   `json:"active"`
	Voted     bool   `json:"voted"`
	Visible   bool   `json:"visible"`
	VoteCount int    `json:"vote_count"`
}

func (it PlaylistItem) NormalizedURL() string {
	if it.URL == "" {
		return ""
	}
	u, err := url.Parse(it.URL)
	if err != nil {
		return it.URL
	}
	if u.Scheme != "" {
		return u.String()
	}
	if u.Host == "" {
		if u, err = url.Parse("https://" + it.URL); err != nil {
			return it.URL
		}
		return u.String()
	}
	u.Scheme = "https"
	return u.String()
}

func parsePlaylistsFromState(body []byte) (*PlaylistsState, error) {
	var envelope struct {
		State [][]json.RawMessage `json:"state"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("decode sync_state: %w", err)
	}
	for _, pair := range envelope.State {
		if len(pair) != 2 {
			continue
		}
		var name string
		if err := json.Unmarshal(pair[0], &name); err != nil {
			continue
		}
		if name != "playlists" {
			continue
		}
		var section struct {
			Payload struct {
				Lists            []Playlist `json:"lists"`
				SelectedPlaylist string     `json:"selectedPlaylist"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(pair[1], &section); err != nil {
			return nil, fmt.Errorf("decode playlists section: %w", err)
		}
		return &PlaylistsState{
			Lists:            section.Payload.Lists,
			SelectedPlaylist: section.Payload.SelectedPlaylist,
		}, nil
	}

	return &PlaylistsState{}, nil
}
