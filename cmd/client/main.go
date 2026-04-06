package main

import (
	"fmt"

	"github.com/TheAntiFish/learn-pub-sub-starter/internal/gamelogic"
	"github.com/TheAntiFish/learn-pub-sub-starter/internal/pubsub"
	"github.com/TheAntiFish/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	connectionString := "amqp://guest:guest@localhost:5672/"

	conn, err := amqp.Dial(connectionString)
	if err != nil {
		fmt.Printf("Failed to connect to RabbitMQ: %s\n", err)
		return
	}
	defer conn.Close()

	userName, err := gamelogic.ClientWelcome()
	if err != nil {
		fmt.Printf("Failed to get client welcome: %s\n", err)
		return
	}

	gameState := gamelogic.NewGameState(userName)

	pubsub.SubscribeJSON(conn, routing.ExchangePerilDirect, routing.PauseKey + "." + userName, routing.PauseKey, pubsub.Transient, handlerPause(gameState))

	clientLoop:
	for {
		input := gamelogic.GetInput()

		if len(input) == 0 {
			continue
		}

		switch input[0] {
		case "spawn":
			err := gameState.CommandSpawn(input)
			if err != nil {
				fmt.Printf("Failed to spawn: %s\n", err)
			}
		case "move":
			_, err := gameState.CommandMove(input)
			if err != nil {
				fmt.Printf("Failed to move: %s\n", err)
			}
		case "status":
			gameState.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			fmt.Println("Spamming not implemented yet!")
		case "quit":
			gamelogic.PrintQuit()
			break clientLoop
		default:
			fmt.Printf("Unknown command: %s\n", input[0])
		}
	}

	/*
		// wait for ctrl+c
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)
		<-signalChan
	*/
}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) {
	return func(ps routing.PlayingState) {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
	}
}
