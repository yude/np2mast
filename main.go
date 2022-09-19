package main

import (
	"context"
	b64 "encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/mattn/go-mastodon"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		if os.Getenv("MASTODON_INSTANCE_URL") == "" || os.Getenv("MASTODON_CLIENT_ID") == "" || os.Getenv("MASTODON_CLIENT_SECRET") == "" || os.Getenv("MASTODON_ACCOUNT_EMAIL") == "" || os.Getenv("MASTODON_ACCOUNT_PASSWORD") == "" || os.Getenv("SPOTIFY_CLIENT_ID") == "" || os.Getenv("SPOTIFY_CLIENT_SECRET") == "" || os.Getenv("SPOTIFY_REFRESH_TOKEN") == "" {
			log.Fatal("Failed to load credentials.")
		}
	}

	mastodon_client := mastodon.NewClient(&mastodon.Config{
		Server:       os.Getenv("MASTODON_INSTANCE_URL"),
		ClientID:     os.Getenv("MASTODON_CLIENT_ID"),
		ClientSecret: os.Getenv("MASTODON_CLIENT_SECRET"),
	})
	err = mastodon_client.Authenticate(context.Background(), os.Getenv("MASTODON_ACCOUNT_EMAIL"), os.Getenv("MASTODON_ACCOUNT_PASSWORD"))
	if err != nil {
		log.Fatal(err)
	}

	last_title := ""

	ticker := time.NewTicker(5 * time.Second)

	for {
		select {
		case <-ticker.C:
			is_playing, title, artist, album := get_spotify_np()
			if is_playing {
				if last_title == "" || title != last_title {
					message := fmt.Sprintf("🎵 #NowPlaying #np: %s / %s (%s)\n", title, artist, album)
					fmt.Println(message)
					toot := mastodon.Toot{
						Status:     message,
						Visibility: "unlisted",
					}
					mastodon_client.PostStatus(context.Background(), &toot)

					last_title = title
				}
			} else {
				title, artist, album = "", "", ""
			}

		}
	}
}

func get_spotify_access_token() string {
	values := make(url.Values)
	values.Set("grant_type", "refresh_token")
	values.Set("refresh_token", os.Getenv("SPOTIFY_REFRESH_TOKEN"))

	req, err := http.NewRequest(http.MethodPost, "https://accounts.spotify.com/api/token", strings.NewReader(values.Encode()))
	if err != nil {
		log.Fatal(err)
	}

	spotify_auth_string := b64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", os.Getenv("SPOTIFY_CLIENT_ID"), os.Getenv("SPOTIFY_CLIENT_SECRET"))))

	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", spotify_auth_string))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var jsonObj interface{}
	if err := json.Unmarshal(body, &jsonObj); err != nil {
		fmt.Println(err)
		return "ERR"
	}

	return jsonObj.(map[string]interface{})["access_token"].(string)
}

func get_spotify_np() (is_playing bool, title string, artist string, album string) {
	req, err := http.NewRequest(http.MethodGet, "https://api.spotify.com/v1/me/player/currently-playing", nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", get_spotify_access_token()))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var jsonObj interface{}
	if err := json.Unmarshal(body, &jsonObj); err != nil {
		log.Fatal(err)
	}

	is_playing = jsonObj.(map[string]interface{})["is_playing"].(bool)

	if is_playing == true {
		title = jsonObj.(map[string]interface{})["item"].(map[string]interface{})["name"].(string)

		artists := jsonObj.(map[string]interface{})["item"].(map[string]interface{})["artists"]
		for i := 0; i < len(artists.([]interface{})); i++ {
			artist += artists.([]interface{})[i].(map[string]interface{})["name"].(string)
			if i < len(artists.([]interface{}))-1 {
				artist += ", "
			}
		}

		album = jsonObj.(map[string]interface{})["item"].(map[string]interface{})["album"].(map[string]interface{})["name"].(string)
	} else {
		is_playing = false

		title, artist, album = "", "", ""
	}

	return is_playing, title, artist, album
}
