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

	go func() {
		http.HandleFunc("/login", spotify_login)
		auth_code := make(chan string)
		go pass_callback(auth_code)
		handleCallback := spotify_callback(auth_code)
		http.HandleFunc("/callback", handleCallback)

		err := http.ListenAndServe("localhost:3000", nil)

		if err != nil {
			log.Fatal(err)
		}
	}()

	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Failed to load credentials.")
	} else if os.Getenv("MASTODON_INSTANCE_URL") == "" || os.Getenv("MASTODON_CLIENT_ID") == "" || os.Getenv("MASTODON_CLIENT_SECRET") == "" || os.Getenv("MASTODON_ACCOUNT_EMAIL") == "" || os.Getenv("MASTODON_ACCOUNT_PASSWORD") == "" || os.Getenv("SPOTIFY_CLIENT_ID") == "" || os.Getenv("SPOTIFY_CLIENT_SECRET") == "" {
		log.Fatal("Failed to load credentials.")
	} else if os.Getenv("SPOTIFY_REFRESH_TOKEN") == "" {
		fmt.Println("`SPOTIFY_REFRESH_TOKEN` is not set. Please click the URL below.")
		values := url.Values{}
		values.Add("client_id", os.Getenv("SPOTIFY_CLIENT_ID"))
		values.Add("response_type", "code")
		values.Add("redirect_uri", "http://localhost:3000/callback")
		fmt.Println("https://accounts.spotify.com/authorize?" + values.Encode())

		if err != nil {
			log.Fatal(err)
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
			is_playing, title, artist, album, url, progress := get_spotify_np()
			if is_playing {
				if last_title == "" || title != last_title {
					if progress > 5000 {
						message := fmt.Sprintf("🎵 #NowPlaying #np: %s / %s (%s)\n%s", title, artist, album, url)
						fmt.Println(message)
						toot := mastodon.Toot{
							Status:     message,
							Visibility: "unlisted",
						}
						mastodon_client.PostStatus(context.Background(), &toot)

						last_title = title
					}
				}
			} else {
				title, artist, album = "", "", ""
			}

		}
	}
}

func spotify_login(w http.ResponseWriter, req *http.Request) {
	values := url.Values{}
	values.Add("client_id", os.Getenv("SPOTIFY_CLIENT_ID"))
	values.Add("response_type", "code")
	values.Add("redirect_uri", "http://localhost:3000/callback")

	http.Redirect(w, req, "https://accounts.spotify.com/authorize?"+values.Encode(), http.StatusFound)
}

func spotify_callback(auth_code chan string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		query := req.URL.Query().Get("code")
		auth_code <- query

		w.WriteHeader(200)
		w.Header().Set("Content-Type", "text/html; charset=utf8")

		w.Write([]byte("処理が完了しました。この画面を閉じることができます。\nnp2mast を再起動してください。"))

	}
}

func pass_callback(auth_code chan string) {
	for item := range auth_code {
		save_refresh_token(item)
	}
}

func save_refresh_token(auth_code string) {
	values := make(url.Values)
	values.Set("grant_type", "authorization_code")
	values.Set("code", auth_code)
	values.Set("redirect_uri", "http://localhost:3000/callback")
	req, err := http.NewRequest(http.MethodPost, "https://accounts.spotify.com/api/token", strings.NewReader(values.Encode()))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", b64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", os.Getenv("SPOTIFY_CLIENT_ID"), os.Getenv("SPOTIFY_CLIENT_SECRET"))))))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

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

	refresh_token := jsonObj.(map[string]interface{})["refresh_token"].(string)
	refresh_token_env, err := godotenv.Unmarshal(fmt.Sprintf("MASTODON_INSTANCE_URL=%s\nMASTODON_CLIENT_ID=%s\nMASTODON_CLIENT_SECRET=%s\nMASTODON_ACCOUNT_EMAIL=%s\nMASTODON_ACCOUNT_PASSWORD=%s\nSPOTIFY_CLIENT_ID=%s\nSPOTIFY_CLIENT_SECRET=%s\nSPOTIFY_REFRESH_TOKEN=%s\n", os.Getenv("MASTODON_INSTANCE_URL"), os.Getenv("MASTODON_CLIENT_ID"), os.Getenv("MASTODON_CLIENT_SECRET"), os.Getenv("MASTODON_ACCOUNT_EMAIL"), os.Getenv("MASTODON_ACCOUNT_PASSWORD"), os.Getenv("SPOTIFY_CLIENT_ID"), os.Getenv("SPOTIFY_CLIENT_SECRET"), refresh_token))

	if err != nil {
		log.Fatal(err)
	}
	err = godotenv.Write(refresh_token_env, "./.env")
	if err != nil {
		log.Fatal(err)
	}

	os.Exit(0)
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
		log.Fatal(err)
	}
	return jsonObj.(map[string]interface{})["access_token"].(string)
}

func get_spotify_np() (is_playing bool, title string, artist string, album string, url string, progress float64) {
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

	if is_playing {
		title = jsonObj.(map[string]interface{})["item"].(map[string]interface{})["name"].(string)

		artists := jsonObj.(map[string]interface{})["item"].(map[string]interface{})["artists"]
		for i := 0; i < len(artists.([]interface{})); i++ {
			artist += artists.([]interface{})[i].(map[string]interface{})["name"].(string)
			if i < len(artists.([]interface{}))-1 {
				artist += ", "
			}
		}

		album = jsonObj.(map[string]interface{})["item"].(map[string]interface{})["album"].(map[string]interface{})["name"].(string)

		url = jsonObj.(map[string]interface{})["item"].(map[string]interface{})["external_urls"].(map[string]interface{})["spotify"].(string)

		progress = jsonObj.(map[string]interface{})["progress_ms"].(float64)
	} else {
		is_playing = false

		title, artist, album = "", "", ""
	}

	return is_playing, title, artist, album, url, progress
}
