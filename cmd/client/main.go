package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	Durable   = 0
	Transient = 1
)

func main() {
	fmt.Println("starting Peril client...")

	string_connect := "amqp://guest:guest@localhost:5672/"
	connection, err := amqp.Dial(string_connect)
	if err != nil {
		log.Fatalf("could not connect to RabbitMQ: %v", err)
	}
	defer connection.Close()

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		log.Fatalf("could not get username: %v", err)
	}

	ch, queue, err := pubsub.DeclareAndBind(
		connection,
		routing.ExchangePerilDirect,
		routing.PauseKey+"."+username,
		routing.PauseKey,
		pubsub.SimpleQueueTransient)
	if err != nil {
		log.Fatalf("could not subscribe to pause: %v", err)
	}
	fmt.Printf("queue %v declared and bound!\n", queue.Name)

	gs := gamelogic.NewGameState(username)

	err = pubsub.SubscribeJSON(
		connection,
		routing.ExchangePerilDirect,
		routing.PauseKey+"."+username,
		routing.PauseKey,
		pubsub.SimpleQueueTransient,
		handlerPause(gs))
	if err != nil {
		log.Fatalf("could not subscribe to pause: %v", err)
	}

	err = pubsub.SubscribeJSON(
		connection,
		routing.ExchangePerilTopic,
		routing.ArmyMovesPrefix+"."+username,
		routing.ArmyMovesPrefix+"."+"*",
		pubsub.SimpleQueueTransient,
		handlerMove(gs))
	if err != nil {
		log.Fatalf("could not subscribe to moves: %v", err)
	}

	// war queue
	err = pubsub.SubscribeJSON(
		connection,
		routing.ExchangePerilTopic,
		"war",
		routing.WarRecognitionsPrefix+"."+"*",
		pubsub.SimpleQueueDurable,
		handlerWar(gs, ch))
	if err != nil {
		log.Fatalf("could not subscribe to war: %v", err)
	}

	for {
		input := gamelogic.GetInput()
		if len(input) == 0 {
			continue
		}
		switch input[0] {
		case "spawn":
			err = gs.CommandSpawn(input)
			if err != nil {
				log.Printf("could not spawn unit: %v", err)
			}

		case "move":
			mv, err := gs.CommandMove(input)
			if err != nil {
				log.Printf("could not move unit: %v", err)
			}

			err = pubsub.PublishJSON(
				ch,
				routing.ExchangePerilTopic,
				routing.ArmyMovesPrefix+"."+username,
				mv)
			if err != nil {
				log.Printf("could not publish move: %v", err)
			}

			fmt.Println("move published sucessfully")

		case "status":
			gs.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			if len(input) < 2 {
				fmt.Println("invalid command. usage: spam <number>")
				continue
			}
			n, err := strconv.Atoi(input[1])
			if err != nil {
				fmt.Println("error: ", err)
				continue
			}
			for i := 0; i < n; i++ {
				msg := gamelogic.GetMaliciousLog()
				currentTime := time.Now()
				gamelog := routing.GameLog{
					CurrentTime: currentTime,
					Username:    username,
					Message:     msg,
				}
				err = pubsub.PublishJSON(
					ch,
					routing.ExchangePerilTopic,
					routing.GameLogSlug+"."+username,
					gamelog)
				if err != nil {
					log.Printf("could not publish log: %v", err)
				}
			}
		case "quit":
			gamelogic.PrintQuit()
			return
		default:
			fmt.Println("Invalid command")
		}
	}
}
