package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"

	"github.com/boltdb/bolt"
	"gopkg.in/yaml.v2"
)

type BotConfig struct {
	Token    string `yaml:"token"`
	Port     int    `yaml:"port"`
	HookBase string `yaml:"hook_base"`
}

type IgnatBot struct {
	config BotConfig
	db     *bolt.DB
}

func (bot *IgnatBot) Init(filename string) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(data, &bot.config)
	if err != nil {
		panic(err)
	}

	db, err := bolt.Open("ignat.db", 0600, nil)
	if err != nil {
		panic(err)
	}
	bot.db = db

	return nil
}

func (bot *IgnatBot) ProcessUpdate(body []byte) {
	var update Update
	err := json.Unmarshal(body, &update)
	if err != nil {
		fmt.Printf("Error processing JSON: %v\n", err)
		return
	}
	if update.Message != nil {
		fmt.Printf("Got message from %v %v: %v\n", update.Message.From.First_name,
			update.Message.From.LastName, update.Message.Text)

		err := bot.db.Update(func(tx *bolt.Tx) error {
			b, err := tx.CreateBucketIfNotExists([]byte("history"))
			if err != nil {
				return err
			}
			id, _ := b.NextSequence()
			buf, _ := json.Marshal(update.Message)
			return b.Put(itob(int(id)), buf)
		})
		if err != nil {
			fmt.Printf("Cannot save message to history DB: %v\n", err)
		}

		text := update.Message.Text
		if matched, _ := regexp.MatchString(".*[Дд]а$", text); matched {
			bot.ApiPost("sendMessage", map[string]interface{}{
				"chat_id": update.Message.Chat.Id,
				"text":    "Пизда!"})
		}
	}
}

func (bot *IgnatBot) UpdateHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadAll(r.Body)
	fmt.Printf("Got payload %v\n", string(body))
	go bot.ProcessUpdate(body)
}

func (bot *IgnatBot) ApiPost(method string, body interface{}) {
	url := fmt.Sprintf("https://api.telegram.org/bot%v/%v", bot.config.Token, method)
	fmt.Printf("POST %v\n", url)
	json_body, _ := json.Marshal(body)
	fmt.Printf("Body %v\n", string(json_body))
	resp, err := http.Post(url, "Application/json", bytes.NewBuffer(json_body))
	if err != nil {
		fmt.Println(err)
	} else {
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf("Response was %v '%v'\n", resp.StatusCode, string(body))
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "dump" {
		fmt.Println("Dumping history")
		db, _ := bolt.Open("ignat.db", 0400, nil)
		db.View(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("history"))
			c := b.Cursor()

			for k, v := c.First(); k != nil; k, v = c.Next() {
				var msg Message
				err := json.Unmarshal(v, &msg)
				if err != nil {
					fmt.Printf("Got error: %v\n", err)
				}
				fmt.Println(msg)
			}
			return nil
		})
	}
	var ignat IgnatBot
	ignat.Init("ignat.yaml")
	fmt.Println(ignat)

	http.HandleFunc(fmt.Sprintf("/ignat/%v", ignat.config.Token), ignat.UpdateHandler)
	ignat.ApiPost("setWebhook", map[string]interface{}{
		"url":             fmt.Sprintf("%v/ignat/%v", ignat.config.HookBase, ignat.config.Token),
		"allowed_updates": []string{"message"},
	})
	err := http.ListenAndServe(fmt.Sprintf("localhost:%v", ignat.config.Port), nil)
	if err != nil {
		panic(err)
	}

}
