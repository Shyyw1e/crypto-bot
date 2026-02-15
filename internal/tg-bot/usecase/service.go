package usecase

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/Shyyw1e/crypto-bot/internal/proto/analyserpb"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type Service struct {
	log      logger.Logger
	bot      *tgbotapi.BotAPI
	analyser analyserpb.AnalyserSettingsServiceClient

	mu      sync.RWMutex
	dialogs map[int64]*DialogState // chatID -> состояние мастера
}

func NewService(log logger.Logger, bot *tgbotapi.BotAPI, analyser analyserpb.AnalyserSettingsServiceClient) *Service {
	return &Service{
		log:      log,
		bot:      bot,
		analyser: analyser,
		dialogs:  make(map[int64]*DialogState),
	}
}

// /start
func (s *Service) handleStartCommand(ctx context.Context, chatID int64) error {
	s.resetDialog(chatID)

	welcome := "👋 Привет! Давай настроим, какие ситуации отслеживать."
	m := tgbotapi.NewMessage(chatID, welcome)
	m.ReplyMarkup = mainKeyboard(false) // ещё не активен
	if _, err := s.bot.Send(m); err != nil {
		s.log.Error("tgbot_send_welcome_failed", "chat_id", chatID, "err", err)
		return err
	}

	return s.startWizard(ctx, chatID)
}

// запуск мастера настроек
func (s *Service) startWizard(ctx context.Context, chatID int64) error {
	_ = ctx // на будущее (tracing, отмена) — сейчас Telegram-клиент без ctx

	dlg := s.getDialog(chatID)
	dlg.Step = StepChooseType

	text := "Что отслеживать?\n" +
		"• Только факт\n" +
		"• Только потенциал\n" +
		"• Факт + потенциал\n\n" +
		"Напиши один из вариантов: `факт`, `потенциал`, `оба`."

	m := tgbotapi.NewMessage(chatID, text)
	m.ParseMode = "Markdown"
	if _, err := s.bot.Send(m); err != nil {
		s.log.Error("tgbot_send_choose_type_failed", "chat_id", chatID, "err", err)
		return err
	}
	return nil
}

// потокобезопасный доступ к dialogs
func (s *Service) getDialog(chatID int64) *DialogState {
	s.mu.Lock()
	defer s.mu.Unlock()

	dlg, ok := s.dialogs[chatID]
	if !ok {
		dlg = &DialogState{Step: StepIdle}
		s.dialogs[chatID] = dlg
	}
	return dlg
}

func (s *Service) resetDialog(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.dialogs[chatID] = &DialogState{Step: StepIdle}
}

func (s *Service) HandleUpdate(ctx context.Context, upd tgbotapi.Update) error {
	if upd.Message == nil {
		return nil
	}

	chatID := upd.Message.Chat.ID
	msg := upd.Message

	// Команды
	if msg.IsCommand() {
		switch msg.Command() {
		case "start":
			return s.handleStartCommand(ctx, chatID)
		case "stop":
			// оставим для обратной совместимости
			return s.handleStop(ctx, chatID)
		default:
			m := tgbotapi.NewMessage(chatID, "Неизвестная команда. Доступны: /start, /stop")
			_, _ = s.bot.Send(m)
			return nil
		}
	}

	// Текст с клавиатуры
	switch msg.Text {
	case "Начать анализ":
		return s.handleStartAnalysis(ctx, chatID)
	case "Остановить анализ":
		return s.handleStop(ctx, chatID)
	case "Изменить параметры":
		return s.startWizard(ctx, chatID)
	default:
		// если сейчас идёт мастер — трактуем как шаг мастера
		return s.handleWizardMessage(ctx, chatID, msg.Text)
	}
}

func (s *Service) handleWizardMessage(ctx context.Context, chatID int64, text string) error {
    dlg := s.getDialog(chatID)

    switch dlg.Step {
    case StepChooseType:
        return s.handleChooseType(ctx, chatID, dlg, text)
    case StepInputMinDiffFact:
        return s.handleMinDiffFact(ctx, chatID, dlg, text)
    case StepInputMinDiffPot:
        return s.handleMinDiffPot(ctx, chatID, dlg, text)
    case StepInputMaxNotional: // 🆕
        return s.handleMaxNotional(ctx, chatID, dlg, text)
    default:
        // мастер не активен — игнор
        return nil
    }
}



func (s *Service) handleChooseType(ctx context.Context, chatID int64, dlg *DialogState, text string) error {
	_ = ctx // пока здесь нет внешних вызовов кроме Telegram

	t := strings.ToLower(strings.TrimSpace(text))

	switch t {
	case "факт":
		dlg.WatchType = WatchFactOnly
		dlg.Step = StepInputMinDiffFact
		msg := tgbotapi.NewMessage(chatID, "Введите минимальный дифф для *факта* (например, 0.03. Обратите внимание: ситуации с разницей <= 0.01 автоматически игнорируются):")
		msg.ParseMode = "Markdown"
		_, err := s.bot.Send(msg)
		return err
	case "потенциал":
		dlg.WatchType = WatchPotentialOnly
		dlg.Step = StepInputMinDiffPot
		msg := tgbotapi.NewMessage(chatID, "Введите минимальный дифф для *потенциала* (например, 0.03. Обратите внимание: ситуации с разницей <= 0.01 автоматически игнорируются):")
		msg.ParseMode = "Markdown"
		_, err := s.bot.Send(msg)
		return err
	case "оба":
		dlg.WatchType = WatchBoth
		dlg.Step = StepInputMinDiffFact
		msg := tgbotapi.NewMessage(chatID, "Введите минимальный дифф для *факта* (например, 0.03. Обратите внимание: ситуации с разницей <= 0.01 автоматически игнорируются):")
		msg.ParseMode = "Markdown"
		_, err := s.bot.Send(msg)
		return err
	default:
		_, err := s.bot.Send(tgbotapi.NewMessage(chatID, "Не понял. Напишите: `факт`, `потенциал` или `оба`."))
		return err
	}
}

func parseFloat(text string) (float64, error) {
	text = strings.ReplaceAll(text, ",", ".")
	text = strings.TrimSpace(text)
	return strconv.ParseFloat(text, 64)
}

func (s *Service) handleMinDiffFact(ctx context.Context, chatID int64, dlg *DialogState, text string) error {
    _ = ctx

    v, err := parseFloat(text)
    if err != nil || v <= 0 {
        _, _ = s.bot.Send(tgbotapi.NewMessage(chatID, "Не получилось прочитать число, попробуйте ещё раз, например: 0.03"))
        return nil
    }
    dlg.TempMinDiffFact = v

    if dlg.WatchType == WatchFactOnly {
        // 🆕 вместо finishWizard — отдельный шаг для max_notional
        dlg.Step = StepInputMaxNotional
        msg := tgbotapi.NewMessage(
            chatID,
            "Теперь введите *максимальный объём в USDT*, на который готовы заходить.\n"+
                "Например: `1000`\nЕсли хотите без лимита — введите `0`.",
        )
        msg.ParseMode = "Markdown"
        _, err := s.bot.Send(msg)
        return err
    }

    // иначе надо спросить ещё потенциал
    dlg.Step = StepInputMinDiffPot
    msg := tgbotapi.NewMessage(chatID, "Теперь введите минимальный дифф для *потенциала* (например, 0.03. Обратите внимание: ситуации с разницей <= 0.01 автоматически игнорируются):")
    msg.ParseMode = "Markdown"
    _, err = s.bot.Send(msg)
    return err
}


func (s *Service) handleMinDiffPot(ctx context.Context, chatID int64, dlg *DialogState, text string) error {
    _ = ctx

    v, err := parseFloat(text)
    if err != nil || v <= 0 {
        _, _ = s.bot.Send(tgbotapi.NewMessage(chatID, "Не получилось прочитать число, попробуйте ещё раз, например: 0.03"))
        return nil
    }
    dlg.TempMinDiffPotential = v

    // 🆕 следующий шаг — ввод max_notional
    dlg.Step = StepInputMaxNotional
    msg := tgbotapi.NewMessage(
        chatID,
        "Теперь введите *максимальный объём в USDT*, на который готовы заходить.\n"+
            "Например: `1000`\nЕсли хотите без лимита — введите `0`.",
    )
    msg.ParseMode = "Markdown"
    _, err = s.bot.Send(msg)
    return err
}

func (s *Service) handleMaxNotional(ctx context.Context, chatID int64, dlg *DialogState, text string) error {
    _ = ctx

    v, err := parseFloat(text)
    // разрешаем 0 (без лимита), но не разрешаем отрицательные
    if err != nil || v < 0 {
        _, _ = s.bot.Send(tgbotapi.NewMessage(
            chatID,
            "Не получилось прочитать число, попробуйте ещё раз.\n"+
                "Пример: `1000` или `0` для отсутствия лимита.",
        ))
        return nil
    }

    dlg.TempMaxNotional = v

    return s.finishWizard(ctx, chatID, dlg)
}


func (s *Service) finishWizard(ctx context.Context, chatID int64, dlg *DialogState) error {
    // Собираем настройки для analyser’а
    watchFact := dlg.WatchType == WatchFactOnly || dlg.WatchType == WatchBoth
    watchPot := dlg.WatchType == WatchPotentialOnly || dlg.WatchType == WatchBoth

    settings := &analyserpb.UserSettings{
        ChatId:           chatID,
        WatchFact:        watchFact,
        WatchPotential:   watchPot,
        MinDiffFact:      dlg.TempMinDiffFact,
        MinDiffPotential: dlg.TempMinDiffPotential,
        MaxNotional:      dlg.TempMaxNotional, // 🆕 тут юзерский лимит
        IsActive:         false,
    }

    if _, err := s.analyser.UpsertUserSettings(ctx,
        &analyserpb.UpsertUserSettingsRequest{Settings: settings},
    ); err != nil {
        s.log.Error("tgbot_upsert_settings_failed", "chat_id", chatID, "err", err)
        s.bot.Send(tgbotapi.NewMessage(chatID, "Не удалось сохранить настройки, попробуйте позже."))
        return err
    }

    dlg.Step = StepIdle

    summary := fmt.Sprintf(
        "Сохранил настройки:\n"+
            "• Факт: %v (мин. дифф %.3f)\n"+
            "• Потенциал: %v (мин. дифф %.3f)\n"+
            "• Макс. объём: %.2f USDT (0 = без лимита)\n\n"+
            "Нажмите «Начать анализ», чтобы запустить.",
        watchFact, settings.MinDiffFact,
        watchPot, settings.MinDiffPotential,
        settings.MaxNotional,
    )

    msg := tgbotapi.NewMessage(chatID, summary)
    msg.ReplyMarkup = mainKeyboard(false) // ещё не активен
    _, err := s.bot.Send(msg)
    return err
}


func (s *Service) handleStartAnalysis(ctx context.Context, chatID int64) error {
	_, err := s.analyser.SetUserActive(ctx, &analyserpb.SetUserActiveRequest{
		ChatId:   chatID,
		IsActive: true,
	})
	if err != nil {
		s.log.Error("tgbot_start_analysis_failed", "chat_id", chatID, "err", err)
		s.bot.Send(tgbotapi.NewMessage(chatID, "Не удалось запустить анализ, попробуйте позже."))
		return err
	}

	msg := tgbotapi.NewMessage(chatID, "🚀 Анализ запущен. Буду присылать сигналы, как только появятся.")
	msg.ReplyMarkup = mainKeyboard(true)
	_, err = s.bot.Send(msg)
	return err
}

func (s *Service) handleStop(ctx context.Context, chatID int64) error {
	_, err := s.analyser.SetUserActive(ctx, &analyserpb.SetUserActiveRequest{
		ChatId:   chatID,
		IsActive: false,
	})
	if err != nil {
		s.log.Error("tgbot_stop_analysis_failed", "chat_id", chatID, "err", err)
		s.bot.Send(tgbotapi.NewMessage(chatID, "Не удалось остановить анализ, попробуйте позже."))
		return err
	}

	msg := tgbotapi.NewMessage(chatID, "🛑 Анализ остановлен. В любой момент можно снова нажать «Начать анализ».")
	msg.ReplyMarkup = mainKeyboard(false)
	_, err = s.bot.Send(msg)
	return err
}
