package main

import (
	"fmt"
	"github.com/alitto/pond"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
	"net/http"
	"os"
	"strconv"
)

const allowedChatID = -1001905601063

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	telegramToken := os.Getenv("TELEGRAM_API_TOKEN")
	if telegramToken == "" {
		logger.Panic("telegram api token is empty")
	}

	bot, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		logger.Panic("telegram bot api", zap.Error(err))
	}
	bot.Debug = true

	palmApiToken := os.Getenv("PALM_API_KEY")
	if palmApiToken == "" {
		logger.Panic("PALM_API_KEY is empty")
	}
	openAiClient := NewLLMClient(palmApiToken)

	pool := pond.New(100, 1000)
	defer pool.StopAndWait()

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30
	updates := bot.GetUpdatesChan(updateConfig)

	go func() {
		// health check http handler
		httpPort := os.Getenv("PORT")
		if httpPort == "" {
			httpPort = "8080"
		}

		logger.Info("http server started", zap.String("port", httpPort))

		if err := startHealthServer(httpPort); err != nil {
			logger.Error("http server error", zap.Error(err))
		}
	}()

	for update := range updates {

		logger.Info("update", zap.Any("update", update))
		// ignore any non-Message Updates
		if update.Message == nil {
			continue
		}

		if update.FromChat().ID != allowedChatID {
			bot.Send(tgbotapi.NewMessage(update.FromChat().ID, "To use this bot please contact @henry_duocnv"))
			continue
		}

		if update.Message.From.IsBot {
			continue
		}

		// make a copy of the message, because we need to pass it to the goroutine
		message := *update.Message
		// handle commands
		if message.IsCommand() {
			pool.Submit(func() {
				handleCommand(openAiClient, bot, message)
			})
			continue
		}

		// handle text messages
		if bot.IsMessageToMe(message) {
			pool.Submit(func() {
				handleTextMessage(openAiClient, bot, message)
			})
			continue
		}
	}

}
func handleTextMessage(llmClient *LLMClient, bot *tgbotapi.BotAPI, message tgbotapi.Message) {
	conversationID := strconv.FormatInt(message.Chat.ID, 10)
	responseMessage, err := llmClient.GenerateText(conversationID, message.From.UserName, message.Text)
	if err != nil {
		responseMessage = err.Error()
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, responseMessage)
	msg.ReplyToMessageID = message.MessageID

	if _, err := bot.Send(msg); err != nil {
		panic(err)
	}
}

func handleCommand(llmClient *LLMClient, bot *tgbotapi.BotAPI, message tgbotapi.Message) {
	switch message.Command() {
	case "reset":
		llmClient.ResetConversation(strconv.FormatInt(message.Chat.ID, 10))
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Reset successfully"))
	}
}

func startHealthServer(httpPort string) error {
	handler := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "OK")
	}

	// Listen on port 8080.
	http.HandleFunc("/healthz", handler)
	if err := http.ListenAndServe(":"+httpPort, nil); err != nil {
		return err
	}

	return nil
}
