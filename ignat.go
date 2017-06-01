package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/boltdb/bolt"
	"gopkg.in/yaml.v2"
)

type Chain map[string][]string

type BotConfig struct {
	Token    string `yaml:"token"`
	Port     int    `yaml:"port"`
	HookBase string `yaml:"hook_base"`
}

type IgnatBot struct {
	config BotConfig
	db     *bolt.DB
	chain  Chain
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
	bot.chain = bot.MakeChain()

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
		last_words := regexp.MustCompile(`(\p{Cyrillic}+\s+\p{Cyrillic}+)[-.!:() ]*$`)
		if m := last_words.FindStringSubmatch(text); m != nil {
			url := fmt.Sprintf("https://miga.me.uk/mark?len=20&feed=%v", url.PathEscape(m[1]))
			resp, err := http.Get(url)
			if err != nil || resp.StatusCode != 200 {
				return
			}
			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)
			if rand.Intn(100) < 30 {
				bot.ApiPost("sendMessage", map[string]interface{}{
					"chat_id": update.Message.Chat.Id,
					"text":    string(body)})
			}
		}
	}
}

func (bot *IgnatBot) UpdateHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadAll(r.Body)
	fmt.Printf("Got payload %v\n", string(body))
	go bot.ProcessUpdate(body)
}

func (bot *IgnatBot) DumpHandler(w http.ResponseWriter, r *http.Request) {
	stat := bot.DumpHistory()
	s := map[string]map[string]int{
		"result": stat,
	}
	body, _ := json.Marshal(s)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Write(body)
}

func (bot *IgnatBot) DumpChainHandler(w http.ResponseWriter, r *http.Request) {
	s := map[string]interface{}{
		"result": bot.chain,
	}
	body, _ := json.Marshal(s)
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Write(body)
}

func (bot *IgnatBot) GenerateHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(bot.MakeSentence()))
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

func (bot *IgnatBot) MakeChain() Chain {
	chain := make(Chain)
	splitter := regexp.MustCompile("[ .,!?;]+")
	printable := regexp.MustCompile("\\p{Cyrillic}+")

	bot.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("history"))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var msg Message
			err := json.Unmarshal(v, &msg)
			if err != nil {
				fmt.Printf("Got error: %v\n", err)
				return err
			}

			//for _, sentence := range strings.Split(msg.Text, "\n") {
			var words []string
			for _, w := range splitter.Split(msg.Text, -1) {
				if printable.Match([]byte(w)) {
					words = append(words, w)
				}
			}
			for i := 0; i < len(words)-3; i++ {
				key := fmt.Sprintf("%v %v", words[i], words[i+1])
				chain[key] = append(chain[key], words[3])
			}
			//}
		}
		return nil
	})

	return chain
}

func (bot *IgnatBot) MakeSentence() string {

	i := 0
	couples := make([]string, len(bot.chain))
	fmt.Printf("%v %v\n", len(couples), len(bot.chain))
	for c := range bot.chain {
		couples[i] = c
		i++
	}
	fmt.Printf("%v %v\n", len(couples), len(bot.chain))

	couple := couples[rand.Intn(len(couples))]
	sentence := couple
	fmt.Printf("Start is '%v'\n", sentence)
	for {
		words := strings.Split(couple, " ")
		next, ok := bot.chain[couple]
		if !ok {
			break
		}
		fmt.Printf("Couple is %v -> %v\n", couple, next)

		next_word := next[rand.Intn(len(next))]
		sentence = fmt.Sprintf("%v %v", sentence, next_word)
		couple = fmt.Sprintf("%v %v", words[1], next_word)
		fmt.Printf("Next couple is '%v'\n", couple)
	}
	return fmt.Sprint(sentence, "\n")
}

func (bot *IgnatBot) DumpHistory() map[string]int {
	word_stat := make(map[string]int)
	splitter := regexp.MustCompile("[ .,!?;\n]+")
	printable := regexp.MustCompile("\\p{Cyrillic}+")

	bot.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("history"))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var msg Message
			err := json.Unmarshal(v, &msg)
			if err != nil {
				fmt.Printf("Got error: %v\n", err)
				return err
			}
			for _, w := range splitter.Split(msg.Text, -1) {
				if printable.Match([]byte(w)) {
					word_stat[strings.ToLower(w)] += 1
				}
			}
		}
		return nil
	})

	return word_stat
}

func main() {

	var ignat IgnatBot
	ignat.Init("ignat.yaml")

	http.HandleFunc(fmt.Sprintf("/ignat/%v", ignat.config.Token), ignat.UpdateHandler)
	http.HandleFunc("/ignat/dump", ignat.DumpChainHandler)
	http.HandleFunc("/ignat/generate", ignat.GenerateHandler)
	ignat.ApiPost("setWebhook", map[string]interface{}{
		"url":             fmt.Sprintf("%v/ignat/%v", ignat.config.HookBase, ignat.config.Token),
		"allowed_updates": []string{"message"},
	})
	err := http.ListenAndServe(fmt.Sprintf("localhost:%v", ignat.config.Port), nil)
	if err != nil {
		panic(err)
	}

}
