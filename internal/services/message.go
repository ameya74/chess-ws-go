package services

import (
	"fmt"
	"sync"

	"github.com/corentings/chess/v2"
	"github.com/gorilla/websocket"
)

type MessageService struct {
	games           map[*websocket.Conn]*chess.Game
	mu              sync.Mutex
	messageChannels map[*websocket.Conn]chan interface{}
}

func NewMessageService() *MessageService {
	return &MessageService{
		games:           make(map[*websocket.Conn]*chess.Game),
		messageChannels: make(map[*websocket.Conn]chan interface{}),
	}
}

func (s *MessageService) MakeMove(conn *websocket.Conn, algebraicMove string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	game, ok := s.games[conn]
	if !ok {
			game = chess.NewGame()
			s.games[conn] = game
	}

	// Use PushMove with options (if needed)
	// Example: options := &chess.PushMoveOptions{Strict: true}
	options := &chess.PushMoveOptions{} // Default options
	err := game.PushMove(algebraicMove, options)
	if err != nil {
			return fmt.Errorf("invalid move: %w", err)
	}

	return nil
}
func (s *MessageService) GetGame(conn *websocket.Conn) *chess.Game {
	s.mu.Lock()
	defer s.mu.Unlock()
	game, ok := s.games[conn]
	if !ok {
		return nil
	}
	return game
}

func (s *MessageService) GetGameState(conn *websocket.Conn) interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	game := s.games[conn]
	if game == nil {
		return nil
	}

	return struct {
		Board string `json:"board"`
	}{
		Board: game.Position().Board().Draw(),
	}
}

func (s *MessageService) GetGameResult(conn *websocket.Conn) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	game := s.games[conn]
	if game == nil {
		return ""
	}
	return fmt.Sprintf("%s by %s", game.Outcome(), game.Method())
}

func (s *MessageService) GetMessageChannel(conn *websocket.Conn) chan interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch, ok := s.messageChannels[conn]
	if !ok {
		ch = make(chan interface{})
		s.messageChannels[conn] = ch
	}
	return ch
}

func (s *MessageService) IsGameOver(conn *websocket.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	game, ok := s.games[conn]
	if !ok {
		return false
	}
	return game.Outcome() != chess.NoOutcome
}
