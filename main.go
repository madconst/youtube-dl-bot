package main

import (
	"crypto/sha256"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/madconst/youtube-dl-bot/utils"
)

var bot *tgbotapi.BotAPI
var cfg *Config

type VideoDone struct {
	Update   tgbotapi.Update
	Filename string
}

type StatusMessage struct {
	Update    tgbotapi.Update
	MessageID tgbotapi.MessageID
	Message   string
}

func init() {
	var err error
	configPath := flag.String("config", "config.yaml", "Path to the configuration file")
	flag.Parse()
	cfg, err = LoadConfig(*configPath)
	if err != nil {
		log.Panic(err)
	}
	bot, err = tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		log.Panic(err)
	}
	bot.Debug = cfg.Debug
	if cfg.HttpServer != "" {
		go startHttpServer(cfg.HttpServer)
	}
}

func startHttpServer(addr string) {
	fs := http.FileServer(http.Dir(cfg.StorageDir))
	http.Handle("/", fs)
	log.Printf("Listening on %s...", addr)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func processUpdate(update tgbotapi.Update, done chan VideoDone, result chan StatusMessage) {
	// TODO: implement bot commands
	progress := make(chan string)
	if update.Message != nil {
		arg1 := update.Message.Text
		go func() {
			filename, err := downloadVideo(arg1, progress)
			if err != nil {
				log.Print(err.Error())
				result <- StatusMessage{update, tgbotapi.MessageID{}, "Download failed: " + err.Error()}
				return
			}
			done <- VideoDone{update, strings.TrimSuffix(filename, "\n")}
		}()
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Started")
		msg.ReplyToMessageID = update.Message.MessageID
		msgResult, err := bot.Send(msg)
		if err != nil {
			log.Print(err.Error())
		}
		output := []string{}
		remainder := ""
		lines := []string{}
		timeSent := time.Now()
		for chunk := range progress {
			lines, remainder = utils.ProcessConsoleOutput(remainder, chunk)
			output = append(output, lines...)
			elapsed := time.Since(timeSent)
			if elapsed < 2*time.Second {
				continue
			}
			m := tgbotapi.NewEditMessageText(
				update.Message.Chat.ID,
				msgResult.MessageID,
				fmt.Sprintf("```\n%s```\n", strings.Join(output, "\n")+remainder))
			m.ParseMode = "MarkdownV2"
			if _, err := bot.Send(m); err != nil {
				log.Print(err.Error())
			}
			timeSent = time.Now()
		}
	}
}

func escape(text string) string {
	re := regexp.MustCompile(`([-_*\[\]()~>#+-=|{}.!])`) // TODO: add backtick
	return re.ReplaceAllString(text, "\\$1")
}

func onVideoDone(video VideoDone) {
	msgText := ""
	for _, filePath := range strings.Split(video.Filename, "\n") {
		_, filename := path.Split(filePath)
		videoUrl := makeVideoUrl(filePath)
		msgText += fmt.Sprintf("[%s](%s)\n", escape(filename), videoUrl.String())
	}
	msg := tgbotapi.NewMessage(video.Update.Message.Chat.ID, msgText)
	msg.ParseMode = "MarkdownV2"
	msg.ReplyToMessageID = video.Update.Message.MessageID
	if _, err := bot.Send(msg); err != nil {
		log.Print(err.Error())
	}
}

func onStatusMessage(message StatusMessage) {
	msg := tgbotapi.NewMessage(message.Update.Message.Chat.ID, message.Message)
	msg.ReplyToMessageID = message.Update.Message.MessageID
	msg.ParseMode = "MarkdownV2"
	if _, err := bot.Send(msg); err != nil {
		log.Print(err.Error())
	}
}

func makeCommand(videoUrl string, args ...string) *exec.Cmd {
	dirname := makeHash([]byte(videoUrl))
	outputTemplate := path.Join(dirname, "%(title)s.%(ext)s")
	args = append(args, videoUrl)
	args = append([]string{
		"-o", outputTemplate,
		"-f", "bestvideo[height<=1080]+bestaudio,bestaudio",
		"--merge-output-format", "mp4",
	}, args...)
	cmd := exec.Command(cfg.DownloaderPath, args...)
	cmd.Dir = cfg.StorageDir
	return cmd
}

func downloadVideo(videoUrl string, progress chan string) (string, error) {
	_, err := url.Parse(videoUrl)
	if err != nil {
		return "", err
	}
	cmd := makeCommand(videoUrl, "--write-info-json")
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	err = cmd.Start()
	if err != nil {
		return "", err
	}
	buff := make([]byte, 65536)
	for {
		n, err := stdout.Read(buff)
		if err == io.EOF {
			close(progress)
			break
		}
		if err != nil {
			return "", err
		}
		chunk := string(buff[:n])
		progress <- chunk
		showSpecialChars := strings.Replace(chunk, "\r", "<R>", -1)
		showSpecialChars = strings.Replace(chunk, "\n", "<N>", -1)
		log.Printf("Chunk: %s\n", showSpecialChars)
	}
	if _, err := io.Copy(os.Stderr, stderr); err != nil {
		return "", err
	}
	err = cmd.Wait()
	if err != nil {
		return "", err
	}
	cmd = makeCommand(videoUrl, "--print", "filename")
	filename, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(filename), nil
}

func makeHash(src []byte) string {
	sum := sha256.Sum256(src)
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(sum[:])
}

func makeVideoUrl(filename string) *url.URL {
	u, err := url.Parse(cfg.BaseUrl)
	if err != nil {
		log.Panic(cfg.BaseUrl+filename, err)
	}
	u.Path = path.Join(u.Path, filename)
	return u
}

func main() {
	log.Printf("Authorized %s", bot.Self.UserName)
	offset := 0
	u := tgbotapi.NewUpdate(offset)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)
	done := make(chan VideoDone)
	messages := make(chan StatusMessage)
	for {
		select {
		case update := <-updates:
			go processUpdate(update, done, messages)
		case video := <-done:
			onVideoDone(video)
		case message := <-messages:
			onStatusMessage(message)
		}
	}
}
