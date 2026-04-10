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

	err = pubsub.SubscribeGob(conn, routing.ExchangePerilTopic, routing.GameLogSlug, routing.GameLogSlug + ".*", pubsub.Durable, handlerLogs[any]())
	if err != nil {
		fmt.Printf("Failed to subscribe to Gob: %s\n", err)
		return
	}

	gamelogic.PrintServerHelp()

	serverLoop:
		for {
			input := gamelogic.GetInput()

			if len(input) == 0 {
				continue
			}

			switch input[0] {
			case "pause":
				fmt.Println("Sending Pause Message")
				pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true})
			case "resume":
				fmt.Println("Sending Resume Message")
				pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: false})
			case "quit":
				fmt.Println("Quitting Server")
				break serverLoop
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

	fmt.Println("Closing Connection")
}

func handlerLogs[T any]() func(routing.GameLog) pubsub.Acktype {
	defer fmt.Print("> ")
	return func(log routing.GameLog) pubsub.Acktype {
		gamelogic.WriteLog(log)
		return pubsub.Ack
	}
}
