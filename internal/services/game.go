package services

import (
	"context"
	"fmt"
	"math"
	"sync"

	"chess-ws-go/internal/repositories"

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
func (s *GameService) MakeMove(gameID, moveStr string, ctx context.Context, userRepo repositories.UserRepository) error {
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

	// Check if the game is over after this move
	isOver, _, _, _ := s.IsGameOver(gameID)
	if isOver && ctx != nil && userRepo != nil {
		// Update ELO ratings if game is over
		_ = s.UpdatePlayerRatings(ctx, gameID, state.WhitePlayer, state.BlackPlayer, userRepo)
	}

	return nil
}

// ResignGame handles a player resigning
func (s *GameService) ResignGame(gameID string, color chess.Color, ctx context.Context, userRepo repositories.UserRepository) error {
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

	// Set the game as resigned
	if color == chess.White {
		game.Resign(chess.White)
	} else {
		game.Resign(chess.Black)
	}

	// Update ELO ratings if context and repo are provided
	if ctx != nil && userRepo != nil {
		_ = s.UpdatePlayerRatings(ctx, gameID, state.WhitePlayer, state.BlackPlayer, userRepo)
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
func (s *GameService) AcceptDraw(gameID string, ctx context.Context, userRepo repositories.UserRepository) error {
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

	// Update ELO ratings if context and repo are provided
	if ctx != nil && userRepo != nil {
		_ = s.UpdatePlayerRatings(ctx, gameID, state.WhitePlayer, state.BlackPlayer, userRepo)
	}

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

// CalculateEloChange calculates the ELO rating change based on game outcome
func calculateEloChange(playerRating, opponentRating int, outcome float64) int {
	// K-factor determines the maximum possible adjustment
	// Higher for newer players, lower for established players
	kFactor := 32

	// Expected score based on ELO difference
	expectedScore := 1.0 / (1.0 + math.Pow(10, float64(opponentRating-playerRating)/400.0))

	// Calculate ELO change
	// outcome: 1.0 for win, 0.5 for draw, 0.0 for loss
	change := int(math.Round(float64(kFactor) * (outcome - expectedScore)))

	return change
}

// UpdatePlayerRatings updates the ELO ratings of both players after a game
func (s *GameService) UpdatePlayerRatings(
	ctx context.Context,
	gameID string,
	whiteUserID string,
	blackUserID string,
	userRepo repositories.UserRepository,
) error {
	// Get game outcome
	isOver, outcome, _, err := s.IsGameOver(gameID)
	if err != nil {
		return err
	}

	if !isOver {
		return fmt.Errorf("game is not over yet")
	}

	// Get player ratings
	whiteUser, err := userRepo.GetByID(ctx, whiteUserID)
	if err != nil {
		return err
	}

	blackUser, err := userRepo.GetByID(ctx, blackUserID)
	if err != nil {
		return err
	}

	whiteRating := whiteUser.EloRating
	blackRating := blackUser.EloRating

	// Determine outcome values for ELO calculation
	var whiteOutcome, blackOutcome float64

	switch outcome {
	case chess.WhiteWon:
		whiteOutcome = 1.0
		blackOutcome = 0.0
	case chess.BlackWon:
		whiteOutcome = 0.0
		blackOutcome = 1.0
	case chess.Draw:
		whiteOutcome = 0.5
		blackOutcome = 0.5
	default:
		return fmt.Errorf("invalid game outcome")
	}

	// Calculate rating changes
	whiteRatingChange := calculateEloChange(whiteRating, blackRating, whiteOutcome)
	blackRatingChange := calculateEloChange(blackRating, whiteRating, blackOutcome)

	// Update ratings
	whiteUser.EloRating += whiteRatingChange
	blackUser.EloRating += blackRatingChange

	// Save updated ratings
	err = userRepo.Update(ctx, whiteUser)
	if err != nil {
		return err
	}

	err = userRepo.Update(ctx, blackUser)
	if err != nil {
		return err
	}

	return nil
}
