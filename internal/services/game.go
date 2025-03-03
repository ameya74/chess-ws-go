package services

import (
	"fmt"
	"sync"

	"github.com/corentings/chess/v2"
	"github.com/google/uuid"
)

// GameService handles chess game logic
type GameService struct {
	games      map[string]*chess.Game
	gameStates map[string]*GameState
	mu         sync.Mutex
}

// GameState represents the current state of a chess game
type GameState struct {
	WhitePlayer string
	BlackPlayer string
	CurrentTurn chess.Color
	DrawOffered bool
	TimeControl struct {
		WhiteTimeLeft float64
		BlackTimeLeft float64
	}
	ChatHistory []ChatMessage
}

// ChatMessage represents a chat message in a game
type ChatMessage struct {
	Sender  string
	Message string
}

// NewGameService creates a new game service
func NewGameService() *GameService {
	return &GameService{
		games:      make(map[string]*chess.Game),
		gameStates: make(map[string]*GameState),
	}
}

// CreateGame creates a new chess game and returns its ID
func (s *GameService) CreateGame(whitePlayer, blackPlayer string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	gameID := uuid.New().String()
	s.games[gameID] = chess.NewGame()
	s.gameStates[gameID] = &GameState{
		WhitePlayer: whitePlayer,
		BlackPlayer: blackPlayer,
		CurrentTurn: chess.White,
		DrawOffered: false,
		TimeControl: struct {
			WhiteTimeLeft float64
			BlackTimeLeft float64
		}{
			WhiteTimeLeft: 600, // 10 minutes in seconds
			BlackTimeLeft: 600,
		},
		ChatHistory: []ChatMessage{},
	}

	return gameID
}

// GetGame returns a chess game by ID
func (s *GameService) GetGame(gameID string) (*chess.Game, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	game, exists := s.games[gameID]
	if !exists {
		return nil, fmt.Errorf("game not found")
	}
	return game, nil
}

// GetGameState returns the state of a game by ID
func (s *GameService) GetGameState(gameID string) (*GameState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, exists := s.gameStates[gameID]
	if !exists {
		return nil, fmt.Errorf("game state not found")
	}
	return state, nil
}

// MakeMove makes a move in a chess game
func (s *GameService) MakeMove(gameID, moveStr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	game, exists := s.games[gameID]
	if !exists {
		return fmt.Errorf("game not found")
	}

	state, exists := s.gameStates[gameID]
	if !exists {
		return fmt.Errorf("game state not found")
	}

	// Verify it's the correct player's turn
	if game.Position().Turn() != state.CurrentTurn {
		return fmt.Errorf("not your turn")
	}

	// Make the move
	err := game.PushMove(moveStr, nil)
	if err != nil {
		return fmt.Errorf("invalid move: %w", err)
	}

	// Update turn
	state.CurrentTurn = chess.Color(1 - int(state.CurrentTurn))

	// Reset draw offer after a move
	state.DrawOffered = false

	return nil
}

// ResignGame handles a player resigning
func (s *GameService) ResignGame(gameID string, color chess.Color) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	game, exists := s.games[gameID]
	if !exists {
		return fmt.Errorf("game not found")
	}

	// Set the game as resigned
	if color == chess.White {
		game.Resign(chess.White)
	} else {
		game.Resign(chess.Black)
	}

	return nil
}

// OfferDraw offers a draw in a game
func (s *GameService) OfferDraw(gameID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, exists := s.gameStates[gameID]
	if !exists {
		return fmt.Errorf("game state not found")
	}

	state.DrawOffered = true
	return nil
}

// AcceptDraw accepts a draw offer
func (s *GameService) AcceptDraw(gameID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	game, exists := s.games[gameID]
	if !exists {
		return fmt.Errorf("game not found")
	}

	state, exists := s.gameStates[gameID]
	if !exists {
		return fmt.Errorf("game state not found")
	}

	if !state.DrawOffered {
		return fmt.Errorf("no draw was offered")
	}

	// Set the game as drawn by agreement
	game.Draw(chess.DrawOffer)
	return nil
}

// DeclineDraw declines a draw offer
func (s *GameService) DeclineDraw(gameID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, exists := s.gameStates[gameID]
	if !exists {
		return fmt.Errorf("game state not found")
	}

	state.DrawOffered = false
	return nil
}

// UpdateTime updates the remaining time for a player
func (s *GameService) UpdateTime(gameID string, color chess.Color, timeLeft float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, exists := s.gameStates[gameID]
	if !exists {
		return fmt.Errorf("game state not found")
	}

	if color == chess.White {
		state.TimeControl.WhiteTimeLeft = timeLeft
	} else {
		state.TimeControl.BlackTimeLeft = timeLeft
	}

	return nil
}

// AddChatMessage adds a chat message to the game
func (s *GameService) AddChatMessage(gameID, sender, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, exists := s.gameStates[gameID]
	if !exists {
		return fmt.Errorf("game state not found")
	}

	state.ChatHistory = append(state.ChatHistory, ChatMessage{
		Sender:  sender,
		Message: message,
	})

	return nil
}

// IsGameOver checks if a game is over
func (s *GameService) IsGameOver(gameID string) (bool, chess.Outcome, chess.Method, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	game, exists := s.games[gameID]
	if !exists {
		return false, chess.NoOutcome, chess.NoMethod, fmt.Errorf("game not found")
	}

	isOver := game.Outcome() != chess.NoOutcome
	return isOver, game.Outcome(), game.Method(), nil
}

// GetActiveGamesCount returns the number of active games
func (s *GameService) GetActiveGamesCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.games)
}
