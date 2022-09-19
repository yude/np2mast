package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/mattn/go-mastodon"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		if os.Getenv("INSTANCE_URL") == "" || os.Getenv("CLIENT_ID") == "" || os.Getenv("CLIENT_SECRET") == "" || os.Getenv("ACCOUNT_EMAIL") == "" || os.Getenv("ACCOUNT_PASSWORD") == "" {
			log.Fatal("Failed to load credentials.")
		}
	}

	c := mastodon.NewClient(&mastodon.Config{
		Server:       os.Getenv("INSTANCE_URL"),
		ClientID:     os.Getenv("CLIENT_ID"),
		ClientSecret: os.Getenv("CLIENT_SECRET"),
	})
	err = c.Authenticate(context.Background(), os.Getenv("ACCOUNT_EMAIL"), os.Getenv("ACCOUNT_PASSWORD"))
	if err != nil {
		log.Fatal(err)
	}

	ticker := time.NewTicker(5 * time.Second)
	title, artist, album := "", "", ""

	for {
		select {
		case <-ticker.C:
			var jsonObj interface{}

			resp, err := http.Get("https://vercel-spotify-api.vercel.app/api/Spotify")
			if err != nil {
				fmt.Println("Error: ", err)
				return
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)

			if err := json.Unmarshal(body, &jsonObj); err != nil {
				fmt.Println(err)
				return
			}
			if jsonObj.(map[string]interface{})["isPlaying"] == true {
				if title == "" || title != jsonObj.(map[string]interface{})["title"] {
					title = jsonObj.(map[string]interface{})["title"].(string)
					artist = jsonObj.(map[string]interface{})["artist"].(string)
					album = jsonObj.(map[string]interface{})["album"].(string)

					message := fmt.Sprintf("🎵 #NowPlaying #np: %s / %s (%s)\n", title, artist, album)
					fmt.Println(message)
					toot := mastodon.Toot{
						Status: message,
					}
					c.PostStatus(context.Background(), &toot)
				}
			} else if jsonObj.(map[string]interface{})["isPlaying"] == false {
				title, artist, album = "", "", ""
			}

		}
	}
}
