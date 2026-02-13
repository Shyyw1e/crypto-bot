package grpcserver

import (
	"context"

	"github.com/Shyyw1e/crypto-bot/internal/analyser/domain"
	"github.com/Shyyw1e/crypto-bot/internal/analyser/usecase"
	"github.com/Shyyw1e/crypto-bot/internal/proto/analyserpb"
	"github.com/Shyyw1e/crypto-bot/internal/shared/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SettingsServer реализует gRPC-сервис AnalyserSettingsService,
// который описан в analyser_user_settings.proto.
// Его поднимает analyser, а tg-bot ходит к нему как клиентом.
type SettingsServer struct {
	analyserpb.UnimplementedAnalyserSettingsServiceServer

	log logger.Logger
	svc *usecase.Service
}

func NewSettingsServer(log logger.Logger, svc *usecase.Service) *SettingsServer {
	return &SettingsServer{
		log: log.With("component", "settings_grpc"),
		svc: svc,
	}
}

// UpsertUserSettings — создать/обновить настройки пользователя.
// tg-bot дергает это, когда пользователь проходит мастер настройки в телеге.
func (s *SettingsServer) UpsertUserSettings(
	ctx context.Context,
	req *analyserpb.UpsertUserSettingsRequest,
) (*analyserpb.UpsertUserSettingsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}

	

	// !!! ВАЖНО:
	// Здесь я предполагаю, что в proto у UpsertUserSettingsRequest есть поле:
	//   UserSettings settings = ...;
	// Тогда в Go будет метод req.GetSettings().
	// Если у тебя в proto поле называется иначе (например, user_settings),
	// метод будет GetUserSettings() — тогда просто поменяй эту строчку.
	settingsMsg := req.GetSettings()
	if settingsMsg == nil {
		return nil, status.Error(codes.InvalidArgument, "settings is required")
	}

	chatID := settingsMsg.GetChatId()
	if chatID == 0 {
		return nil, status.Error(codes.InvalidArgument, "chat_id is required")
	}

	s.log.Info("analyser_grpc_upsert_user_settings_called",
		"chat_id", chatID,
		"watch_fact", settingsMsg.GetWatchFact(),
		"watch_potential", settingsMsg.GetWatchPotential(),
		"min_diff_fact", settingsMsg.GetMinDiffFact(),
		"min_diff_potential", settingsMsg.GetMinDiffPotential(),
		"max_notional", settingsMsg.GetMaxNotional(),
	)

	// Маппим proto → domain.UserSettings.
	// Эти поля используются в MatchUserSettings:
	//   ChatID, WatchFact, WatchPotential, MinDiffFact, MinDiffPotential, MaxNotional.
	domainSettings := &domain.UserSettings{
		ChatID:           chatID,
		WatchFact:        settingsMsg.GetWatchFact(),
		WatchPotential:   settingsMsg.GetWatchPotential(),
		MinDiffFact:      settingsMsg.GetMinDiffFact(),
		MinDiffPotential: settingsMsg.GetMinDiffPotential(),
		MaxNotional:      settingsMsg.GetMaxNotional(),
		// Если в домене есть Active/CreatedAt/UpdatedAt — сами выставляем в usecase.
	}

	if err := s.svc.UpsertUserSettings(ctx, domainSettings); err != nil {
		s.log.Error("analyser_grpc_upsert_user_settings_failed",
			"chat_id", chatID,
			"err", err,
		)
		return nil, status.Errorf(codes.Internal, "upsert user settings: %v", err)
	}

	s.log.Info("analyser_grpc_upsert_user_settings_ok", "chat_id", chatID)

	return &analyserpb.UpsertUserSettingsResponse{
	}, nil
}

// SetUserActive — включить/выключить пользователя.
// Можно дергать из tg-bot, когда юзер жмёт "пауза"/"возобновить".
func (s *SettingsServer) SetUserActive(
	ctx context.Context,
	req *analyserpb.SetUserActiveRequest,
) (*analyserpb.SetUserActiveResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}

	chatID := req.GetChatId()
	if chatID == 0 {
		return nil, status.Error(codes.InvalidArgument, "chat_id is required")
	}

	// Предполагаю, что в SetUserActiveRequest есть поле:
	//   bool active = ...;
	active := req.GetIsActive()

	s.log.Info("analyser_grpc_set_user_active_called",
		"chat_id", chatID,
		"active", active,
	)

	if err := s.svc.SetUserActive(ctx, chatID, active); err != nil {
		s.log.Error("analyser_grpc_set_user_active_failed",
			"chat_id", chatID,
			"active", active,
			"err", err,
		)
		return nil, status.Errorf(codes.Internal, "set user active: %v", err)
	}

	s.log.Info("analyser_grpc_set_user_active_ok",
		"chat_id", chatID,
		"active", active,
	)

	return &analyserpb.SetUserActiveResponse{
	}, nil
}
