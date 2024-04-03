package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/spf13/viper"
)

var (
	q       *Queue
	bot     *tgbotapi.BotAPI
	watcher *fsnotify.Watcher
	err     error
	c       *Config

	t *time.Ticker
)

func main() {
	viper.SetConfigFile("config.yaml")
	c = &Config{}
	viper.ReadInConfig()

	initWatcher()

	viper.OnConfigChange(func(in fsnotify.Event) {
		for _, v := range watcher.WatchList() {
			watcher.Remove(v)
		}
		t.Stop()
		initWatcher()
	})
	viper.WatchConfig()
	defer watcher.Close()
	// Start listening for events.
	go func() {
		for {
			event, ok := <-watcher.Events
			if !ok {
				return
			}
			// if event.Has(fsnotify.Create | fsnotify.Chmod | fsnotify.Write) {
			// 	if err != nil {
			// 		log.Panic(err)
			// 	}
			q.Add(event.Name + " " + event.Op.String())
			// }
		}
	}()

	go startTicker()
	log.Println("now watching ")
	// Block main goroutine forever.
	<-make(chan struct{})
}
func initWatcher() {
	viper.Unmarshal(c)
	q = &Queue{
		max: c.Max,
	}
	if bot, err = tgbotapi.NewBotAPI(c.Token); err != nil {
		panic(err)
	}
	if watcher, err = fsnotify.NewWatcher(); err != nil {
		log.Fatal(err)
	}
	for _, f := range c.Path {
		// abc/def/* 匹配abc/def/下的所有
		if strings.LastIndex(f, "*") > 0 {
			f = strings.Replace(f, "*", "", -1)
			abf, _ := filepath.Abs(f)
			filepath.Walk(abf, func(path string, info os.FileInfo, err error) error {

				if info.IsDir() {
					path, err := filepath.Abs(path)
					if err != nil {
						log.Fatal(err)
					}
					if err := watcher.Add(path); err != nil {
						log.Fatal(err)
					}
					log.Printf("add %s success \n", path)
				} else {

					if err := watcher.Add(path); err != nil {
						log.Fatal(err)
					}
					log.Printf("add %s success \n", path)
				}
				return nil
			})
		} else {
			watcher.Add(f)
		}

	}
	var it int64 = 3
	if c.Interval > it {
		it = c.Interval
	}
	t = time.NewTicker(time.Duration(it) * time.Second)
}
func startTicker() {

	for range t.C {
		str := q.String()
		if str != "" {
			txt := "[" + c.Title + " ] Warning !!! \n" + q.String()
			msg := tgbotapi.NewMessage(c.ChatId, txt)
			bot.Send(msg)
			q.Flush()
		}
	}
}

type Queue struct {
	mu    sync.Mutex
	max   int
	files []string
}

type Config struct {
	Title    string   // your server title or other identification
	Token    string   //telegram token
	ChatId   int64    //telegram chat id
	Interval int64    //interval to send notify ,min 3s
	Max      int      // max length to send queue message
	Path     []string // watch paths
}

func (e *Queue) String() (str string) {
	if len(e.files) == 0 {
		return
	}
	l := len(e.files)
	if l > e.max {
		l = e.max
	}
	for i := 0; i < l; i++ {
		str += e.files[i] + "\n"
	}
	return str
}
func (e *Queue) Add(str string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.files = append(e.files, str)
}
func (e *Queue) Flush() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.files = []string{}

}
