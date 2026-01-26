package usecase

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"




func mainKeyboard(active bool) tgbotapi.ReplyKeyboardMarkup {
    // active = true  -> показываем "Остановить анализ"
    // active = false -> "Начать анализ"
    var row1 []tgbotapi.KeyboardButton
    if active {
        row1 = []tgbotapi.KeyboardButton{
            tgbotapi.NewKeyboardButton("Остановить анализ"),
        }
    } else {
        row1 = []tgbotapi.KeyboardButton{
            tgbotapi.NewKeyboardButton("Начать анализ"),
        }
    }
    row2 := []tgbotapi.KeyboardButton{
        tgbotapi.NewKeyboardButton("Изменить параметры"),
    }

    kb := tgbotapi.NewReplyKeyboard(row1, row2)
    kb.ResizeKeyboard = true
    kb.OneTimeKeyboard = false
    return kb
}
