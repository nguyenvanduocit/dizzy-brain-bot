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

var allowedChatIDs = []int64{
	-1001905601063,
	1831420107,
}

var adminID int64 = 1831420107

func sendMessageToAdmin(bot *tgbotapi.BotAPI, message string) {
	msg := tgbotapi.NewMessage(adminID, message)
	msg.ParseMode = tgbotapi.ModeMarkdownV2
	if _, err := bot.Send(msg); err != nil {
		panic(err)
	}
}

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}

	// health check
	go func() {
		httpPort := os.Getenv("PORT")
		if httpPort == "" {
			httpPort = "8080"
		}

		logger.Info("http server started", zap.String("port", httpPort))

		if err := startHealthServer(httpPort); err != nil {
			logger.Error("http server error", zap.Error(err))
		}
	}()

	// the AI
	telegramToken := os.Getenv("TELEGRAM_API_TOKEN")
	if telegramToken == "" {
		logger.Panic("telegram api token is empty")
	}

	bot, err := tgbotapi.NewBotAPI(telegramToken)
	if err != nil {
		logger.Panic("telegram bot api", zap.Error(err))
	}
	bot.Debug = true

	sendMessageToAdmin(bot, "Bot started")
	defer sendMessageToAdmin(bot, "Bot stopped")

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

	for update := range updates {

		logger.Info("update", zap.Any("update", update))
		// ignore any non-Message Updates
		if update.Message == nil {
			continue
		}

		isAllowed := false
		for _, allowedChatID := range allowedChatIDs {
			if update.Message.Chat.ID == allowedChatID {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
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
		if bot.IsMessageToMe(message) || message.Chat.IsPrivate() {
			pool.Submit(func() {
				handleTextMessage(openAiClient, bot, message)
			})
			continue
		}
	}

}
func handleTextMessage(llmClient *LLMClient, bot *tgbotapi.BotAPI, message tgbotapi.Message) {
	conversationID := strconv.FormatInt(message.Chat.ID, 10)
	responseMessage, err := llmClient.GenerateText(conversationID, message.From.ID, message.Text)
	if err != nil {
		responseMessage = err.Error()
	}

	if responseMessage == "" {
		responseMessage = "I don't know what to say"
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, responseMessage)
	msg.ReplyMarkup = tgbotapi.ForceReply{
		InputFieldPlaceholder: "Type a message...",
	}
	msg.ParseMode = tgbotapi.ModeMarkdownV2
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
	case "context":
		llmClient.SetContext(strconv.FormatInt(message.Chat.ID, 10), message.CommandArguments())
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Set context successfully"))
	case "restart":
		// send terminate signal to the bot
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Restarting..."))
		os.Exit(0)
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
