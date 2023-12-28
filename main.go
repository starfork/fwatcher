package main

import (
	"log"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/spf13/viper"
)

type Config struct {
	Token    string   //telegram token
	ChatId   int64    //telegram chat id
	Interval int64    //interval to send notify ,min 3s
	Max      int      // max length to send queue message
	Path     []string // watch paths
}

func main() {
	viper.SetConfigFile("config.yaml")
	c := &Config{}
	viper.ReadInConfig()
	viper.Unmarshal(c)

	// Create new watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()
	bot, err := tgbotapi.NewBotAPI(c.Token)
	q := &Queue{
		max: c.Max,
	}
	// Start listening for events.
	go func() {
		for {
			event, ok := <-watcher.Events
			if !ok {
				return
			}
			if event.Has(fsnotify.Create) {
				if err != nil {
					log.Panic(err)
				}
				q.Add(event.Name)
			}
		}
	}()
	for _, f := range c.Path {
		watcher.Add(f)
	}
	go func() {
		var it int64 = 3
		if c.Interval > it {
			it = c.Interval
		}
		for range time.Tick(time.Duration(it) * time.Second) {
			str := q.String()
			if str != "" {
				txt := "!!! Warning !!! \n" + q.String() + " Has Changed!"
				msg := tgbotapi.NewMessage(c.ChatId, txt)
				bot.Send(msg)
				q.Flush()
			}
		}

	}()

	// Block main goroutine forever.
	<-make(chan struct{})
}

type Queue struct {
	mu    sync.Mutex
	max   int
	files []string
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
