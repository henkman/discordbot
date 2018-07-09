package main

import (
	"bytes"
	"flag"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/bwmarrin/discordgo"
	"github.com/henkman/markov"
)

func main() {
	var opts struct {
		Token      string
		MinWords   uint
		MaxWords   uint
		MaxHistory int
	}
	flag.StringVar(&opts.Token, "t", "", "token")
	flag.UintVar(&opts.MinWords, "min", 1, "min words")
	flag.UintVar(&opts.MaxWords, "max", 10, "max words")
	flag.IntVar(&opts.MaxHistory, "hmax", -1, "max history messages")
	flag.Parse()
	if opts.Token == "" {
		flag.Usage()
		return
	}
	dg, err := discordgo.New("Bot " + opts.Token)
	if err != nil {
		log.Println(err)
		return
	}
	var tg markov.TextGenerator
	tg.Init(time.Now().Unix())
	dg.AddHandler(onReady(&tg, opts.MaxHistory))
	dg.AddHandler(onMessage(&tg, opts.MinWords, opts.MaxWords))
	if err := dg.Open(); err != nil {
		log.Println(err)
		return
	}
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
	dg.Close()
}

var (
	reReplaceEmotes = regexp.MustCompile(`<a?(:[^:]+:)\d+>`)
)

func isValidMessageContent(m string) bool {
	return !strings.Contains(m, "http") &&
		!strings.HasPrefix(m, "/") &&
		!strings.Contains(m, "<@")
}

func onReady(tg *markov.TextGenerator, maxHistory int) interface{} {
	return func(s *discordgo.Session, event *discordgo.Ready) {
		s.UpdateStatus(0, "Sshhhhh! Attack on my mark!")
		gs, err := s.UserGuilds(0, "", "")
		if err != nil {
			s.Close()
			return
		}
		var buf bytes.Buffer
		for _, g := range gs {
			log.Println("reading guild", g.Name)
			chs, err := s.GuildChannels(g.ID)
			if err != nil {
				s.Close()
				return
			}
		channel:
			for _, ch := range chs {
				if ch.Type != discordgo.ChannelTypeGuildText {
					continue
				}
				log.Println("reading channel", ch.Name)
				n := 0
				before := ""
				for {
					ms, err := s.ChannelMessages(ch.ID, 100, before, "", "")
					if err != nil {
						s.Close()
						return
					}
					if len(ms) == 0 {
						break
					}
					for _, m := range ms {
						if m.Author.ID == s.State.User.ID ||
							!isValidMessageContent(m.Content) {
							continue
						}
						msg := m.Content
						msg = reReplaceEmotes.ReplaceAllString(msg, "$1")
						msg = strings.ToLower(msg)
						buf.WriteString(msg)
						tg.Feed(&buf)
						buf.Reset()
						n++
						if maxHistory != -1 && n >= maxHistory {
							continue channel
						}
					}
					last := ms[len(ms)-1]
					before = last.ID
				}
			}
		}
	}
}

func onMessage(tg *markov.TextGenerator, minWords, maxWords uint) interface{} {
	mr := rand.New(rand.NewSource(time.Now().Unix()))
	return func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID {
			return
		}
		mentioned := false
		for _, ment := range m.Mentions {
			if ment.ID == s.State.User.ID {
				mentioned = true
				break
			}
		}
		if !mentioned {
			msg := m.Content
			msg = reReplaceEmotes.ReplaceAllString(msg, "$1")
			msg = strings.ToLower(msg)
			if m.Type != discordgo.MessageTypeDefault ||
				!isValidMessageContent(m.Content) {
				return
			}
			tg.Feed(bytes.NewBufferString(msg))
			return
		}
		x := mr.Int31n(int32(maxWords-minWords)+1) + int32(minWords)
		text := WordJoin(tg.Generate(uint(x)))
		go s.ChannelMessageDelete(m.ChannelID, m.ID)
		s.ChannelMessageSend(m.ChannelID, text)
	}
}

func WordJoin(words []string) string {
	text := ""
	for i, _ := range words {
		text += words[i]
		isLast := i == (len(words) - 1)
		if !isLast {
			next := words[i+1]
			fc := []rune(next)[0]
			word := []rune(words[i])
			lc := word[len(word)-1]
			if lc == '.' || lc == ',' || lc == '?' || lc == '!' || lc == ';' ||
				(unicode.IsLetter(lc) || unicode.IsDigit(lc)) &&
					(unicode.IsLetter(fc) || unicode.IsDigit(fc)) {
				text += " "
			}
		}
	}
	return text
}
