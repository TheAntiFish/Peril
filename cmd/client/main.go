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

	ch, err := conn.Channel()
	if err != nil {
		fmt.Printf("Failed to open a channel: %s\n", err)
		return
	}
	defer ch.Close()

	userName, err := gamelogic.ClientWelcome()
	if err != nil {
		fmt.Printf("Failed to get client welcome: %s\n", err)
		return
	}

	gameState := gamelogic.NewGameState(userName)

	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilDirect, routing.PauseKey + "." + userName, routing.PauseKey, pubsub.Transient, handlerPause(gameState))
	if err != nil {
		fmt.Printf("Failed to subscribe to pause messages: %s\n", err)
		return
	}
	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic, routing.ArmyMovesPrefix + "." + userName, routing.ArmyMovesPrefix + ".*", pubsub.Transient, handlerMove(gameState, ch))
	if err != nil {
		fmt.Printf("Failed to subscribe to move messages: %s\n", err)
		return
	}

	err = pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic, routing.WarRecognitionsPrefix, routing.WarRecognitionsPrefix + ".*", pubsub.Durable, handlerWarRecognition(gameState))
	if err != nil {
		fmt.Printf("Failed to subscribe to war recognition messages: %s\n", err)
		return
	}

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
			move, err := gameState.CommandMove(input)
			if err != nil {
				fmt.Printf("Failed to move: %s\n", err)
				continue
			}
			err = pubsub.PublishJSON(ch, routing.ExchangePerilTopic, routing.ArmyMovesPrefix + "." + userName, move)
			if err != nil {
				fmt.Printf("Failed to publish move: %s\n", err)
				continue
			}
			fmt.Printf("Published move")
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

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.Acktype {
	return func(ps routing.PlayingState) pubsub.Acktype {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState, ch *amqp.Channel) func(gamelogic.ArmyMove) pubsub.Acktype {
	return func(mv gamelogic.ArmyMove) pubsub.Acktype {
		defer fmt.Print("> ")
		outcome := gs.HandleMove(mv)

		if outcome == gamelogic.MoveOutComeSafe {
			return pubsub.Ack
		}

		if outcome == gamelogic.MoveOutcomeMakeWar {
			err := pubsub.PublishJSON(
				ch,
				routing.ExchangePerilTopic,
				routing.WarRecognitionsPrefix + "." + gs.Player.Username,
				gamelogic.RecognitionOfWar{
					Attacker: mv.Player,
					Defender: gs.GetPlayerSnap(),
			})
			if err != nil {
				fmt.Printf("Failed to publish war: %s\n", err)
				return pubsub.NackRequeue
			}

			return pubsub.Ack
		}
		return pubsub.NackDiscard
	}
}

func handlerWarRecognition(gs *gamelogic.GameState) func(gamelogic.RecognitionOfWar) pubsub.Acktype {
	return func(rec gamelogic.RecognitionOfWar) pubsub.Acktype {
		defer fmt.Print("> ")

		outcome, _, _ := gs.HandleWar(rec)

		if outcome == gamelogic.WarOutcomeNotInvolved{
			return pubsub.NackRequeue
		}

		if outcome == gamelogic.WarOutcomeNoUnits {
			return pubsub.NackDiscard
		}

		if outcome == gamelogic.WarOutcomeOpponentWon || outcome == gamelogic.WarOutcomeYouWon || outcome == gamelogic.WarOutcomeDraw {
			return pubsub.Ack
		}

		fmt.Printf("Unknown war outcome: %d\n", outcome)
		return pubsub.NackDiscard
	}
}
